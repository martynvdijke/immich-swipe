<script setup lang="ts">
import { RouterView } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import LoadingOverlay from '@/components/LoadingOverlay.vue'
import ToastNotification from '@/components/ToastNotification.vue'

const authStore = useAuthStore()
</script>

<template>
  <RouterView />
  <LoadingOverlay />
  <ToastNotification />

  <!-- Version badge -->
  <span
    v-if="authStore.serverVersion"
    class="fixed bottom-2 right-2 z-50 px-2 py-0.5 text-[10px] rounded-full opacity-40 hover:opacity-80 transition-opacity pointer-events-none select-none"
    :class="'bg-black/20 text-white/50'"
  >
    v{{ authStore.serverVersion }}
  </span>
</template>
