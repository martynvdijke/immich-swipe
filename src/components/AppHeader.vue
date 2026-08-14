<script setup lang="ts">
import { useUiStore } from '@/stores/ui'
import { useAuthStore } from '@/stores/auth'
import { useRouter } from 'vue-router'
import { usePreferencesStore } from '@/stores/preferences'
import { useReviewedStore } from '@/stores/reviewed'
import { computed, ref } from 'vue'

const props = defineProps<{
  /** Human-readable label of the active review scope; empty = library feed. */
  scopeLabel?: string
  /** Human-readable label of the selected person; empty = everyone. */
  personLabel?: string
  /** Total reviewable assets in the active feed; null/0 hides the progress bar. */
  reviewTotal?: number | null
}>()

const emit = defineEmits<{
  openScopePicker: []
  openPersonPicker: []
}>()

const uiStore = useUiStore()
const authStore = useAuthStore()
const preferencesStore = usePreferencesStore()
const reviewedStore = useReviewedStore()
const router = useRouter()
const showResetModal = ref(false)
const personMenuOpen = ref(false)

const hasActiveScope = computed(() => props.scopeLabel != null && props.scopeLabel.length > 0)
const hasActivePerson = computed(() => props.personLabel != null && props.personLabel.length > 0)

// Review progress: hidden when the total is unknown or zero; capped at 100%.
const total = computed(() => props.reviewTotal ?? 0)
const progressPercent = computed(() => {
  if (total.value <= 0) return 0
  return Math.min(100, Math.round((reviewedStore.reviewedCount / total.value) * 100))
})
const showProgress = computed(() => total.value > 0)
const libraryReviewed = computed(
  () => showProgress.value && reviewedStore.reviewedCount >= total.value
)

function selectPerson(key: string) {
  authStore.switchTo(key)
  personMenuOpen.value = false
  // Identity watchers in HomeView + per-user stores reload the new person's state.
}

function addPerson() {
  personMenuOpen.value = false
  router.push('/login')
}

function signOutPerson(key: string) {
  const wasActive = key === authStore.activeSessionKey
  const remaining = wasActive ? authStore.logout() : authStore.logoutSession(key)
  personMenuOpen.value = false
  if (remaining) {
    // Another session became active automatically; identity watchers reload.
    uiStore.toast(`Signed out — switched to ${authStore.currentUserName}`, 'info')
  } else {
    // No sessions left -> the login page (shows configured users + manual forms)
    router.push('/login')
  }
}

function toggleReviewOrder() {
  const current = preferencesStore.reviewOrder
  const next =
    current === 'random'
      ? 'chronological'
      : current === 'chronological'
        ? 'chronological-desc'
        : 'random'
  preferencesStore.setReviewOrder(next)
}

function openResetModal() {
  showResetModal.value = true
}

function closeResetModal() {
  showResetModal.value = false
}

function confirmResetReviewed() {
  uiStore.resetStats()
  reviewedStore.resetReviewed()
  uiStore.toast('Review history cleared', 'info', 1500)
  closeResetModal()
}
</script>

