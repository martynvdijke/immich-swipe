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

  // Accessing login page
  if (to.path === '/login') {
    if (authStore.envUsers.length === 1) {
      // Single env user -> auto-login via backend
      const ok = await authStore.loginWithUser(authStore.envUsers[0])
      if (ok) {
        next('/')
      } else {
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
      const ok = await authStore.loginWithUser(authStore.envUsers[0])
      if (ok) {
        next('/')
      } else {
        next('/login')
      }
    } else {
      next()
    }
    return
  }

  // Protected routes
  if (to.meta.requiresAuth) {
    if (authStore.envUsers.length === 1) {
      const ok = await authStore.loginWithUser(authStore.envUsers[0])
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
