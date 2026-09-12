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
</script>

<template>
  <div class="min-h-screen flex flex-col items-center justify-center p-6"
    :class="uiStore.isDarkMode ? 'bg-black text-white' : 'bg-white text-black'"
  >
    <div class="w-full max-w-md">
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
        <div v-if="error" class="p-3 rounded-lg bg-red-500/20 text-red-400 text-sm">
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
    </div>
  </div>
</template>
