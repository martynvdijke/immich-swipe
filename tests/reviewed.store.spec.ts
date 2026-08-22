import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { useAuthStore } from '@/stores/auth'
import { useReviewedStore } from '@/stores/reviewed'
import { seedAuthSession, seedAuthSessions } from './helpers/seedAuth'

describe('reviewed store', () => {
  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia())
  })

  it('persists keep/delete decisions', async () => {
    seedAuthSession('test-token', 'Alice', 'http://server-a')

    const reviewed = useReviewedStore()
    reviewed.markReviewed('asset-1', 'keep')
    reviewed.markReviewed('asset-2', 'delete')

    await nextTick()

    expect(reviewed.isReviewed('asset-1')).toBe(true)
    expect(reviewed.getDecision('asset-1')).toBe('keep')
    expect(reviewed.getDecision('asset-2')).toBe('delete')

    const key = Object.keys(localStorage).find((k) => k.startsWith('immich-swipe-reviewed'))
    const stored = JSON.parse(localStorage.getItem(key || '') || '{}')
    expect(stored.kept).toContain('asset-1')
    expect(stored.deleted).toContain('asset-2')

    reviewed.unmarkReviewed('asset-1')
    await nextTick()
    expect(reviewed.isReviewed('asset-1')).toBe(false)
  })

  it('scopes cache by server/user', async () => {
    seedAuthSessions(
      [
        { token: 't-alice', userName: 'Alice', serverUrl: 'http://server-a' },
        { token: 't-bob', userName: 'Bob', serverUrl: 'http://server-b' },
      ],
      'http://server-a|Alice',
    )
    const auth = useAuthStore()

    const reviewed = useReviewedStore()
    reviewed.markReviewed('asset-1', 'keep')
    await nextTick()

    auth.switchTo('http://server-b|Bob')
    await nextTick()

    expect(reviewed.isReviewed('asset-1')).toBe(false)

    // Only one key persisted — switching namespaces doesn't persist empty state
    const keys = Object.keys(localStorage).filter((k) => k.startsWith('immich-swipe-reviewed'))
    expect(keys.length).toBe(1)
  })

  it('persists skip decisions and counts them as reviewed', async () => {
    seedAuthSession('test-token', 'Alice', 'http://server-a')

    const reviewed = useReviewedStore()
    reviewed.markReviewed('asset-s', 'skip')

    await nextTick()

    expect(reviewed.isReviewed('asset-s')).toBe(true)
    expect(reviewed.getDecision('asset-s')).toBe('skip')
    expect(reviewed.reviewedCount).toBe(1)

    const key = Object.keys(localStorage).find((k) => k.startsWith('immich-swipe-reviewed'))
    const stored = JSON.parse(localStorage.getItem(key || '') || '{}')
    expect(stored.v).toBe(2)
    expect(stored.skipped).toContain('asset-s')
    expect(stored.kept).not.toContain('asset-s')

    reviewed.unmarkReviewed('asset-s')
    await nextTick()
    expect(reviewed.isReviewed('asset-s')).toBe(false)
  })

  it('re-deciding a skipped asset moves it out of skipped', async () => {
    seedAuthSession('test-token', 'Alice', 'http://server-a')

    const reviewed = useReviewedStore()
    reviewed.markReviewed('asset-1', 'skip')
    reviewed.markReviewed('asset-1', 'keep')

    await nextTick()

    expect(reviewed.getDecision('asset-1')).toBe('keep')
    const key = Object.keys(localStorage).find((k) => k.startsWith('immich-swipe-reviewed'))
    const stored = JSON.parse(localStorage.getItem(key || '') || '{}')
    expect(stored.skipped).not.toContain('asset-1')
  })

  it('loads legacy v1 payloads without a skipped array', async () => {
    seedAuthSession('test-token', 'Alice', 'http://server-a')
    localStorage.setItem(
      'immich-swipe-reviewed:http://server-a:Alice',
      JSON.stringify({ v: 1, kept: ['k1'], deleted: ['d1'] })
    )

    const reviewed = useReviewedStore()
    await nextTick()

    expect(reviewed.isReviewed('k1')).toBe(true)
    expect(reviewed.isReviewed('d1')).toBe(true)
    expect(reviewed.reviewedCount).toBe(2)
  })

  it('exposes reviewedCount as kept + deleted sizes', () => {
    seedAuthSession('test-token', 'Alice', 'http://server-a')

    const reviewed = useReviewedStore()
    expect(reviewed.reviewedCount).toBe(0)

    reviewed.markReviewed('a', 'keep')
    expect(reviewed.reviewedCount).toBe(1)

    reviewed.markReviewed('b', 'delete')
    expect(reviewed.reviewedCount).toBe(2)

    // Same decision twice must not double count
    reviewed.markReviewed('b', 'delete')
    expect(reviewed.reviewedCount).toBe(2)

    reviewed.unmarkReviewed('a')
    expect(reviewed.reviewedCount).toBe(1)
  })
})
