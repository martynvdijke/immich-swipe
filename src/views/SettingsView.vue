<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useEmailSettingsStore } from '@/stores/emailSettings'
import { useObservabilityStore } from '@/stores/observability'
import { useUiStore } from '@/stores/ui'
import { validateObservabilitySettings, type ObservabilitySettings } from '@/types/observability'
import { loadUmami, umamiStatus } from '@/composables/useUmami'
import { initOtel } from '@/composables/useOtel'

const store = useObservabilityStore()
const uiStore = useUiStore()
const authStore = useAuthStore()
const emailStore = useEmailSettingsStore()

// Local editable copy — only persisted on Save, so invalid input never
// overwrites the active (persisted) configuration.
const draft = reactive<ObservabilitySettings>({
  umami: { ...store.settings.umami },
  otel: { ...store.settings.otel },
})

const errors = computed(() => validateObservabilitySettings(draft))
const umami = umamiStatus()
const saved = ref(false)

// ── Local account password ────────────────────────────────────────────────
// Only API-key sessions can carry an Immich API key that the account can be
// bound to, so access-token sessions (Immich password login) cannot set one.
const accountAvailable = computed(
  () => authStore.isLoggedIn && authStore.activeSessionMode !== 'accessToken'
)
const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const accountError = ref('')
const accountSaving = ref(false)
const accountSaved = ref(false)

async function saveAccountPassword() {
  accountError.value = ''
  if (newPassword.value.length < 8) {
    accountError.value = 'Password must be at least 8 characters'
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    accountError.value = 'Passwords do not match'
    return
  }
  accountSaving.value = true
  const result = await authStore.setAccountPassword(currentPassword.value, newPassword.value)
  accountSaving.value = false
  if (result.ok) {
    accountSaved.value = true
    currentPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
    uiStore.toast('Account password saved', 'success', 1500)
    setTimeout(() => (accountSaved.value = false), 2000)
  } else {
    accountError.value = result.error
  }
}

// ── Immich API key ────────────────────────────────────────────────────────
const apiKey = ref('')
const apiKeyError = ref('')
const apiKeySaving = ref(false)
const apiKeySaved = ref(false)

async function saveApiKey() {
  apiKeyError.value = ''
  if (!apiKey.value.trim()) {
    apiKeyError.value = 'Please enter your Immich API key'
    return
  }
  apiKeySaving.value = true
  const result = await authStore.setApiKey(apiKey.value.trim())
  apiKeySaving.value = false
  if (result.ok) {
    apiKeySaved.value = true
    apiKey.value = ''
    uiStore.toast('API key saved', 'success', 1500)
    setTimeout(() => (apiKeySaved.value = false), 2000)
  } else {
    apiKeyError.value = result.error
  }
}

// ── Account email ───────────────────────────────────────────────────────
const accountEmail = ref('')
const accountEmailError = ref('')
const accountEmailSaving = ref(false)
const accountEmailSaved = ref(false)
async function saveAccountEmail() {
  accountEmailError.value = ''
  if (!accountEmail.value.trim() || !accountEmail.value.includes('@')) {
    accountEmailError.value = 'Please enter a valid email'
    return
  }
  accountEmailSaving.value = true
  const result = await authStore.setAccountEmail(accountEmail.value.trim())
  accountEmailSaving.value = false
  if (result.ok) {
    accountEmailSaved.value = true
    accountEmail.value = ''
    uiStore.toast('Email saved', 'success', 1500)
    setTimeout(() => (accountEmailSaved.value = false), 2000)
  } else {
    accountEmailError.value = result.error
  }
}

