<script setup lang="ts">
import { ref, watch } from 'vue'
import type { ImmichAlbum, ReviewScope } from '@/types/immich'

const props = defineProps<{
  open: boolean
  albums: ImmichAlbum[]
  loading: boolean
  error: string | null
}>()

const emit = defineEmits<{
  close: []
  apply: [ReviewScope]
  clear: []
}>()

const from = ref('')
const to = ref('')
const touchStart = ref<{ x: number; y: number } | null>(null)
const touchDelta = ref({ x: 0, y: 0 })

function applyDateRange() {
  if (!from.value || !to.value) return
  emit('apply', { kind: 'dateRange', from: from.value, to: to.value })
}

function handleTouchStart(event: TouchEvent) {
  if (event.touches.length !== 1) return
  const touch = event.touches[0]
  touchStart.value = { x: touch.clientX, y: touch.clientY }
  touchDelta.value = { x: 0, y: 0 }
}

function handleTouchMove(event: TouchEvent) {
  if (!touchStart.value || event.touches.length !== 1) return
  const touch = event.touches[0]
  touchDelta.value = {
    x: touch.clientX - touchStart.value.x,
    y: touch.clientY - touchStart.value.y,
  }
}

function handleTouchEnd() {
  if (!touchStart.value) return
  const { x, y } = touchDelta.value
  const isSwipeDown = y > 80 && Math.abs(y) > Math.abs(x)
  const isSwipeRight = x > 80 && Math.abs(x) > Math.abs(y)
  if (isSwipeDown || isSwipeRight) {
    emit('close')
  }
  touchStart.value = null
  touchDelta.value = { x: 0, y: 0 }
}

watch(
  () => props.open,
  (isOpen) => {
    if (!isOpen) {
      from.value = ''
      to.value = ''
    }
  }
)
</script>

<template>
  <transition name="fade">
    <div
      v-if="open"
      class="fixed inset-0 z-30 flex items-end sm:items-center justify-center bg-black/60 px-4"
      style="padding-bottom: env(safe-area-inset-bottom);"
      @click="emit('close')"
    >
      <transition name="sheet">
        <div
          v-if="open"
          class="w-full max-w-3xl bg-white dark:bg-gray-900 rounded-t-2xl sm:rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[95vh] max-h-[95dvh] min-h-[85vh] min-h-[85dvh] sm:max-h-none sm:min-h-0"
          @click.stop
        >
          <div
            class="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-800"
            @touchstart="handleTouchStart"
            @touchmove="handleTouchMove"
            @touchend="handleTouchEnd"
          >
            <div class="flex flex-col leading-tight">
              <p class="text-sm text-gray-500 dark:text-gray-400">Review scope</p>
              <h2 class="text-xl font-semibold text-gray-900 dark:text-gray-50">Constrain the feed</h2>
            </div>
          </div>

          <div class="px-4 py-3 safe-area-bottom flex flex-col gap-5 flex-1 min-h-0 overflow-y-auto">
            <!-- Library (clear scope) -->
            <div>
              <p class="text-sm font-semibold text-gray-800 dark:text-gray-100 mb-2">Scope</p>
              <button
                type="button"
                class="w-full text-left px-3 py-2 rounded-lg border border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-800/50 hover:border-blue-400 transition-colors"
                @click="emit('clear')"
              >
                <span class="font-medium text-gray-900 dark:text-gray-100">Entire library</span>
                <span class="block text-xs text-gray-500 dark:text-gray-400">
                  No filtering — the default feed
                </span>
              </button>
            </div>

            <!-- Albums -->
            <div>
              <p class="text-sm font-semibold text-gray-800 dark:text-gray-100 mb-2">Album</p>
              <div v-if="loading" class="flex items-center justify-center py-6">
                <div class="w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full animate-spin"></div>
              </div>
              <div v-else-if="error" class="p-4 rounded-lg bg-red-50 text-red-700 border border-red-200">
                {{ error }}
              </div>
              <div v-else class="grid sm:grid-cols-2 gap-2">
                <button
                  v-for="album in albums"
                  :key="album.id"
                  type="button"
                  class="text-left px-3 py-2 rounded-lg border border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-800/50 flex items-center justify-between gap-3 hover:border-blue-400 transition-colors"
                  @click="emit('apply', { kind: 'album', albumId: album.id })"
                >
                  <span class="min-w-0">
                    <span class="block font-medium text-gray-900 dark:text-gray-100 truncate">{{ album.albumName }}</span>
                    <span v-if="album.assetCount !== undefined" class="block text-xs text-gray-500 dark:text-gray-400">
                      {{ album.assetCount }} items
                    </span>
                  </span>
                  <span class="shrink-0 px-3 py-1 rounded-full text-sm font-medium bg-blue-600 text-white">
                    Select
                  </span>
                </button>
                <p v-if="albums.length === 0" class="col-span-2 text-sm text-gray-500 dark:text-gray-400 py-2 text-center">
                  No albums found.
                </p>
              </div>
            </div>

            <!-- Date range -->
            <div>
              <p class="text-sm font-semibold text-gray-800 dark:text-gray-100 mb-2">Date range</p>
              <div class="flex flex-col sm:flex-row gap-3">
                <label class="flex-1 flex flex-col gap-1 text-xs text-gray-500 dark:text-gray-400">
                  From
                  <input
                    v-model="from"
                    type="date"
                    class="px-3 py-2 rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  />
                </label>
                <label class="flex-1 flex flex-col gap-1 text-xs text-gray-500 dark:text-gray-400">
                  To
                  <input
                    v-model="to"
                    type="date"
                    class="px-3 py-2 rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  />
                </label>
                <button
                  type="button"
                  class="self-end px-4 py-2 rounded-lg text-sm font-semibold bg-blue-600 text-white hover:bg-blue-700 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                  :disabled="!from || !to"
                  @click="applyDateRange"
                >
                  Apply
                </button>
              </div>
            </div>

            <!-- Favorites -->
            <div>
              <p class="text-sm font-semibold text-gray-800 dark:text-gray-100 mb-2">Favorites</p>
              <button
                type="button"
                class="w-full text-left px-3 py-2 rounded-lg border border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-800/50 hover:border-blue-400 transition-colors"
                @click="emit('apply', { kind: 'favorites' })"
              >
                <span class="font-medium text-gray-900 dark:text-gray-100">Favorites only</span>
                <span class="block text-xs text-gray-500 dark:text-gray-400">
                  Review only assets you have favorited
                </span>
              </button>
            </div>

            <div class="pt-2">
              <button
                type="button"
                class="w-full h-10 rounded-lg text-sm font-semibold transition-colors bg-gray-100 dark:bg-gray-800 text-gray-800 dark:text-gray-200 hover:bg-gray-200 dark:hover:bg-gray-700"
                @click="emit('close')"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      </transition>
    </div>
  </transition>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.sheet-enter-active,
.sheet-leave-active {
  transition: transform 0.3s ease, opacity 0.2s ease;
}
.sheet-enter-from,
.sheet-leave-to {
  transform: translateY(100%);
  opacity: 0;
}
</style>
