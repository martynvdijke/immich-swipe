<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useImmich } from '@/composables/useImmich'
import { useUiStore } from '@/stores/ui'
import { useAuthStore } from '@/stores/auth'
import { usePreferencesStore } from '@/stores/preferences'
import { trackEvent } from '@/composables/useUmami'
import { recordPersonFilter } from '@/composables/useOtel'
import type { ImmichAlbum, ImmichPerson, ReviewScope } from '@/types/immich'
import AppHeader from '@/components/AppHeader.vue'
import SwipeCard from '@/components/SwipeCard.vue'
import ActionButtons from '@/components/ActionButtons.vue'
import AlbumPicker from '@/components/AlbumPicker.vue'
import ReviewScopePicker from '@/components/ReviewScopePicker.vue'
import PersonPicker from '@/components/PersonPicker.vue'
import AssetDetailOverlay from '@/components/AssetDetailOverlay.vue'

const {
  currentAsset,
  error,
  loadInitialAsset,
  keepPhoto,
  keepPhotoToAlbum,
  toggleFavorite,
  deletePhoto,
  skipPhoto,
  undoLastAction,
  fetchAlbums,
  fetchPeople,
  getPersonThumbnailUrl,
  reviewTotal,
  canUndo,
} = useImmich()
const uiStore = useUiStore()
const authStore = useAuthStore()
const preferencesStore = usePreferencesStore()

const showAlbumPicker = ref(false)
const isLoadingAlbums = ref(false)
const albumsError = ref<string | null>(null)
const albums = ref<ImmichAlbum[]>([])

const showScopePicker = ref(false)

const showPersonPicker = ref(false)
const isLoadingPeople = ref(false)
const peopleError = ref<string | null>(null)
const people = ref<ImmichPerson[]>([])

// Full-resolution detail overlay (tap on card / Z key). While open, review
// actions and hotkeys are suppressed.
const isDetailOpen = ref(false)
const swipeCardRef = ref<InstanceType<typeof SwipeCard> | null>(null)
const currentVideoSrc = computed(() => swipeCardRef.value?.videoBlobUrl ?? null)

const scopeLabel = computed(() => {
  const scope = preferencesStore.scope
  if (scope.kind === 'library') return ''
  if (scope.kind === 'album') {
    return albums.value.find((album) => album.id === scope.albumId)?.albumName || 'Album'
  }
  if (scope.kind === 'favorites') return 'Favorites'
  if (scope.kind === 'duplicates') return 'Duplicates'
  if (scope.kind === 'location') {
    return [scope.country, scope.city, scope.state].filter(Boolean).join(' · ') || 'Location'
  }
  if (scope.kind === 'camera') {
    return [scope.make, scope.model].filter(Boolean).join(' · ') || 'Camera'
  }
  return 'Date range'
})

const personLabel = computed(() => {
  const id = preferencesStore.selectedPersonId
  if (!id) return ''
  return people.value.find((person) => person.id === id)?.name || ''
})

const selectedPersonName = computed(() => {
  const id = preferencesStore.selectedPersonId
  if (!id) return ''
  return people.value.find((person) => person.id === id)?.name || ''
})

// Keyboard navigation
function handleKeydown(e: KeyboardEvent) {
  // Escape dismisses the topmost overlay and never triggers review actions.
  if (e.key === 'Escape') {
    if (isDetailOpen.value) {
      e.preventDefault()
      isDetailOpen.value = false
    } else if (showAlbumPicker.value) {
      e.preventDefault()
      showAlbumPicker.value = false
    } else if (showScopePicker.value) {
      e.preventDefault()
      showScopePicker.value = false
    } else if (showPersonPicker.value) {
      e.preventDefault()
      showPersonPicker.value = false
    }
    return
  }

  // Uniform suppression: typing in inputs or any open overlay/picker blocks
  // every shortcut below (including undo and arrow-key decisions).
  if (shouldIgnoreHotkeys()) return
  if (!currentAsset.value) return

  if (e.key === 'ArrowUp') {
    e.preventDefault()
    undoLastAction()
    return
  }
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'z') {
    e.preventDefault()
    undoLastAction()
    return
  }

  if (e.key === 'ArrowRight') {
    e.preventDefault()
    keepPhoto()
  } else if (e.key === 'ArrowLeft') {
    e.preventDefault()
    deletePhoto()
  } else if (e.key.toLowerCase() === 'f') {
    e.preventDefault()
    toggleFavorite()
  } else if (e.key.toLowerCase() === 's') {
    e.preventDefault()
    skipPhoto()
  } else if (e.key.toLowerCase() === 'a') {
    e.preventDefault()
    void openAlbumPicker()
  } else if (e.key.toLowerCase() === 'z' && !e.ctrlKey && !e.metaKey) {
    e.preventDefault()
    isDetailOpen.value = !isDetailOpen.value
  } else if (/^[0-9]$/.test(e.key)) {
    const albumId = preferencesStore.albumHotkeys[e.key]
    if (!albumId) {
      uiStore.toast(`No album configured for key ${e.key}`, 'info', 2000)
      return
    }
    const album = albums.value.find((item) => item.id === albumId)
    keepPhotoToAlbum(album || { id: albumId, albumName: `Album ${e.key}` })
  }
}

