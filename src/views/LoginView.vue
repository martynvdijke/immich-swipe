<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'

type LoginMode = 'account' | 'apiKey' | 'swipe' | 'create'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const uiStore = useUiStore()

const loginMode = ref<LoginMode>('account')
const serverUrl = ref(authStore.immichServerUrl || authStore.defaultServerUrl || '')
const email = ref('')
const password = ref('')
const apiKey = ref('')
const userName = ref('')
const error = ref('')
const isSubmitting = ref(false)

// Pre-fill the user name when the router redirected here because the account
// password is required, or when a URL query names a user.
const pendingUser = authStore.pendingPasswordUser
if (pendingUser) {
  userName.value = pendingUser
  loginMode.value = 'swipe'
  authStore.pendingPasswordUser = null
} else {
  const queryUser = typeof route.query.user === 'string' ? route.query.user : ''
  if (queryUser) {
    userName.value = queryUser
    loginMode.value = 'swipe'
  }
}

function setMode(mode: LoginMode) {
  loginMode.value = mode
  error.value = ''
}

/** One-click login with an env-configured user (server-side API key). */
async function pickUser(name: string) {
  error.value = ''
  isSubmitting.value = true
  const result = await authStore.loginWithUser(name)
  if (result.ok) {
    uiStore.toast('Connected successfully!', 'success')
    router.push('/')
  } else if (result.code === 'password_required') {
    userName.value = name
    loginMode.value = 'swipe'
    error.value = 'This account has a password. Enter it to sign in.'
  } else {
    error.value = result.error
  }
  isSubmitting.value = false
}

