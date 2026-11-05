<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'

type LoginMode = 'swipe' | 'create'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const uiStore = useUiStore()

const loginMode = ref<LoginMode>('swipe')
const serverUrl = ref(authStore.immichServerUrl || authStore.defaultServerUrl || '')
const password = ref('')
const apiKey = ref('')
const userName = ref('')
const error = ref('')
const isSubmitting = ref(false)
const demoSubmitting = ref(false)

function setMode(mode: LoginMode) {
  loginMode.value = mode
  error.value = ''
}

/** SSO login: redirect the browser to the IdP via the backend. */
async function startSso() {
  error.value = ''
  const url = serverUrl.value.trim() || authStore.defaultServerUrl || ''
  if (!url) {
    error.value = 'Please enter your Immich server URL'
    return
  }
  isSubmitting.value = true
  const result = await authStore.startOAuthLogin(url)
  if (result.ok && result.url) {
    window.location.href = result.url
  } else if (!result.ok) {
    error.value = result.error
    isSubmitting.value = false
  }
}

// Consume the backend's OAuth handoff (full-page redirect lands here).
onMounted(async () => {
  const oauthCode = typeof route.query.oauthCode === 'string' ? route.query.oauthCode : ''
  const oauthError = typeof route.query.oauthError === 'string' ? route.query.oauthError : ''
  if (!oauthCode && !oauthError) return
  // Strip the handoff params so a reload doesn't replay them.
  const { oauthCode: _code, oauthError: _err, ...rest } = route.query
  router.replace({ path: '/login', query: rest })
  if (oauthError) {
    error.value =
      oauthError === 'invalid_state'
        ? 'SSO login expired. Please try again.'
        : 'SSO login failed. Please try again.'
    return
  }
  isSubmitting.value = true
  const result = await authStore.loginWithOAuthCode(oauthCode)
  isSubmitting.value = false
  if (result.ok) {
    uiStore.toast('Connected successfully!', 'success')
    router.push('/')
  } else {
    error.value = result.error
  }
})

async function handleSubmit() {
  error.value = ''
  if (!serverUrl.value.trim()) {
    error.value = 'Please enter your Immich server URL'
    return
  }

  isSubmitting.value = true

  if (loginMode.value === 'swipe') {
    if (!userName.value.trim()) {
      error.value = 'Please enter your user name'
      isSubmitting.value = false
      return
    }
    if (!password.value) {
      error.value = 'Please enter your password'
      isSubmitting.value = false
      return
    }

    const result = await authStore.loginWithAccount(
      userName.value.trim(),
      password.value,
      serverUrl.value.trim(),
    )

    if (result.ok) {
      if (result.needsApiKey === true) {
        uiStore.toast('Set your Immich API key in Settings to start', 'info')
        router.push('/settings')
      } else {
        uiStore.toast('Connected successfully!', 'success')
        router.push('/')
      }
    } else {
      error.value = result.error
    }
  } else {
    if (!userName.value.trim()) {
      error.value = 'Please enter a user name'
      isSubmitting.value = false
      return
    }
    if (!password.value) {
      error.value = 'Please enter a password (at least 8 characters)'
      isSubmitting.value = false
      return
    }
    if (password.value.length < 8) {
      error.value = 'Password must be at least 8 characters'
      isSubmitting.value = false
      return
    }

    const result = await authStore.loginWithAccountCreate(
      userName.value.trim(),
      password.value,
      apiKey.value.trim(),
      serverUrl.value.trim(),
    )

    if (result.ok) {
      if (result.needsApiKey === true) {
        uiStore.toast('Set your Immich API key in Settings to start', 'info')
        router.push('/settings')
      } else {
        uiStore.toast('Connected successfully!', 'success')
        router.push('/')
      }
    } else {
      error.value = result.error
    }
  }

  isSubmitting.value = false
}

/** Fill the form with the shared demo credentials and run the normal login. */
async function useDemoAccount() {
  if (isSubmitting.value) return
  error.value = ''
  loginMode.value = 'swipe'
  userName.value = authStore.demoUserName
  password.value = authStore.demoPassword
  serverUrl.value = authStore.demoServerUrl || authStore.defaultServerUrl || ''
  demoSubmitting.value = true
  try {
    await handleSubmit()
  } finally {
    demoSubmitting.value = false
  }
}
</script>

