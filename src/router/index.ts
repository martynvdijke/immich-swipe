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
      // User selection is part of the login page now. Logged-out visitors are
      // sent to /login by the guard; logged-in visitors reach the home view.
      path: '/select-user',
      redirect: '/',
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

  // Logged in with session token -> allow navigation. /login stays reachable
  // while logged in: it is the "add person" flow.
  if (authStore.isLoggedIn) {
    next()
    return
  }

  // Not logged in -> the login page is the landing point. It shows the
  // configured users as one-click options and the manual login/account
  // creation forms. There is deliberately no auto-login: a fresh visit
  // always presents the login screen, even with a single env user.
  try {
    await authStore.fetchConfig()
  } catch {
    // Backend unavailable, proceed to login
  }

  if (to.path === '/login') {
    next()
    return
  }

  next('/login')
})

export default router
