<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'

const router = useRouter()
const authStore = useAuthStore()
const uiStore = useUiStore()

async function selectUser(userName: string) {
  const success = await authStore.loginWithUser(userName)
  
  if (success) {
    uiStore.toast(`Welcome, ${userName}!`, 'success')
    router.push('/')
  } else {
    uiStore.toast('Connection failed. Please check the configuration.', 'error')
  }
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
          Who's swiping ?
        </p>
      </div>

      <!-- User buttons -->
      <div class="space-y-4">
        <button
          v-for="name in authStore.envUsers"
          :key="name"
          @click="selectUser(name)"
          class="w-full py-4 px-6 rounded-xl font-medium text-lg transition-all transform hover:scale-102 active:scale-98 shadow-lg flex items-center justify-center gap-3"
          :class="uiStore.isDarkMode
            ? 'bg-gradient-to-r from-indigo-600 to-purple-600 hover:from-indigo-500 hover:to-purple-500 text-white'
            : 'bg-gradient-to-r from-indigo-500 to-purple-500 hover:from-indigo-400 hover:to-purple-400 text-white'"
        >
          <!-- User icon -->
          <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
          </svg>
          {{ name }}
        </button>
      </div>

      <!-- Alternate login for non-env Immich accounts -->
      <div class="mt-6 text-center">
        <button
          type="button"
          class="text-sm underline-offset-2 hover:underline transition-colors"
          :class="uiStore.isDarkMode ? 'text-gray-400 hover:text-white' : 'text-gray-600 hover:text-black'"
          @click="router.push('/login')"
        >
          Sign in with Immich account
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

<style scoped>
.hover\:scale-102:hover {
  transform: scale(1.02);
}
.active\:scale-98:active {
  transform: scale(0.98);
}
</style>