function shouldIgnoreHotkeys(): boolean {
  const active = document.activeElement as HTMLElement | null
  const isTyping = active && ['INPUT', 'TEXTAREA'].includes(active.tagName)
  return (
    !!isTyping ||
    showAlbumPicker.value ||
    showScopePicker.value ||
    showPersonPicker.value ||
    isDetailOpen.value
  )
}

async function openScopePicker() {
  await ensureAlbumsLoaded()
  showScopePicker.value = true
}

function closeScopePicker() {
  showScopePicker.value = false
}

function handleScopeApply(scope: ReviewScope) {
  preferencesStore.setScope(scope)
  showScopePicker.value = false
}

function handleScopeClear() {
  preferencesStore.setScope({ kind: 'library' })
  showScopePicker.value = false
}

async function ensurePeopleLoaded() {
  if (people.value.length > 0) return
  try {
    isLoadingPeople.value = true
    peopleError.value = null
    people.value = await fetchPeople()
  } catch (e) {
    console.error(e)
    peopleError.value = e instanceof Error ? e.message : 'Failed to load people'
  } finally {
    isLoadingPeople.value = false
  }
}

async function openPersonPicker() {
  await ensurePeopleLoaded()
  showPersonPicker.value = true
}

function closePersonPicker() {
  showPersonPicker.value = false
}

function handlePersonSelect(personId: string) {
  const person = people.value.find((p) => p.id === personId)
  const personName = person?.name || personId
  preferencesStore.setSelectedPerson(personId)
  trackEvent('swipe.person_filter', { personName })
  recordPersonFilter(personName)
  showPersonPicker.value = false
}

function handlePersonClear() {
  const previousName = personLabel.value
  preferencesStore.setSelectedPerson(null)
  trackEvent('swipe.person_filter', { personName: previousName || 'none' })
  recordPersonFilter(previousName || 'none')
  showPersonPicker.value = false
}

async function ensureAlbumsLoaded() {
  if (albums.value.length > 0) return
  try {
    isLoadingAlbums.value = true
    albumsError.value = null
    albums.value = await fetchAlbums()
  } catch (e) {
    console.error(e)
    albumsError.value = e instanceof Error ? e.message : 'Failed to load albums'
  } finally {
    isLoadingAlbums.value = false
  }
}

async function openAlbumPicker() {
  await ensureAlbumsLoaded()
  showAlbumPicker.value = true
}

function closeAlbumPicker() {
  showAlbumPicker.value = false
}

async function handleAlbumSelected(album: ImmichAlbum) {
  await keepPhotoToAlbum(album)
  showAlbumPicker.value = false
}

function handleAssignHotkey(key: string, albumId: string | null) {
  if (albumId) {
    preferencesStore.setHotkey(key, albumId)
  } else {
    preferencesStore.clearHotkey(key)
  }
}

watch(
  () => preferencesStore.reviewOrder,
  async () => {
    await loadInitialAsset()
  }
)

// Scope changes reset the flow and reload from the scoped feed.
watch(
  () => preferencesStore.scope,
  async () => {
    await loadInitialAsset()
  }
)

// Person selection resets the flow and reloads from the person-scoped feed.
watch(
  () => preferencesStore.selectedPersonId,
  async () => {
    await loadInitialAsset()
  }
)

// Account switch (multi-person sessions): reload the feed for the newly
// active person. Per-user stores reload via their own identity watchers.
watch(
  () => [authStore.immichServerUrl, authStore.currentUserName],
  async () => {
    await loadInitialAsset()
  }
)

// Skip-videos changes the feed composition (videos included or not): reload
// so the feed and the review-progress total reflect the new filter.
watch(
  () => uiStore.skipVideos,
  async () => {
    await loadInitialAsset()
  }
)

