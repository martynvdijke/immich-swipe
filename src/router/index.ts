import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { trackPageView } from '@/composables/useUmami'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'home',
      component: () => import('@/views/HomeView.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('@/views/SettingsView.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
    },
    {
      path: '/select-user',
      name: 'select-user',
      component: () => import('@/views/UserSelectView.vue'),
    },
  ],
})

// Track page views in Umami on every navigation (no-op until script loaded).
router.afterEach((to) => {
  trackPageView(to.fullPath)
})

// Navigation guard
router.beforeEach(async (to, _from, next) => {
  const authStore = useAuthStore()

  // Restore a persisted session after reload when none is active yet.
  if (!authStore.isLoggedIn && authStore.sessionCount > 0) {
    authStore.restoreLastActive()
  }

  // Logged in with session token -> home
  if (authStore.isLoggedIn) {
    // /login stays reachable while logged in: it is the "add person" flow.
    // /select-user is env-user selection for the not-logged-in case only.
    if (to.path === '/select-user') {
      next('/')
    } else {
      next()
    }
    return
  }

  // Not logged in -> check backend config for env users
  try {
    await authStore.fetchConfig()
  } catch {
    // Backend unavailable, proceed to login
  }

  // Helper: attempt auto-login for a single env user, but only if the
  // loop guard has not been tripped by a prior 401 or failed auto-login.
  // passwordRequired is true when the user must enter the account password
  // (auto-login with userName alone is disabled for that account).
  async function tryAutoLogin(): Promise<{ ok: boolean; passwordRequired: boolean } | null> {
    if (authStore.envUsers.length !== 1) return null
    if (authStore.autoLoginBlocked) return { ok: false, passwordRequired: false }
    const result = await authStore.loginWithUser(authStore.envUsers[0])
    if (!result.ok) {
      // Block further auto-login attempts to break the login loop.
      authStore.autoLoginBlocked = true
      return { ok: false, passwordRequired: result.code === 'password_required' }
    }
    return { ok: true, passwordRequired: false }
  }

  // Accessing login page
  if (to.path === '/login') {
    if (authStore.pendingPasswordUser) {
      // Redirected here because the account needs its password.
      next()
    } else if (authStore.envUsers.length === 1) {
      // Single env user -> auto-login via backend (unless blocked)
      const result = await tryAutoLogin()
      if (result?.ok) {
        next('/')
      } else if (result?.passwordRequired) {
        // Account has a password: let the person type it on the login page.
        authStore.pendingPasswordUser = authStore.envUsers[0]
        next()
      } else {
        // Blocked or failed -> stay on manual login
        next()
      }
    } else if (authStore.envUsers.length > 1) {
      // Multi user -> selection
      next('/select-user')
    } else {
      // No env users -> manual login
      next()
    }
    return
  }

  // Accessing user selection
  if (to.path === '/select-user') {
    if (authStore.envUsers.length === 0) {
      next('/login')
    } else if (authStore.envUsers.length === 1) {
      const result = await tryAutoLogin()
      if (result?.ok) {
        next('/')
      } else if (result?.passwordRequired) {
        authStore.pendingPasswordUser = authStore.envUsers[0]
        next('/login')
      } else {
        next('/login')
      }
    } else {
      // Multi-user: allow manual selection even if autoLoginBlocked is set
      next()
    }
    return
  }

  // Protected routes
  if (to.meta.requiresAuth) {
    if (authStore.envUsers.length === 1) {
      const result = await tryAutoLogin()
      if (result?.ok) {
        next()
      } else if (result?.passwordRequired) {
        authStore.pendingPasswordUser = authStore.envUsers[0]
        next('/login')
      } else {
        next('/login')
      }
    } else if (authStore.envUsers.length > 1) {
      next('/select-user')
    } else {
      next('/login')
    }
    return
  }

  next()
})

export default router