// ── Email / SMTP ─────────────────────────────────────────────────────────
onMounted(() => { if (authStore.isLoggedIn) emailStore.fetch() })
const emailDraft = ref({ host: '', port: 0, user: '', pass: '', from: '', tls: 'starttls' })
const emailError = ref('')
const emailSaving = ref(false)
const emailSaved = ref(false)
const testEmailTo = ref('')
const testEmailSending = ref(false)
const testEmailResult = ref('')
watch(() => emailStore.host, () => {
  emailDraft.value.host = emailStore.host
  emailDraft.value.port = emailStore.port
  emailDraft.value.user = emailStore.user
  emailDraft.value.from = emailStore.from
  emailDraft.value.tls = emailStore.tls || 'starttls'
}, { immediate: true })
async function saveEmailSettings() {
  emailError.value = ''
  if (!emailDraft.value.host.trim()) { emailError.value = 'SMTP host is required'; return }
  emailSaving.value = true
  try {
    await emailStore.save({ host: emailDraft.value.host.trim(), port: emailDraft.value.port, user: emailDraft.value.user.trim(), pass: emailDraft.value.pass, from: emailDraft.value.from.trim(), tls: emailDraft.value.tls })
    emailSaved.value = true
    uiStore.toast('Email settings saved', 'success', 1500)
    setTimeout(() => (emailSaved.value = false), 2000)
  } catch (e: unknown) {
    emailError.value = e instanceof Error ? e.message : 'Save failed'
  } finally { emailSaving.value = false }
}
async function sendTestEmail() {
  testEmailResult.value = ''
  if (!testEmailTo.value.includes('@')) { testEmailResult.value = 'Enter a valid email'; return }
  testEmailSending.value = true
  try {
    await emailStore.sendTest(testEmailTo.value.trim())
    testEmailResult.value = 'Test email sent ✓'
  } catch (e: unknown) {
    testEmailResult.value = e instanceof Error ? e.message : 'Failed'
  } finally { testEmailSending.value = false }
}

// Keep draft in sync when the active settings change (e.g. re-login as
// another user, or initial load that happened after first render).
watch(
  () => store.settings,
  (next) => {
    draft.umami = { ...next.umami }
    draft.otel = { ...next.otel }
  },
  { deep: true },
)

async function save() {
  if (!errors.value.valid) return
  store.setUmami({ ...draft.umami })
  store.setOtel({ ...draft.otel })
  // Hot-apply: (re)configure or tear down integrations immediately.
  await loadUmami(store.settings.umami)
  await initOtel(store.settings.otel)
  saved.value = true
  uiStore.toast('Observability settings saved', 'success', 1500)
  setTimeout(() => (saved.value = false), 2000)
}

onBeforeUnmount(() => {
  // Nothing to tear down here: the bootstrap watcher in main.ts owns the
  // live integrations and will reflect any later store change.
})
</script>