onMounted(() => {
  loadInitialAsset()
  ensurePeopleLoaded()
  window.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <div class="viewport-fit flex flex-col"
    :class="uiStore.isDarkMode ? 'bg-black text-white' : 'bg-white text-black'"
  >
    <AppHeader
      :scope-label="scopeLabel"
      :person-label="personLabel"
      :review-total="reviewTotal"
      @open-scope-picker="openScopePicker"
      @open-person-picker="openPersonPicker"
    />

    <!-- Main content -->
    <main class="flex-1 flex flex-col px-4 safe-area-bottom min-h-0 gap-3 overflow-hidden">
      <!-- Error state -->
      <div v-if="error && !currentAsset" class="flex-1 flex flex-col items-center justify-center gap-4">
        <div class="text-center">
          <svg class="w-16 h-16 mx-auto mb-4"
            :class="uiStore.isDarkMode ? 'text-gray-600' : 'text-gray-400'"
            fill="none" stroke="currentColor" viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <p class="text-lg"
            :class="uiStore.isDarkMode ? 'text-gray-400' : 'text-gray-600'"
          >
            {{ error }}
          </p>
        </div>
        <button
          @click="() => loadInitialAsset()"
          class="px-6 py-2 rounded-lg transition-colors"
          :class="uiStore.isDarkMode
            ? 'bg-white text-black hover:bg-gray-200'
            : 'bg-black text-white hover:bg-gray-800'"
        >
          Try Again
        </button>
      </div>

      <!-- Swipe area -->
      <div v-else class="flex-1 flex flex-col min-h-0 gap-2">
        <!-- Card container -->
        <div class="flex-1 min-h-0 flex items-center justify-center p-1">
          <div v-if="currentAsset" class="w-full h-full max-w-4xl max-h-full">
            <SwipeCard
              ref="swipeCardRef"
              :asset="currentAsset"
              :person-name="selectedPersonName"
              @keep="keepPhoto"
              @delete="deletePhoto"
              @inspect="isDetailOpen = true"
            />
          </div>

          <!-- Empty state while loading -->
          <div v-else class="flex h-full items-center justify-center"
            :class="uiStore.isDarkMode ? 'text-gray-600' : 'text-gray-400'"
          >
            <svg class="w-16 h-16 animate-pulse" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
            </svg>
          </div>
        </div>

        <div class="shrink-0 flex flex-col items-center gap-2">
          <!-- Action buttons -->
          <ActionButtons
            v-if="currentAsset"
            class="-mx-4 sm:mx-0"
            :can-undo="canUndo"
            :is-favorite="currentAsset?.isFavorite ?? false"
            @keep="keepPhoto"
            @delete="deletePhoto"
            @skip="skipPhoto"
            @undo="undoLastAction"
            @toggle-favorite="toggleFavorite"
            @open-album-picker="openAlbumPicker"
            @album-drop="openAlbumPicker"
          />

          <!-- Instructions (now mobile -> hidden) -->
          <div class="hidden sm:flex text-center text-sm py-2 items-center flex-col gap-y-2"
            :class="uiStore.isDarkMode ? 'text-gray-500' : 'text-gray-400'"
          >
            <!-- Mobile -->
            <div class="sm:hidden flex gap-x-2">
              <span class="px-3 py-1 rounded-full text-xs font-medium border transition-colors border-green-700 text-green-600 bg-green-300">
                RIGHT 
                
              </span>
              <div
                class="w-px h-6"
                :class="uiStore.isDarkMode ? 'bg-gray-700' : 'bg-gray-300'"
              ></div>
              <span class="px-3 py-1 rounded-full text-xs font-medium border transition-colors border-red-700 text-red-600 bg-red-300">
                LEFT
              </span>
            </div>
            <!-- Desktop -->
            <div class="hidden sm:flex gap-x-3">
              <span class="flex gap-x-2 px-3 py-1 rounded-full text-xs font-medium border transition-colors border-red-700 text-red-600 bg-red-300">
                LEFT
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path></svg>
              </span>
              <div
                class="w-px h-6"
                :class="uiStore.isDarkMode ? 'bg-gray-700' : 'bg-gray-300'"
              ></div>
              <span class="flex gap-x-2 px-3 py-1 rounded-full text-xs font-medium border transition-colors border-green-700 bg-green-300 text-green-600">
                RIGHT 
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path></svg>
              </span>
            </div>
            <p class="hidden sm:flex">
              (←/→) • Ctrl+Z or ↑ (back) • F = favorite • S = skip • A = album • Z = zoom • 0–9 = album hotkeys
            </p>
          </div>
        </div>
      </div>
    </main>

    <AlbumPicker
      :open="showAlbumPicker"
      :albums="albums"
      :loading="isLoadingAlbums"
      :error="albumsError"
      :hotkeys="preferencesStore.albumHotkeys"
      @close="closeAlbumPicker"
      @select="handleAlbumSelected"
      @assign-hotkey="handleAssignHotkey"
    />

    <ReviewScopePicker
      :open="showScopePicker"
      :albums="albums"
      :loading="isLoadingAlbums"
      :error="albumsError"
      @close="closeScopePicker"
      @apply="handleScopeApply"
      @clear="handleScopeClear"
    />

    <PersonPicker
      :open="showPersonPicker"
      :people="people"
      :loading="isLoadingPeople"
      :error="peopleError"
      :thumbnail-url-fn="getPersonThumbnailUrl"
      @close="closePersonPicker"
      @select="handlePersonSelect"
      @clear="handlePersonClear"
    />

    <AssetDetailOverlay
      :open="isDetailOpen && !!currentAsset"
      :asset="currentAsset"
      :video-src="currentVideoSrc"
      @close="isDetailOpen = false"
    />
  </div>
</template>
