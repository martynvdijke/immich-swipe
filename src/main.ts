import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { watch } from 'vue'
import App from './App.vue'
import router from './router'
import { useObservabilityStore } from './stores/observability'
import { loadUmami } from './composables/useUmami'
import { initOtel } from './composables/useOtel'
import './style.css'

const app = createApp(App)

const pinia = createPinia()
app.use(pinia)
app.use(router)

// Observability bootstrap: apply the persisted settings on startup and react to
// changes (hot-apply on save, reconfigure, and teardown on disable). Both
// composables no-op when their section is disabled, and the store only holds
// real settings once a user/server session is present.
const observabilityStore = useObservabilityStore()
watch(
  () => observabilityStore.settings,
  async (settings) => {
    await loadUmami(settings.umami)
    await initOtel(settings.otel)
  },
  { immediate: true, deep: true }
)

app.mount('#app')