<template>
  <main class="max-w-2xl mx-auto px-4 py-6">
    <h1 class="text-2xl font-bold mb-1" :class="uiStore.isDarkMode ? 'text-white' : 'text-gray-900'">
      Settings
    </h1>

    <!-- Local account -->
    <section
      v-if="accountAvailable"
      class="rounded-2xl shadow-lg border p-5 mb-6"
      :class="uiStore.isDarkMode ? 'bg-gray-900 border-gray-800' : 'bg-white border-gray-200'"
    >
      <h2 class="font-semibold" :class="uiStore.isDarkMode ? 'text-white' : 'text-gray-900'">
        Local account
      </h2>
      <p class="text-xs mt-0.5 mb-4" :class="uiStore.isDarkMode ? 'text-gray-400' : 'text-gray-500'">
        Signed in as <span class="font-medium">{{ authStore.currentUserName }}</span> on
        {{ authStore.immichServerUrl }}. Change your Swipe account password here.
      </p>

      <div class="space-y-4">
        <div>
          <label class="block text-sm mb-1" :class="uiStore.isDarkMode ? 'text-gray-300' : 'text-gray-700'">
            Current password
          </label>
          <input
            v-model="currentPassword"
            type="password"
            data-testid="account-current-password"
            autocomplete="current-password"
            placeholder="Only needed when changing an existing password"
            class="w-full px-3 py-2 rounded-lg border text-sm"
            :class="uiStore.isDarkMode
              ? 'bg-gray-800 border-gray-700 text-white'
              : 'bg-gray-50 border-gray-300 text-gray-900'"
          />
        </div>
        <div>
          <label class="block text-sm mb-1" :class="uiStore.isDarkMode ? 'text-gray-300' : 'text-gray-700'">
            New password
          </label>
          <input
            v-model="newPassword"
            type="password"
            data-testid="account-new-password"
            autocomplete="new-password"
            placeholder="At least 8 characters"
            class="w-full px-3 py-2 rounded-lg border text-sm"
            :class="uiStore.isDarkMode
              ? 'bg-gray-800 border-gray-700 text-white'
              : 'bg-gray-50 border-gray-300 text-gray-900'"
          />
        </div>
        <div>
          <label class="block text-sm mb-1" :class="uiStore.isDarkMode ? 'text-gray-300' : 'text-gray-700'">
            Confirm new password
          </label>
          <input
            v-model="confirmPassword"
            type="password"
            data-testid="account-confirm-password"
            autocomplete="new-password"
            placeholder="Repeat the new password"
            class="w-full px-3 py-2 rounded-lg border text-sm"
            :class="uiStore.isDarkMode
              ? 'bg-gray-800 border-gray-700 text-white'
              : 'bg-gray-50 border-gray-300 text-gray-900'"
          />
        </div>
      </div>

      <p v-if="accountError" class="text-xs text-red-500 mt-3" data-testid="account-error">
        {{ accountError }}
      </p>

      <div class="flex items-center gap-3 mt-4">
        <button
          type="button"
          data-testid="account-save-btn"
          @click="saveAccountPassword"
          :disabled="accountSaving"
          class="px-4 py-2 rounded-full text-sm font-medium border transition-colors disabled:opacity-40"
          :class="uiStore.isDarkMode
            ? 'bg-indigo-600 hover:bg-indigo-500 border-indigo-500 text-white'
            : 'bg-indigo-600 hover:bg-indigo-500 border-indigo-500 text-white'"
        >
          {{ accountSaving ? 'Saving…' : 'Save password' }}
        </button>
        <span v-if="accountSaved" class="text-sm" :class="uiStore.isDarkMode ? 'text-green-400' : 'text-green-600'">
          Saved ✓
        </span>
      </div>

      <!-- Divider -->
      <div class="my-6 h-px" :class="uiStore.isDarkMode ? 'bg-gray-800' : 'bg-gray-200'"></div>

      <h3 class="font-medium text-sm mb-1" :class="uiStore.isDarkMode ? 'text-white' : 'text-gray-900'">
        Immich API key
      </h3>
      <p class="text-xs mb-3" :class="uiStore.isDarkMode ? 'text-gray-400' : 'text-gray-500'">
        Required to review photos. Stored on the server for this account. If Immich rejects your key you can update it here.
      </p>

      <div>
        <label class="block text-sm mb-1" :class="uiStore.isDarkMode ? 'text-gray-300' : 'text-gray-700'">
          API key
        </label>
        <input
          v-model="apiKey"
          type="password"
          data-testid="account-api-key"
          autocomplete="off"
          placeholder="Your Immich API key"
          class="w-full px-3 py-2 rounded-lg border text-sm"
          :class="uiStore.isDarkMode
            ? 'bg-gray-800 border-gray-700 text-white'
            : 'bg-gray-50 border-gray-300 text-gray-900'"
        />
      </div>

      <p v-if="apiKeyError" class="text-xs text-red-500 mt-3" data-testid="account-api-key-error">
        {{ apiKeyError }}
      </p>

      <div class="flex items-center gap-3 mt-4">
        <button
          type="button"
          data-testid="account-api-key-save-btn"
          @click="saveApiKey"
          :disabled="apiKeySaving"
          class="px-4 py-2 rounded-full text-sm font-medium border transition-colors disabled:opacity-40"
          :class="uiStore.isDarkMode
            ? 'bg-indigo-600 hover:bg-indigo-500 border-indigo-500 text-white'
            : 'bg-indigo-600 hover:bg-indigo-500 border-indigo-500 text-white'"
        >
          {{ apiKeySaving ? 'Saving…' : 'Save API key' }}
        </button>
        <span v-if="apiKeySaved" class="text-sm" :class="uiStore.isDarkMode ? 'text-green-400' : 'text-green-600'">
          Saved ✓
        </span>
      </div>
    </section>

    <p class="text-sm mb-6" :class="uiStore.isDarkMode ? 'text-gray-400' : 'text-gray-500'">
      Analytics and tracing are optional. Nothing is sent anywhere until you enable them here.
    </p>

    <!-- Umami -->
    <section
      class="rounded-2xl shadow-lg border p-5 mb-6"
      :class="uiStore.isDarkMode ? 'bg-gray-900 border-gray-800' : 'bg-white border-gray-200'"
    >
      <div class="flex items-center justify-between mb-4">
        <div>
          <h2 class="font-semibold" :class="uiStore.isDarkMode ? 'text-white' : 'text-gray-900'">
            Umami Analytics
          </h2>
          <p class="text-xs mt-0.5" :class="uiStore.isDarkMode ? 'text-gray-400' : 'text-gray-500'">
            Page views and swipe events. Self-hosted at your Umami instance.
          </p>
        </div>
        <label class="flex items-center cursor-pointer">
          <input v-model="draft.umami.enabled" type="checkbox" class="sr-only" data-testid="umami-enabled" />
          <span
            class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors"
            :class="draft.umami.enabled ? 'bg-indigo-600' : uiStore.isDarkMode ? 'bg-gray-700' : 'bg-gray-300'"
          >
            <span
              class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform"
              :class="draft.umami.enabled ? 'translate-x-6' : 'translate-x-1'"
            />
          </span>
        </label>
      </div>

      <div class="space-y-4" :class="draft.umami.enabled ? '' : 'opacity-50 pointer-events-none'">
        <div>
          <label class="block text-sm mb-1" :class="uiStore.isDarkMode ? 'text-gray-300' : 'text-gray-700'">
            Server URL
          </label>
          <input
            v-model="draft.umami.serverUrl"
            type="url"
            data-testid="umami-server-url"
            placeholder="https://umami.example.com"
            class="w-full px-3 py-2 rounded-lg border text-sm"
            :class="uiStore.isDarkMode
              ? 'bg-gray-800 border-gray-700 text-white'
              : 'bg-gray-50 border-gray-300 text-gray-900'"
          />
          <p v-if="errors.errors['umami.serverUrl']" class="text-xs text-red-500 mt-1">
            {{ errors.errors['umami.serverUrl'] }}
          </p>
        </div>
        <div>
          <label class="block text-sm mb-1" :class="uiStore.isDarkMode ? 'text-gray-300' : 'text-gray-700'">
            Website ID
          </label>
          <input
            v-model="draft.umami.websiteId"
            type="text"
            data-testid="umami-website-id"
            placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
            class="w-full px-3 py-2 rounded-lg border text-sm"
            :class="uiStore.isDarkMode
              ? 'bg-gray-800 border-gray-700 text-white'
              : 'bg-gray-50 border-gray-300 text-gray-900'"
          />
          <p v-if="errors.errors['umami.websiteId']" class="text-xs text-red-500 mt-1">
            {{ errors.errors['umami.websiteId'] }}
          </p>
        </div>
        <div>
          <label class="block text-sm mb-1" :class="uiStore.isDarkMode ? 'text-gray-300' : 'text-gray-700'">
            Host URL <span class="opacity-60">(optional, data-host-url)</span>
          </label>
          <input
            v-model="draft.umami.hostUrl"
            type="url"
            data-testid="umami-host-url"
            placeholder="https://analytics.example.com"
            class="w-full px-3 py-2 rounded-lg border text-sm"
            :class="uiStore.isDarkMode
              ? 'bg-gray-800 border-gray-700 text-white'
              : 'bg-gray-50 border-gray-300 text-gray-900'"
          />
          <p v-if="errors.errors['umami.hostUrl']" class="text-xs text-red-500 mt-1">
            {{ errors.errors['umami.hostUrl'] }}
          </p>
        </div>
      </div>

      <p class="text-xs mt-3" :class="uiStore.isDarkMode ? 'text-gray-400' : 'text-gray-500'">
        <template v-if="umami.loading.value">Loading script…</template>
        <template v-else-if="umami.error.value" class="text-red-500">{{ umami.error.value }}</template>
        <template v-else-if="umami.ready.value">Script loaded and tracking active.</template>
        <template v-else>Not loaded.</template>
      </p>
    </section>

    <!-- Email / SMTP -->
    <section class="rounded-2xl shadow-lg border p-5 mb-6" :class="uiStore.isDarkMode ? 'bg-gray-900 border-gray-800' : 'bg-white border-gray-200'">
      <h2 class="font-semibold" :class="uiStore.isDarkMode ? 'text-white' : 'text-gray-900'">Email / SMTP</h2>
      <p class="text-xs mt-0.5 mb-4" :class="uiStore.isDarkMode ? 'text-gray-400' : 'text-gray-500'">Configure SMTP for password reset emails. Env vars override these settings.</p>
      <div v-if="emailStore.fromEnv" class="text-xs text-amber-600 mb-3">Email is configured via environment variables (SMTP_HOST etc.) and cannot be changed here.</div>
      <div class="space-y-3">
        <div><label class="block text-sm mb-1">SMTP host</label><input v-model="emailDraft.host" :disabled="emailStore.fromEnv" data-testid="smtp-host" class="w-full px-3 py-2 rounded-lg border text-sm" /></div>
        <div><label class="block text-sm mb-1">SMTP port</label><input v-model.number="emailDraft.port" type="number" :disabled="emailStore.fromEnv" data-testid="smtp-port" class="w-full px-3 py-2 rounded-lg border text-sm" /></div>
        <div><label class="block text-sm mb-1">SMTP user</label><input v-model="emailDraft.user" :disabled="emailStore.fromEnv" data-testid="smtp-user" class="w-full px-3 py-2 rounded-lg border text-sm" /></div>
        <div><label class="block text-sm mb-1">SMTP password <span v-if="emailStore.hasPassword" class="opacity-60">(saved)</span></label><input v-model="emailDraft.pass" type="password" :disabled="emailStore.fromEnv" data-testid="smtp-pass" class="w-full px-3 py-2 rounded-lg border text-sm" /></div>
        <div><label class="block text-sm mb-1">From address</label><input v-model="emailDraft.from" :disabled="emailStore.fromEnv" data-testid="smtp-from" class="w-full px-3 py-2 rounded-lg border text-sm" /></div>
        <div><label class="block text-sm mb-1">TLS</label><select v-model="emailDraft.tls" :disabled="emailStore.fromEnv" data-testid="smtp-tls" class="w-full px-3 py-2 rounded-lg border text-sm"><option value="starttls">STARTTLS</option><option value="ssl">SSL</option><option value="none">None</option></select></div>
      </div>
      <p v-if="emailError" class="text-xs text-red-500 mt-2">{{ emailError }}</p>
      <div class="flex items-center gap-3 mt-4">
        <button type="button" data-testid="email-save-btn" @click="saveEmailSettings" :disabled="emailSaving || emailStore.fromEnv" class="px-4 py-2 rounded-full text-sm font-medium border bg-indigo-600 text-white disabled:opacity-40">Save SMTP</button>
        <span v-if="emailSaved" class="text-sm text-green-600">Saved ✓</span>
      </div>
      <div class="mt-4 flex gap-2">
        <input v-model="testEmailTo" placeholder="test@example.com" data-testid="smtp-test-to" class="flex-1 px-3 py-2 rounded-lg border text-sm" />
        <button type="button" @click="sendTestEmail" :disabled="testEmailSending" data-testid="smtp-test-btn" class="px-4 py-2 rounded-full text-sm border disabled:opacity-40">Send test</button>
      </div>
      <p v-if="testEmailResult" class="text-xs mt-2">{{ testEmailResult }}</p>

      <div class="mt-6">
        <h3 class="font-medium text-sm mb-1">Account email</h3>
        <p class="text-xs mb-2 opacity-70">Used for password reset. Set it if it is missing.</p>
        <div class="flex gap-2">
          <input v-model="accountEmail" placeholder="you@example.com" data-testid="account-email" class="flex-1 px-3 py-2 rounded-lg border text-sm" />
          <button type="button" @click="saveAccountEmail" :disabled="accountEmailSaving" data-testid="account-email-save-btn" class="px-4 py-2 rounded-full text-sm border bg-indigo-600 text-white">Save email</button>
        </div>
        <p v-if="accountEmailError" class="text-xs text-red-500 mt-1">{{ accountEmailError }}</p>
        <span v-if="accountEmailSaved" class="text-xs text-green-600">Saved ✓</span>
      </div>
    </section>

    <!-- OpenTelemetry -->
    <section
      class="rounded-2xl shadow-lg border p-5 mb-6"
      :class="uiStore.isDarkMode ? 'bg-gray-900 border-gray-800' : 'bg-white border-gray-200'"
    >
      <div class="flex items-center justify-between mb-4">
        <div>
          <h2 class="font-semibold" :class="uiStore.isDarkMode ? 'text-white' : 'text-gray-900'">
            OpenTelemetry
          </h2>
          <p class="text-xs mt-0.5" :class="uiStore.isDarkMode ? 'text-gray-400' : 'text-gray-500'">
            Browser traces and swipe statistics sent to an OTLP/HTTP collector.
          </p>
        </div>
        <label class="flex items-center cursor-pointer">
          <input v-model="draft.otel.enabled" type="checkbox" class="sr-only" data-testid="otel-enabled" />
          <span
            class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors"
            :class="draft.otel.enabled ? 'bg-indigo-600' : uiStore.isDarkMode ? 'bg-gray-700' : 'bg-gray-300'"
          >
            <span
              class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform"
              :class="draft.otel.enabled ? 'translate-x-6' : 'translate-x-1'"
            />
          </span>
        </label>
      </div>

      <div class="space-y-4" :class="draft.otel.enabled ? '' : 'opacity-50 pointer-events-none'">
        <div>
          <label class="block text-sm mb-1" :class="uiStore.isDarkMode ? 'text-gray-300' : 'text-gray-700'">
            OTLP/HTTP endpoint
          </label>
          <input
            v-model="draft.otel.endpoint"
            type="url"
            data-testid="otel-endpoint"
            placeholder="https://collector.example.com:4318"
            class="w-full px-3 py-2 rounded-lg border text-sm"
            :class="uiStore.isDarkMode
              ? 'bg-gray-800 border-gray-700 text-white'
              : 'bg-gray-50 border-gray-300 text-gray-900'"
          />
          <p class="text-xs mt-1 opacity-70" :class="uiStore.isDarkMode ? 'text-gray-400' : 'text-gray-500'">
            Traces go to <code>/v1/traces</code>, metrics to <code>/v1/metrics</code>.
          </p>
          <p v-if="errors.errors['otel.endpoint']" class="text-xs text-red-500 mt-1">
            {{ errors.errors['otel.endpoint'] }}
          </p>
        </div>
        <div>
          <label class="block text-sm mb-1" :class="uiStore.isDarkMode ? 'text-gray-300' : 'text-gray-700'">
            Sampling <span class="opacity-60">(0–100 %)</span>
          </label>
          <input
            v-model.number="draft.otel.samplingPercent"
            type="number"
            data-testid="otel-sampling"
            min="0"
            max="100"
            class="w-full px-3 py-2 rounded-lg border text-sm"
            :class="uiStore.isDarkMode
              ? 'bg-gray-800 border-gray-700 text-white'
              : 'bg-gray-50 border-gray-300 text-gray-900'"
          />
          <p class="text-xs mt-1 opacity-70" :class="uiStore.isDarkMode ? 'text-gray-400' : 'text-gray-500'">
            100% samples every trace root; 0% disables traces but metrics are always reported.
          </p>
          <p v-if="errors.errors['otel.samplingPercent']" class="text-xs text-red-500 mt-1">
            {{ errors.errors['otel.samplingPercent'] }}
          </p>
        </div>
      </div>
    </section>

    <div class="flex items-center gap-3">
      <button
        type="button"
        data-testid="save-btn" @click="save"
        :disabled="!errors.valid"
        class="px-4 py-2 rounded-full text-sm font-medium border transition-colors disabled:opacity-40"
        :class="uiStore.isDarkMode
          ? 'bg-indigo-600 hover:bg-indigo-500 border-indigo-500 text-white'
          : 'bg-indigo-600 hover:bg-indigo-500 border-indigo-500 text-white'"
      >
        Save
      </button>
      <span v-if="saved" class="text-sm" :class="uiStore.isDarkMode ? 'text-green-400' : 'text-green-600'">
        Saved ✓
      </span>
    </div>
  </main>
</template>
