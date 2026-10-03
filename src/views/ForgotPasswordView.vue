<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'

const router = useRouter()
const authStore = useAuthStore()
const uiStore = useUiStore()
const serverUrl = ref(authStore.immichServerUrl || authStore.defaultServerUrl || '')
const userName = ref('')
const error = ref('')
const success = ref(false)
const submitting = ref(false)

async function submit() {
  error.value = ''
  if (!userName.value.trim()) {
    error.value = 'Please enter your user name or email'
    return
  }
  submitting.value = true
  const result = await authStore.forgotPassword(serverUrl.value.trim(), userName.value.trim())
  submitting.value = false
  if (result.ok) {
    success.value = true
  } else {
    error.value = result.error
  }
}
</script>
<template>
  <div class="min-h-screen flex flex-col items-center justify-center p-6" :class="uiStore.isDarkMode ? 'bg-black text-white' : 'bg-white text-black'">
    <div class="w-full max-w-md">
      <h1 class="text-2xl font-bold mb-4">Forgot password</h1>
      <p v-if="success" class="text-sm text-green-600 mb-4">If an account exists, a reset email has been sent. Check your inbox.</p>
      <form v-else @submit.prevent="submit" class="space-y-4">
        <div>
          <label class="block text-sm font-medium mb-1">Immich Server URL</label>
          <input v-model="serverUrl" type="url" placeholder="https://immich.example.com" class="w-full px-4 py-3 rounded-lg border" />
        </div>
        <div>
          <label class="block text-sm font-medium mb-1">User name or email</label>
          <input v-model="userName" type="text" placeholder="Your user name or email" class="w-full px-4 py-3 rounded-lg border" />
        </div>
        <div v-if="error" class="text-sm text-red-500">{{ error }}</div>
        <button type="submit" :disabled="submitting" class="w-full py-3 rounded-lg font-medium bg-black text-white disabled:opacity-50">{{ submitting ? 'Sending...' : 'Send reset link' }}</button>
      </form>
      <button class="mt-4 text-sm underline" @click="router.push('/login')">Back to login</button>
    </div>
  </div>
</template>
