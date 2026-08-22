import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import { useAuthStore } from '@/stores/auth'

type ReviewDecision = 'keep' | 'delete' | 'skip'

interface ReviewedPayload {
  v: 2
  kept: string[]
  deleted: string[]
  skipped?: string[]
}

const STORAGE_PREFIX = 'immich-swipe-reviewed'
const STORAGE_VERSION = 2

export const useReviewedStore = defineStore('reviewed', () => {
  const authStore = useAuthStore()
  const kept = ref<Set<string>>(new Set())
  const deleted = ref<Set<string>>(new Set())
  const skipped = ref<Set<string>>(new Set())
  const initialized = ref(false)

  const storageKey = computed(() => {
    const server = authStore.immichServerUrl || 'unknown-server'
    const user = authStore.currentUserName || 'default-user'
    return `${STORAGE_PREFIX}:${server}:${user}`
  })

  function loadFromStorage() {
    initialized.value = false
    kept.value = new Set()
    deleted.value = new Set()
    skipped.value = new Set()

    const raw = localStorage.getItem(storageKey.value)
    if (!raw) {
      initialized.value = true
      return
    }

    try {
      const parsed = JSON.parse(raw) as Partial<ReviewedPayload>
      const keptIds = Array.isArray(parsed.kept) ? parsed.kept : []
      const deletedIds = Array.isArray(parsed.deleted) ? parsed.deleted : []
      // `skipped` was introduced with payload v2; v1 payloads simply have none.
      const skippedIds = Array.isArray(parsed.skipped) ? parsed.skipped : []
      kept.value = new Set(keptIds.filter((id) => typeof id === 'string'))
      deleted.value = new Set(deletedIds.filter((id) => typeof id === 'string'))
      skipped.value = new Set(skippedIds.filter((id) => typeof id === 'string'))
    } catch (e) {
      console.error('Failed to parse reviewed cache from localStorage', e)
    } finally {
      initialized.value = true
    }
  }

  function persist() {
    if (!initialized.value) return
    const payload: ReviewedPayload = {
      v: STORAGE_VERSION,
      kept: Array.from(kept.value),
      deleted: Array.from(deleted.value),
      skipped: Array.from(skipped.value),
    }
    localStorage.setItem(storageKey.value, JSON.stringify(payload))
  }

  function isReviewed(id: string): boolean {
    return kept.value.has(id) || deleted.value.has(id) || skipped.value.has(id)
  }

  // Number of assets already reviewed (kept + deleted + skipped) in the
  // current server:user scope. Drives the review-progress indicator.
  const reviewedCount = computed(() => kept.value.size + deleted.value.size + skipped.value.size)

  function getDecision(id: string): ReviewDecision | null {
    if (kept.value.has(id)) return 'keep'
    if (deleted.value.has(id)) return 'delete'
    if (skipped.value.has(id)) return 'skip'
    return null
  }

  function markReviewed(id: string, decision: ReviewDecision) {
    if (!id) return
    if (decision === 'keep') {
      kept.value.add(id)
      deleted.value.delete(id)
      skipped.value.delete(id)
    } else if (decision === 'delete') {
      deleted.value.add(id)
      kept.value.delete(id)
      skipped.value.delete(id)
    } else {
      skipped.value.add(id)
      kept.value.delete(id)
      deleted.value.delete(id)
    }
    persist()
  }

  function unmarkReviewed(id: string) {
    if (!id) return
    kept.value.delete(id)
    deleted.value.delete(id)
    skipped.value.delete(id)
    persist()
  }

  function resetReviewed() {
    const user = authStore.currentUserName || 'default-user'
    const prefix = `${STORAGE_PREFIX}:`
    const keysToRemove: string[] = []

    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i)
      if (!key || !key.startsWith(prefix)) continue
      if (key.endsWith(`:${user}`)) {
        keysToRemove.push(key)
      }
    }

    if (keysToRemove.length === 0) {
      localStorage.removeItem(storageKey.value)
    } else {
      keysToRemove.forEach((key) => localStorage.removeItem(key))
    }
    loadFromStorage()
  }

  watch(storageKey, () => loadFromStorage(), { immediate: true })

  return {
    isReviewed,
    getDecision,
    markReviewed,
    unmarkReviewed,
    resetReviewed,
    reviewedCount,
  }
})
