import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import { useAuthStore } from '@/stores/auth'
import {
  defaultObservabilitySettings,
  normalizeBaseUrl,
  type ObservabilitySettings,
  type OtelSettings,
  type UmamiSettings,
} from '@/types/observability'

const STORAGE_PREFIX = 'immich-swipe-observability'

interface StoredObservability {
  umami: UmamiSettings
  otel: OtelSettings
}

export const useObservabilityStore = defineStore('observability', () => {
  const authStore = useAuthStore()

  const settings = ref<ObservabilitySettings>(defaultObservabilitySettings())
  const initialized = ref(false)

  const storageKey = computed(() => {
    const server = authStore.immichServerUrl || 'unknown-server'
    const user = authStore.currentUserName || 'default-user'
    return `${STORAGE_PREFIX}:${server}:${user}`
  })

  function loadFromStorage() {
    initialized.value = false
    const raw = localStorage.getItem(storageKey.value)
    if (!raw) {
      settings.value = defaultObservabilitySettings()
      initialized.value = true
      return
    }

    try {
      const parsed = JSON.parse(raw) as Partial<StoredObservability>
      const defaults = defaultObservabilitySettings()
      settings.value = {
        umami: {
          enabled: parsed.umami?.enabled ?? defaults.umami.enabled,
          serverUrl: normalizeBaseUrl(parsed.umami?.serverUrl ?? defaults.umami.serverUrl),
          websiteId: parsed.umami?.websiteId ?? defaults.umami.websiteId,
          hostUrl: normalizeBaseUrl(parsed.umami?.hostUrl ?? defaults.umami.hostUrl),
        },
        otel: {
          enabled: parsed.otel?.enabled ?? defaults.otel.enabled,
          endpoint: normalizeBaseUrl(parsed.otel?.endpoint ?? defaults.otel.endpoint),
          samplingPercent: parsed.otel?.samplingPercent ?? defaults.otel.samplingPercent,
        },
      }
    } catch (e) {
      console.error('Failed to parse observability settings from localStorage', e)
      settings.value = defaultObservabilitySettings()
    } finally {
      initialized.value = true
    }
  }

  function persist() {
    if (!initialized.value) return
    const payload: StoredObservability = {
      umami: { ...settings.value.umami },
      otel: { ...settings.value.otel },
    }
    localStorage.setItem(storageKey.value, JSON.stringify(payload))
  }

  function setUmami(patch: Partial<UmamiSettings>) {
    settings.value.umami = {
      ...settings.value.umami,
      ...patch,
      serverUrl: normalizeBaseUrl(patch.serverUrl ?? settings.value.umami.serverUrl),
      hostUrl: normalizeBaseUrl(patch.hostUrl ?? settings.value.umami.hostUrl),
    }
  }

  function setOtel(patch: Partial<OtelSettings>) {
    settings.value.otel = {
      ...settings.value.otel,
      ...patch,
      endpoint: normalizeBaseUrl(patch.endpoint ?? settings.value.otel.endpoint),
    }
  }

  // Load on init and whenever user/server changes
  watch(storageKey, () => loadFromStorage(), { immediate: true })

  // Persist on changes (deep so nested umami/otel objects are tracked)
  watch(settings, () => persist(), { deep: true })

  return {
    settings,
    initialized,
    setUmami,
    setOtel,
  }
})