<template>
  <header class="flex items-center justify-between px-4 py-3 max-w-4xl mx-auto">
    <div class="flex items-center gap-3">
      <!-- Title -->
      <h1 class="text-xl font-bold sm:inline hidden"
          :class="uiStore.isDarkMode ? 'text-white' : 'text-gray-900'">
        Immich Swipe
      </h1>
      <!-- Person switcher -->
      <div class="relative">
        <button
          @click="personMenuOpen = !personMenuOpen"
          class="flex items-center gap-1.5 px-2 py-0.5 text-xs rounded-full transition-colors"
          :class="uiStore.isDarkMode ? 'bg-indigo-900 text-indigo-200' : 'bg-indigo-100 text-indigo-700'"
          :aria-expanded="personMenuOpen"
          aria-haspopup="menu"
          aria-label="Switch person"
          title="Switch person"
        >
          <span v-if="authStore.currentUserName">{{ authStore.currentUserName }}</span>
          <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </button>

        <!-- Click-outside catcher -->
        <div
          v-if="personMenuOpen"
          class="fixed inset-0 z-40"
          @click="personMenuOpen = false"
        ></div>

        <!-- Person menu -->
        <div
          v-if="personMenuOpen"
          class="absolute left-0 top-full mt-2 z-50 w-64 rounded-2xl shadow-2xl border p-2 text-left"
          :class="uiStore.isDarkMode ? 'bg-gray-900 border-gray-800' : 'bg-white border-gray-200'"
          role="menu"
        >
          <p
            class="px-3 pt-1 pb-2 text-xs font-semibold uppercase tracking-wide"
            :class="uiStore.isDarkMode ? 'text-gray-400' : 'text-gray-500'"
          >
            Logged in
          </p>
          <div
            v-for="session in authStore.sessions"
            :key="session.key"
            class="flex items-center gap-2 rounded-xl px-3 py-2 text-sm"
            :class="session.key === authStore.activeSessionKey
              ? (uiStore.isDarkMode ? 'bg-indigo-900/60 text-indigo-200' : 'bg-indigo-100 text-indigo-800')
              : (uiStore.isDarkMode ? 'text-gray-300 hover:bg-gray-800' : 'text-gray-700 hover:bg-gray-100')"
          >
            <button
              type="button"
              role="menuitem"
              class="flex-1 min-w-0 text-left truncate"
              @click="selectPerson(session.key)"
            >
              {{ session.userName }}
            </button>
            <span
              v-if="session.key === authStore.activeSessionKey"
              class="text-xs opacity-70"
            >
              active
            </span>
            <button
              type="button"
              class="text-xs font-medium opacity-70 hover:opacity-100"
              :title="`Sign out ${session.userName}`"
              :aria-label="`Sign out ${session.userName}`"
              @click="signOutPerson(session.key)"
            >
              Sign out
            </button>
          </div>
          <hr class="my-1 border-gray-200 dark:border-gray-800" />
          <button
            type="button"
            role="menuitem"
            class="flex w-full items-center gap-2 rounded-xl px-3 py-2 text-sm font-medium"
            :class="uiStore.isDarkMode ? 'text-gray-200 hover:bg-gray-800' : 'text-gray-800 hover:bg-gray-100'"
            @click="addPerson"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
            </svg>
            Add person
          </button>
        </div>
      </div>
      <!-- Theme toggle -->
      <button
        @click="uiStore.toggleDarkMode()"
        class="p-2 rounded-full transition-colors"
        :class="uiStore.isDarkMode ? 'hover:bg-gray-800 text-white' : 'hover:bg-gray-200 text-gray-700'"
        aria-label="Toggle theme">
          <!-- Sun (dark mode) -->
          <svg v-if="uiStore.isDarkMode" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
          </svg>
          <!-- Moon (!dark mode) -->
          <svg v-else class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
          </svg>
      </button>

      <!-- Stats -->
      <button
        type="button"
        class="flex items-center gap-2 text-sm px-3 py-1 rounded-full border transition-colors"
        :class="uiStore.isDarkMode
          ? 'border-gray-700 text-gray-300 hover:bg-gray-800'
          : 'border-gray-200 text-gray-600 hover:bg-gray-100'"
        aria-label="Reset reviewed items"
        title="Reset reviewed items"
        @click="openResetModal"
      >
        <span class="flex items-center gap-1">
          <svg class="w-4 h-4 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
          </svg>
          {{ uiStore.keptCount }}
        </span>
        <span class="flex items-center gap-1">
          <svg class="w-4 h-4 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
          {{ uiStore.deletedCount }}
        </span>
      </button>

      <!-- Scope button -->
      <button
        @click="emit('openScopePicker')"
        class="flex items-center gap-2 px-3 py-1 rounded-full text-xs font-medium border transition-colors"
        :class="hasActiveScope
          ? 'bg-purple-600 border-purple-500 text-white'
          : uiStore.isDarkMode
            ? 'border-gray-700 text-gray-300 hover:bg-gray-800'
            : 'border-gray-300 text-gray-600 hover:bg-gray-100'"
        :aria-pressed="hasActiveScope"
        aria-label="Review scope"
        title="Review scope"
      >
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
        </svg>
        <span>Scope</span>
      </button>

      <!-- Active scope badge -->
      <span
        v-if="hasActiveScope"
        class="px-2 py-0.5 text-xs rounded-full font-medium"
        :class="uiStore.isDarkMode ? 'bg-purple-900 text-purple-200' : 'bg-purple-100 text-purple-700'"
      >
        {{ scopeLabel }}
      </span>

      <!-- Person button -->
      <button
        @click="emit('openPersonPicker')"
        class="flex items-center gap-2 px-3 py-1 rounded-full text-xs font-medium border transition-colors"
        :class="hasActivePerson
          ? 'bg-cyan-600 border-cyan-500 text-white'
          : uiStore.isDarkMode
            ? 'border-gray-700 text-gray-300 hover:bg-gray-800'
            : 'border-gray-300 text-gray-600 hover:bg-gray-100'"
        :aria-pressed="hasActivePerson"
        aria-label="Review person"
        title="Review person"
      >
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
        </svg>
        <span>Person</span>
      </button>

      <!-- Active person badge -->
      <span
        v-if="hasActivePerson"
        class="px-2 py-0.5 text-xs rounded-full font-medium"
        :class="uiStore.isDarkMode ? 'bg-cyan-900 text-cyan-200' : 'bg-cyan-100 text-cyan-700'"
      >
        {{ personLabel }}
      </span>

      <!-- Skip videos toggle -->
      <button
        @click="uiStore.toggleSkipVideos()"
        class="flex items-center gap-2 px-3 py-1 rounded-full text-xs font-medium border transition-colors"
        :class="uiStore.skipVideos
          ? 'bg-green-600 border-green-500 text-white'
          : uiStore.isDarkMode
            ? 'border-gray-700 text-gray-300 hover:bg-gray-800'
            : 'border-gray-300 text-gray-600 hover:bg-gray-100'"
        :aria-pressed="uiStore.skipVideos"
      >
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14m-3 4h7a2 2 0 002-2V8a2 2 0 00-2-2h-7M9 18H6a2 2 0 01-2-2V8a2 2 0 012-2h3m0 12V6"
          />
        </svg>
        <span>
          Skip videos
        </span>
      </button>

      <!-- Review order toggle -->
      <button
        @click="toggleReviewOrder"
        class="flex items-center gap-2 px-3 py-1 rounded-full text-xs font-medium border transition-colors"
        :class="preferencesStore.reviewOrder !== 'random'
          ? 'bg-blue-600 border-blue-500 text-white'
          : uiStore.isDarkMode
            ? 'border-gray-700 text-gray-300 hover:bg-gray-800'
            : 'border-gray-300 text-gray-600 hover:bg-gray-100'"
        :aria-pressed="preferencesStore.reviewOrder !== 'random'"
        :aria-label="preferencesStore.reviewOrder === 'chronological'
          ? 'Order: Oldest first'
          : preferencesStore.reviewOrder === 'chronological-desc'
            ? 'Order: Newest first'
            : 'Order: Random'"
        :title="preferencesStore.reviewOrder === 'chronological'
          ? 'Order: Oldest first'
          : preferencesStore.reviewOrder === 'chronological-desc'
            ? 'Order: Newest first'
            : 'Order: Random'"
      >
        <span>Order:</span>
        <svg
          v-if="preferencesStore.reviewOrder === 'random'"
          class="w-4 h-4"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 3h5v5" />
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 20L21 3" />
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 16v5h-5" />
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 15l6 6" />
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4l5 5" />
        </svg>
        <svg
          v-else-if="preferencesStore.reviewOrder === 'chronological'"
          class="w-4 h-4"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h10M4 12h7M4 16h4M18 18V6m0 0-3 3m3-3 3 3" />
        </svg>
        <svg
          v-else
          class="w-4 h-4"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h4M4 12h7M4 16h10M18 6v12m0 0-3-3m3 3 3-3" />
        </svg>
      </button>
      <!-- Settings -->
      <button
        @click="router.push('/settings')"
        class="p-2 rounded-full transition-colors"
        :class="uiStore.isDarkMode ? 'hover:bg-gray-800 text-white' : 'hover:bg-gray-200 text-gray-700'"
        aria-label="Settings"
        title="Settings"
      >
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
      </button>
    </div>
  </header>

  <!-- Review progress -->
  <div v-if="showProgress" class="px-4 py-1.5 max-w-4xl mx-auto w-full">
    <div class="flex items-center gap-2">
      <div class="flex-1 h-1.5 rounded-full bg-gray-200 dark:bg-gray-800 overflow-hidden">
        <div
          class="h-full rounded-full transition-all duration-300"
          :class="libraryReviewed ? 'bg-green-500' : 'bg-blue-500'"
          :style="{ width: progressPercent + '%' }"
        ></div>
      </div>
      <span
        class="text-xs tabular-nums whitespace-nowrap"
        :class="uiStore.isDarkMode ? 'text-gray-400' : 'text-gray-600'"
      >
        {{ reviewedStore.reviewedCount }} / {{ total }}
      </span>
      <span v-if="libraryReviewed" class="text-xs font-semibold text-green-500 whitespace-nowrap">
        Library reviewed ✓
      </span>
    </div>
  </div>

  <div
    v-if="showResetModal"
    class="fixed inset-0 z-40 flex items-center justify-center bg-black/60 px-4"
    @click="closeResetModal"
  >
    <div
      class="w-full max-w-md rounded-2xl shadow-2xl border p-5 text-left"
      :class="uiStore.isDarkMode ? 'bg-gray-900 border-gray-800' : 'bg-white border-gray-200'"
      @click.stop
    >
      <h2
        class="text-lg font-semibold"
        :class="uiStore.isDarkMode ? 'text-gray-100' : 'text-gray-900'"
      >
        Reset reviewed history?
      </h2>
      <p
        class="mt-2 text-sm"
        :class="uiStore.isDarkMode ? 'text-gray-400' : 'text-gray-600'"
      >
        This clears the counters and removes all already visited image and video IDs.
      </p>
      <div
        class="mt-4 flex items-center justify-between rounded-lg px-3 py-2 text-sm"
        :class="uiStore.isDarkMode ? 'bg-gray-800 text-gray-300' : 'bg-gray-50 text-gray-700'"
      >
        <span>Kept: {{ uiStore.keptCount }}</span>
        <span>Deleted: {{ uiStore.deletedCount }}</span>
      </div>
      <div class="mt-5 flex items-center justify-end gap-2">
        <button
          type="button"
          class="px-4 py-2 rounded-full text-sm font-medium border transition-colors"
          :class="uiStore.isDarkMode
            ? 'border-gray-700 text-gray-200 hover:bg-gray-800'
            : 'border-gray-300 text-gray-700 hover:bg-gray-100'"
          @click="closeResetModal"
        >
          Cancel
        </button>
        <button
          type="button"
          class="px-4 py-2 rounded-full text-sm font-semibold text-white bg-red-600 hover:bg-red-700 transition-colors"
          @click="confirmResetReviewed"
        >
          Reset
        </button>
      </div>
    </div>
  </div>
</template>
