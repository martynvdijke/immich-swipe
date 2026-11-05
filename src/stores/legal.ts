import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface LegalConfig {
  appName: string
  controller: string
  contactEmail: string
  contactAddress: string
  hostingRegion: string
  defaultServerUrl: string
  smtpConfigured: boolean
  telemetryConfigured: boolean
  privacyUrl: string
  termsUrl: string
  deletionUrl: string
}

function defaults(): LegalConfig {
  return {
    appName: 'Immich Swipe',
    controller: 'the operator of this Immich Swipe instance',
    contactEmail: '',
    contactAddress: '',
    hostingRegion: 'the European Union',
    defaultServerUrl: '',
    smtpConfigured: false,
    telemetryConfigured: false,
    privacyUrl: '/privacy',
    termsUrl: '/terms',
    deletionUrl: '/account/delete',
  }
}

/**
 * Public deployment facts rendered by the legal pages. Fetched from
 * GET /api/legal, which needs no session, so the pages stay readable before
 * sign-up and during store review.
 */
export const useLegalStore = defineStore('legal', () => {
  const config = ref<LegalConfig>(defaults())
  const loaded = ref(false)
  const error = ref('')
  let pending: Promise<void> | null = null

  async function fetchLegal(): Promise<void> {
    if (pending) return pending
    pending = (async () => {
      try {
        const res = await fetch('/api/legal')
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        const data = (await res.json()) as Partial<LegalConfig>
        config.value = { ...defaults(), ...data }
        loaded.value = true
        error.value = ''
      } catch {
        // Keep neutral fallbacks: the policy remains readable if the API is
        // briefly unavailable, without inventing a controller or contact.
        error.value = 'Could not load this instance’s policy details.'
      } finally {
        pending = null
      }
    })()
    return pending
  }

  return { config, loaded, error, fetchLegal }
})