async function handleSubmit() {
  error.value = ''
  if (!serverUrl.value.trim()) {
    error.value = 'Please enter your Immich server URL'
    return
  }

  isSubmitting.value = true

  if (loginMode.value === 'account') {
    if (!email.value.trim()) {
      error.value = 'Please enter your email'
      isSubmitting.value = false
      return
    }
    if (!password.value) {
      error.value = 'Please enter your password'
      isSubmitting.value = false
      return
    }

    const result = await authStore.loginWithCredentials(
      email.value.trim(),
      password.value,
      serverUrl.value.trim(),
    )

    if (result.ok) {
      uiStore.toast('Connected successfully!', 'success')
      router.push('/')
    } else {
      error.value = result.error
    }
  } else if (loginMode.value === 'swipe') {
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
      uiStore.toast('Connected successfully!', 'success')
      router.push('/')
    } else {
      error.value = result.error
    }
  } else if (loginMode.value === 'create') {
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
    if (!apiKey.value.trim()) {
      error.value = 'Please enter your Immich API key'
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
      uiStore.toast('Account created — connected successfully!', 'success')
      router.push('/')
    } else {
      error.value = result.error
    }
  } else {
    if (!apiKey.value.trim()) {
      error.value = 'Please enter your API key'
      isSubmitting.value = false
      return
    }

    const success = await authStore.loginManual(apiKey.value.trim(), serverUrl.value.trim())

    if (success) {
      uiStore.toast('Connected successfully!', 'success')
      router.push('/')
    } else {
      error.value = 'Failed to connect. Please check your URL and API key.'
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

      <!-- Configured users: one-click login -->
      <div v-if="authStore.envUsers.length > 0" class="mb-6">
        <p class="text-sm font-medium mb-2"
          :class="uiStore.isDarkMode ? 'text-gray-300' : 'text-gray-700'"
        >
          Who&rsquo;s swiping?
        </p>
        <div class="flex flex-wrap gap-2">
          <button
            v-for="user in authStore.envUsers"
            :key="user"
            type="button"
            :disabled="isSubmitting"
            class="px-4 py-2 rounded-lg text-sm font-medium border transition-colors disabled:opacity-50"
            :class="uiStore.isDarkMode
              ? 'border-gray-700 bg-gray-900 text-white hover:bg-gray-800'
              : 'border-gray-300 bg-white text-black hover:bg-gray-100'"
            @click="pickUser(user)"
          >
            {{ user }}
          </button>
        </div>
        <div class="my-6 flex items-center gap-3">
          <div class="flex-1 h-px"
            :class="uiStore.isDarkMode ? 'bg-gray-800' : 'bg-gray-200'"
          ></div>
          <span class="text-xs"
            :class="uiStore.isDarkMode ? 'text-gray-500' : 'text-gray-500'"
          >
            or sign in another way
          </span>
          <div class="flex-1 h-px"
            :class="uiStore.isDarkMode ? 'bg-gray-800' : 'bg-gray-200'"
          ></div>
        </div>
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
          :aria-selected="loginMode === 'account'"
          class="py-2 px-3 rounded-lg text-sm font-medium transition-colors"
          :class="loginMode === 'account'
            ? (uiStore.isDarkMode ? 'bg-white text-black' : 'bg-black text-white')
            : (uiStore.isDarkMode ? 'text-gray-400 hover:text-white' : 'text-gray-600 hover:text-black')"
          @click="setMode('account')"
        >
          Immich account
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="loginMode === 'apiKey'"
          class="py-2 px-3 rounded-lg text-sm font-medium transition-colors"
          :class="loginMode === 'apiKey'
            ? (uiStore.isDarkMode ? 'bg-white text-black' : 'bg-black text-white')
            : (uiStore.isDarkMode ? 'text-gray-400 hover:text-white' : 'text-gray-600 hover:text-black')"
          @click="setMode('apiKey')"
        >
          API key
        </button>
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
          Swipe account
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

        <!-- Account fields -->
        <template v-if="loginMode === 'account'">
          <div>
            <label for="email" class="block text-sm font-medium mb-2"
              :class="uiStore.isDarkMode ? 'text-gray-300' : 'text-gray-700'"
            >
              Email
            </label>
            <input
              id="email"
              v-model="email"
              type="email"
              placeholder="you@example.com"
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
              placeholder="Your Immich password"
              autocomplete="current-password"
              class="w-full px-4 py-3 rounded-lg border focus:outline-none focus:ring-2 transition-colors"
              :class="uiStore.isDarkMode
                ? 'bg-gray-900 border-gray-700 text-white placeholder-gray-500 focus:ring-blue-500 focus:border-blue-500'
                : 'bg-white border-gray-300 text-black placeholder-gray-400 focus:ring-blue-500 focus:border-blue-500'"
            />
            <p class="mt-2 text-xs"
              :class="uiStore.isDarkMode ? 'text-gray-500' : 'text-gray-500'"
            >
              Uses Immich password login. Password login must be enabled on your Immich server.
            </p>
          </div>
        </template>

        <!-- Swipe account fields -->
        <template v-else-if="loginMode === 'swipe'">
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
            <p class="mt-2 text-xs"
              :class="uiStore.isDarkMode ? 'text-gray-500' : 'text-gray-500'"
            >
              Uses the password set for your local Swipe account (Settings → Account).
            </p>
          </div>
        </template>

        <!-- API Key fields -->
        <div v-else-if="loginMode === 'apiKey'">
          <label for="apiKey" class="block text-sm font-medium mb-2"
            :class="uiStore.isDarkMode ? 'text-gray-300' : 'text-gray-700'"
          >
            API Key
          </label>
          <input
            id="apiKey"
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
            Find your API key in Immich: Account Settings → API Keys
          </p>
        </div>

        <!-- Create account fields -->
        <template v-else-if="loginMode === 'create'">
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
              placeholder="At least 8 characters"
              autocomplete="new-password"
              class="w-full px-4 py-3 rounded-lg border focus:outline-none focus:ring-2 transition-colors"
              :class="uiStore.isDarkMode
                ? 'bg-gray-900 border-gray-700 text-white placeholder-gray-500 focus:ring-blue-500 focus:border-blue-500'
                : 'bg-white border-gray-300 text-black placeholder-gray-400 focus:ring-blue-500 focus:border-blue-500'"
            />
            <p class="mt-2 text-xs"
              :class="uiStore.isDarkMode ? 'text-gray-500' : 'text-gray-500'"
            >
              Your password only protects this Swipe account and stays on this server.
            </p>
          </div>

          <div>
            <label for="apiKey" class="block text-sm font-medium mb-2"
              :class="uiStore.isDarkMode ? 'text-gray-300' : 'text-gray-700'"
            >
              Immich API key
            </label>
            <input
              id="apiKey"
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
              Find your API key in Immich: Account Settings → API Keys
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
          <span v-else>Connect</span>
        </button>
      </form>

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
