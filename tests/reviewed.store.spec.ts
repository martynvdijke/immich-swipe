import { createPinia, setActivePinia } from 'pinia'
import { useAuthStore } from '@/stores/auth'
import { useReviewedStore } from '@/stores/reviewed'

describe('reviewed store', () => {
  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia())
  })

  it('persists keep/delete decisions', () => {
    const auth = useAuthStore()
    auth.immichServerUrl = 'http://server-a'
    auth.currentUserName = 'Alice'

    const reviewed = useReviewedStore()
    reviewed.markReviewed('asset-1', 'keep')
    reviewed.markReviewed('asset-2', 'delete')

    expect(reviewed.isReviewed('asset-1')).toBe(true)
    expect(reviewed.getDecision('asset-1')).toBe('keep')
    expect(reviewed.getDecision('asset-2')).toBe('delete')

    const key = Object.keys(localStorage).find((k) => k.startsWith('immich-swipe-reviewed'))
    const stored = JSON.parse(localStorage.getItem(key || '') || '{}')
    expect(stored.kept).toContain('asset-1')
    expect(stored.deleted).toContain('asset-2')

    reviewed.unmarkReviewed('asset-1')
    expect(reviewed.isReviewed('asset-1')).toBe(false)
  })

  it('scopes cache by server/user', () => {
    const auth = useAuthStore()
    auth.immichServerUrl = 'http://server-a'
    auth.currentUserName = 'Alice'

    const reviewed = useReviewedStore()
    reviewed.markReviewed('asset-1', 'keep')

    auth.immichServerUrl = 'http://server-b'
    auth.currentUserName = 'Bob'
    expect(reviewed.isReviewed('asset-1')).toBe(false)

    const keys = Object.keys(localStorage).filter((k) => k.startsWith('immich-swipe-reviewed'))
    expect(keys.length).toBe(2)
  })
})