<template>
  <div
    class="flex h-screen flex-col items-center overflow-y-auto p-6 pt-[max(1.5rem,env(safe-area-inset-top))] pb-[max(1.5rem,env(safe-area-inset-bottom))] supports-[height:100dvh]:h-dvh"
    :class="uiStore.isDarkMode ? 'bg-black text-white' : 'bg-white text-black'"
  >
    <div class="my-auto w-full max-w-md">
      <!-- Logo/Title -->
      <div class="text-center mb-8">
        <h1 class="text-3xl font-bold mb-2">Immich Swipe</h1>
        <p :class="uiStore.isDarkMode ? 'text-gray-400' : 'text-gray-600'">
          Quickly review your photo library
        </p>
      </div>

      <!-- Mode toggle -->
      <div
        class="mb-6 grid grid-cols-2 gap-1 p-1 rounded-xl border"
        :class="uiStore.isDarkMode ? 'border-gray-800 bg-gray-950' : 'border-gray-200 bg-gray-50'"
        role="tablist"
        aria-label="Login method"
      >
        <button
          type="button"
          role="tab"
          :aria-selected="loginMode === 'swipe'"
          class="py-2 px-3 rounded-lg text-sm font-medium transition-colors"
          :class="loginMode === 'swipe'
            ? (uiStore.isDarkMode ? 'bg-white text-black' : 'bg-black text-white')
            : (uiStore.isDarkMode ? 'text-gray-400 hover:text-white' : 'text-gray-600 hover:text-black')"
          @click="setMode('swipe')"
        >
          Sign in
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="loginMode === 'create'"
          class="py-2 px-3 rounded-lg text-sm font-medium transition-colors"
          :class="loginMode === 'create'
            ? (uiStore.isDarkMode ? 'bg-white text-black' : 'bg-black text-white')
            : (uiStore.isDarkMode ? 'text-gray-400 hover:text-white' : 'text-gray-600 hover:text-black')"
          @click="setMode('create')"
        >
          Create account
        </button>
      </div>

      <!-- Login Form -->
      <form @submit.prevent="handleSubmit" class="space-y-6">
        <!-- Server URL -->
        <div>
          <label for="serverUrl" class="block text-sm font-medium mb-2"
            :class="uiStore.isDarkMode ? 'text-gray-300' : 'text-gray-700'"
          >
            Immich Server URL
          </label>
          <input
            id="serverUrl"
            v-model="serverUrl"
            type="url"
            placeholder="https://immich.example.com"
            autocomplete="url"
            class="w-full px-4 py-3 rounded-lg border focus:outline-none focus:ring-2 transition-colors"
            :class="uiStore.isDarkMode
              ? 'bg-gray-900 border-gray-700 text-white placeholder-gray-500 focus:ring-blue-500 focus:border-blue-500'
              : 'bg-white border-gray-300 text-black placeholder-gray-400 focus:ring-blue-500 focus:border-blue-500'"
          />
        </div>

        <!-- Swipe account fields -->
        <template v-if="loginMode === 'swipe'">
          <div>
            <label for="userName" class="block text-sm font-medium mb-2"
              :class="uiStore.isDarkMode ? 'text-gray-300' : 'text-gray-700'"
            >
              User name
            </label>
            <input
              id="userName"
              v-model="userName"
              type="text"
              placeholder="Your user name"
              autocomplete="username"
              class="w-full px-4 py-3 rounded-lg border focus:outline-none focus:ring-2 transition-colors"
              :class="uiStore.isDarkMode
                ? 'bg-gray-900 border-gray-700 text-white placeholder-gray-500 focus:ring-blue-500 focus:border-blue-500'
                : 'bg-white border-gray-300 text-black placeholder-gray-400 focus:ring-blue-500 focus:border-blue-500'"
            />
          </div>

          <div>
            <label for="password" class="block text-sm font-medium mb-2"
              :class="uiStore.isDarkMode ? 'text-gray-300' : 'text-gray-700'"
            >
              Password
            </label>
            <input
              id="password"
              v-model="password"
              type="password"
              placeholder="Your Swipe account password"
              autocomplete="current-password"
              class="w-full px-4 py-3 rounded-lg border focus:outline-none focus:ring-2 transition-colors"
              :class="uiStore.isDarkMode
                ? 'bg-gray-900 border-gray-700 text-white placeholder-gray-500 focus:ring-blue-500 focus:border-blue-500'
                : 'bg-white border-gray-300 text-black placeholder-gray-400 focus:ring-blue-500 focus:border-blue-500'"
            />
          </div>
          <div class="text-right">
            <a href="/forgot-password" class="text-xs underline opacity-70">Forgot password?</a>
          </div>
        </template>

        <!-- Create account fields -->
        <template v-else>
          <div>
            <label for="createUserName" class="block text-sm font-medium mb-2"
              :class="uiStore.isDarkMode ? 'text-gray-300' : 'text-gray-700'"
            >
              User name
            </label>
            <input
              id="createUserName"
              v-model="userName"
              type="text"
              placeholder="Your user name"
              autocomplete="username"
              class="w-full px-4 py-3 rounded-lg border focus:outline-none focus:ring-2 transition-colors"
              :class="uiStore.isDarkMode
                ? 'bg-gray-900 border-gray-700 text-white placeholder-gray-500 focus:ring-blue-500 focus:border-blue-500'
                : 'bg-white border-gray-300 text-black placeholder-gray-400 focus:ring-blue-500 focus:border-blue-500'"
            />
          </div>

          <div>
            <label for="createPassword" class="block text-sm font-medium mb-2"
              :class="uiStore.isDarkMode ? 'text-gray-300' : 'text-gray-700'"
            >
              Password
            </label>
            <input
              id="createPassword"
              v-model="password"
              type="password"
              placeholder="At least 8 characters"
              autocomplete="new-password"
              class="w-full px-4 py-3 rounded-lg border focus:outline-none focus:ring-2 transition-colors"
              :class="uiStore.isDarkMode
                ? 'bg-gray-900 border-gray-700 text-white placeholder-gray-500 focus:ring-blue-500 focus:border-blue-500'
                : 'bg-white border-gray-300 text-black placeholder-gray-400 focus:ring-blue-500 focus:border-blue-500'"
            />
          </div>

          <div>
            <label for="createApiKey" class="block text-sm font-medium mb-2"
              :class="uiStore.isDarkMode ? 'text-gray-300' : 'text-gray-700'"
            >
              Immich API key <span class="font-normal opacity-60">(optional)</span>
            </label>
            <input
              id="createApiKey"
              v-model="apiKey"
              type="password"
              placeholder="Your Immich API key"
              autocomplete="off"
              class="w-full px-4 py-3 rounded-lg border focus:outline-none focus:ring-2 transition-colors"
              :class="uiStore.isDarkMode
                ? 'bg-gray-900 border-gray-700 text-white placeholder-gray-500 focus:ring-blue-500 focus:border-blue-500'
                : 'bg-white border-gray-300 text-black placeholder-gray-400 focus:ring-blue-500 focus:border-blue-500'"
            />
            <p class="mt-2 text-xs"
              :class="uiStore.isDarkMode ? 'text-gray-500' : 'text-gray-500'"
            >
              New accounts can leave this blank and add the key later in Settings. If this user was previously configured on the server, enter the same Immich API key to claim the account.
            </p>
          </div>
        </template>

        <!-- Error message -->
        <div v-if="error" class="p-3 rounded-lg bg-red-500/20 text-red-400 text-sm" role="alert">
          {{ error }}
        </div>

        <!-- Submit button -->
        <button
          type="submit"
          :disabled="isSubmitting"
          class="w-full py-3 px-4 rounded-lg font-medium transition-colors disabled:opacity-50"
          :class="uiStore.isDarkMode
            ? 'bg-white text-black hover:bg-gray-200'
            : 'bg-black text-white hover:bg-gray-800'"
        >
          <span v-if="isSubmitting" class="flex items-center justify-center gap-2">
            <svg class="w-5 h-5 animate-spin" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            Connecting...
          </span>
          <span v-else>{{ loginMode === 'swipe' ? 'Sign in' : 'Create account' }}</span>
        </button>
      </form>

      <!-- SSO login (only when Immich has OAuth enabled) -->
      <div v-if="authStore.oauthEnabled" class="mt-6">
        <div class="flex items-center gap-3 mb-4">
          <div class="flex-1 h-px"
            :class="uiStore.isDarkMode ? 'bg-gray-800' : 'bg-gray-200'"
          ></div>
          <span class="text-xs"
            :class="uiStore.isDarkMode ? 'text-gray-500' : 'text-gray-500'"
          >
            or
          </span>
          <div class="flex-1 h-px"
            :class="uiStore.isDarkMode ? 'bg-gray-800' : 'bg-gray-200'"
          ></div>
        </div>
        <button
          type="button"
          :disabled="isSubmitting"
          class="w-full py-3 px-4 rounded-lg font-medium border transition-colors disabled:opacity-50"
          :class="uiStore.isDarkMode
            ? 'border-gray-700 bg-gray-900 text-white hover:bg-gray-800'
            : 'border-gray-300 bg-white text-black hover:bg-gray-100'"
          @click="startSso"
        >
          {{ authStore.oauthButtonText || 'Login with SSO' }}
        </button>
      </div>

      <!-- Shared demo account (only when the operator enables it) -->
      <section v-if="authStore.demoEnabled" class="mt-6" aria-labelledby="demo-heading">
        <div
          class="rounded-2xl border p-4 sm:p-5"
          :class="uiStore.isDarkMode
            ? 'border-indigo-400/25 bg-indigo-400/[0.06]'
            : 'border-indigo-200 bg-indigo-50/70'"
        >
          <div class="flex items-start gap-3">
            <span
              class="mt-0.5 inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-xl"
              :class="uiStore.isDarkMode ? 'bg-indigo-400/15 text-indigo-300' : 'bg-indigo-100 text-indigo-600'"
              aria-hidden="true"
            >
              <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M15 19.128a9.38 9.38 0 002.625.372 9.337 9.337 0 004.121-.952 4.125 4.125 0 00-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 018.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0111.964-3.07M12 6.375a3.375 3.375 0 11-6.75 0 3.375 3.375 0 016.75 0zm8.25 2.25a2.625 2.625 0 11-5.25 0 2.625 2.625 0 015.25 0z" />
              </svg>
            </span>
            <div class="min-w-0">
              <h2
                id="demo-heading"
                class="text-sm font-semibold"
                :class="uiStore.isDarkMode ? 'text-white' : 'text-gray-900'"
              >
                Shared demo account
              </h2>
              <p
                class="mt-1 text-xs leading-5"
                :class="uiStore.isDarkMode ? 'text-gray-400' : 'text-gray-600'"
              >
                Anyone can use it to try the app.
              </p>
            </div>
          </div>

          <button
            type="button"
            :disabled="isSubmitting"
            class="mt-4 flex w-full items-center justify-center gap-2 rounded-lg border py-2.5 px-4 text-sm font-medium transition-colors disabled:opacity-50"
            :class="uiStore.isDarkMode
              ? 'border-indigo-400/30 bg-indigo-400/10 text-indigo-200 hover:bg-indigo-400/20'
              : 'border-indigo-200 bg-white text-indigo-700 hover:bg-indigo-50'"
            @click="useDemoAccount"
          >
            <svg
              v-if="demoSubmitting"
              class="h-4 w-4 animate-spin"
              fill="none"
              viewBox="0 0 24 24"
              aria-hidden="true"
            >
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            <span>{{ demoSubmitting ? 'Signing in…' : 'Use the demo account' }}</span>
          </button>

          <p
            v-if="authStore.demoUserName"
            class="mt-2.5 text-center text-[11px]"
            :class="uiStore.isDarkMode ? 'text-gray-500' : 'text-gray-500'"
          >
            You will be signed in as
            <code
              class="rounded px-1 py-0.5 font-mono"
              :class="uiStore.isDarkMode ? 'bg-white/10 text-gray-300' : 'bg-black/5 text-gray-700'"
            >{{ authStore.demoUserName }}</code>
          </p>
        </div>
      </section>

      <!-- Theme toggle -->
      <div class="mt-8 flex justify-center">
        <button
          @click="uiStore.toggleDarkMode"
          class="flex items-center gap-2 text-sm transition-colors"
          :class="uiStore.isDarkMode ? 'text-gray-400 hover:text-white' : 'text-gray-600 hover:text-black'"
        >
          <svg v-if="uiStore.isDarkMode" class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
          </svg>
          <svg v-else class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
          </svg>
        </button>
      </div>

      <!-- Public legal pages: readable without an account. -->
      <div class="mt-6 flex flex-wrap justify-center gap-x-4 gap-y-1 text-xs">
        <a href="/privacy" class="underline hover:no-underline" :class="uiStore.isDarkMode ? 'text-gray-400' : 'text-gray-600'">
          Privacy
        </a>
        <a href="/terms" class="underline hover:no-underline" :class="uiStore.isDarkMode ? 'text-gray-400' : 'text-gray-600'">
          Terms
        </a>
        <a href="/account/delete" class="underline hover:no-underline" :class="uiStore.isDarkMode ? 'text-gray-400' : 'text-gray-600'">
          Delete account
        </a>
      </div>
    </div>
  </div>
</template>
