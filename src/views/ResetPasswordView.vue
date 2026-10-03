<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const uiStore = useUiStore()
const token = ref(typeof route.query.token === 'string' ? route.query.token : '')
const password = ref('')
const confirm = ref('')
const error = ref('')
const success = ref(false)
const submitting = ref(false)

async function submit() {
  error.value = ''
  if (!token.value) { error.value = 'Missing reset token'; return }
  if (password.value.length < 8) { error.value = 'Password must be at least 8 characters'; return }
  if (password.value !== confirm.value) { error.value = 'Passwords do not match'; return }
  submitting.value = true
  const result = await authStore.resetPassword(token.value, password.value)
  submitting.value = false
  if (result.ok) {
    success.value = true
    setTimeout(() => router.push('/login'), 1500)
  } else {
    error.value = result.error
  }
}
</script>
<template>
  <div class="min-h-screen flex flex-col items-center justify-center p-6" :class="uiStore.isDarkMode ? 'bg-black text-white' : 'bg-white text-black'">
    <div class="w-full max-w-md">
      <h1 class="text-2xl font-bold mb-4">Reset password</h1>
      <p v-if="success" class="text-sm text-green-600 mb-4">Password reset successful. Redirecting to login...</p>
      <form v-else @submit.prevent="submit" class="space-y-4">
        <div>
          <label class="block text-sm font-medium mb-1">New password</label>
          <input v-model="password" type="password" placeholder="At least 8 characters" class="w-full px-4 py-3 rounded-lg border" />
        </div>
        <div>
          <label class="block text-sm font-medium mb-1">Confirm password</label>
          <input v-model="confirm" type="password" placeholder="Repeat password" class="w-full px-4 py-3 rounded-lg border" />
        </div>
        <div v-if="error" class="text-sm text-red-500">{{ error }}</div>
        <button type="submit" :disabled="submitting" class="w-full py-3 rounded-lg font-medium bg-black text-white disabled:opacity-50">{{ submitting ? 'Saving...' : 'Reset password' }}</button>
      </form>
    </div>
  </div>
</template>
