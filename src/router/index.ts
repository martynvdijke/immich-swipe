import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

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

// Navigation guard
router.beforeEach(async (to, _from, next) => {
  const authStore = useAuthStore()

  // Logged in with session token -> home
  if (authStore.isLoggedIn) {
    if (to.path === '/login' || to.path === '/select-user') {
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
  async function tryAutoLogin(): Promise<boolean | null> {
    if (authStore.envUsers.length !== 1) return null
    if (authStore.autoLoginBlocked) return false
    const ok = await authStore.loginWithUser(authStore.envUsers[0])
    if (!ok) {
      // Block further auto-login attempts to break the login loop.
      authStore.autoLoginBlocked = true
    }
    return ok
  }

  // Accessing login page
  if (to.path === '/login') {
    if (authStore.envUsers.length === 1) {
      // Single env user -> auto-login via backend (unless blocked)
      const ok = await tryAutoLogin()
      if (ok) {
        next('/')
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
      const ok = await tryAutoLogin()
      if (ok) {
        next('/')
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
      const ok = await tryAutoLogin()
      if (ok) {
        next()
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
