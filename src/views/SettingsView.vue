<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useObservabilityStore } from '@/stores/observability'
import { useUiStore } from '@/stores/ui'
import { validateObservabilitySettings, type ObservabilitySettings } from '@/types/observability'
import { loadUmami, umamiStatus } from '@/composables/useUmami'
import { initOtel } from '@/composables/useOtel'

const store = useObservabilityStore()
const uiStore = useUiStore()

// Local editable copy — only persisted on Save, so invalid input never
// overwrites the active (persisted) configuration.
const draft = reactive<ObservabilitySettings>({
  umami: { ...store.settings.umami },
  otel: { ...store.settings.otel },
})

const errors = computed(() => validateObservabilitySettings(draft))
const umami = umamiStatus()
const saved = ref(false)

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
