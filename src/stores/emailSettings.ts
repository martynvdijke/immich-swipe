import { defineStore } from 'pinia'
import { ref } from 'vue'
import { useAuthStore } from './auth'

export const useEmailSettingsStore = defineStore('emailSettings', () => {
  const host = ref('')
  const port = ref<number>(0)
  const user = ref('')
  const from = ref('')
  const tls = ref('starttls')
  const hasPassword = ref(false)
  const fromEnv = ref(false)
  const configured = ref(false)
  const loading = ref(false)
  const error = ref('')

  async function fetch() {
    loading.value = true
    error.value = ''
    try {
      const auth = useAuthStore()
      const res = await window.fetch('/api/admin/email', { headers: auth.authHeader as Record<string,string> })
      if (!res.ok) throw new Error('failed')
      const data = await res.json()
      host.value = data.smtp_host || ''
      port.value = data.smtp_port || 0
      user.value = data.smtp_user || ''
      from.value = data.smtp_from || ''
      tls.value = data.smtp_tls || 'starttls'
      hasPassword.value = !!data.has_password
      fromEnv.value = !!data.from_env
      configured.value = !!data.configured
    } catch (e: unknown) {
      error.value = e instanceof Error ? e.message : 'failed'
    } finally { loading.value = false }
  }

  async function save(payload: { host: string; port: number; user: string; pass: string; from: string; tls: string }) {
    const auth = useAuthStore()
    const res = await window.fetch('/api/admin/email', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', ...(auth.authHeader as Record<string,string>) },
      body: JSON.stringify({ smtp_host: payload.host, smtp_port: payload.port, smtp_user: payload.user, smtp_pass: payload.pass, smtp_from: payload.from, smtp_tls: payload.tls }),
    })
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      throw new Error(data.error || 'save failed')
    }
    await fetch()
  }

  async function sendTest(to: string) {
    const auth = useAuthStore()
    const res = await window.fetch('/api/admin/email/test', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...(auth.authHeader as Record<string,string>) },
      body: JSON.stringify({ to }),
    })
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      throw new Error(data.error || 'send failed')
    }
  }

  return { host, port, user, from, tls, hasPassword, fromEnv, configured, loading, error, fetch, save, sendTest }
})
