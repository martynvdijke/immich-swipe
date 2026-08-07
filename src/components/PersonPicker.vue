<script setup lang="ts">
import { computed, ref } from 'vue'
import type { ImmichPerson } from '@/types/immich'

const props = defineProps<{
  open: boolean
  people: ImmichPerson[]
  loading: boolean
  error: string | null
  thumbnailUrlFn: (personId: string) => string
}>()

const emit = defineEmits<{
  close: []
  select: [personId: string]
  clear: []
}>()

const touchStart = ref<{ x: number; y: number } | null>(null)
const touchDelta = ref({ x: 0, y: 0 })

// Non-hidden people first, hidden ones sorted last.
const sortedPeople = computed(() => {
  return [...props.people].sort((a, b) => {
    if (a.isHidden !== b.isHidden) {
      return a.isHidden ? 1 : -1
    }
    return a.name.localeCompare(b.name)
  })
})

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
              <p class="text-sm text-gray-500 dark:text-gray-400">Review person</p>
              <h2 class="text-xl font-semibold text-gray-900 dark:text-gray-50">People</h2>
            </div>
          </div>

          <div class="px-4 py-3 safe-area-bottom flex flex-col gap-5 flex-1 min-h-0 overflow-y-auto">
            <!-- Clear person filter -->
            <div>
              <p class="text-sm font-semibold text-gray-800 dark:text-gray-100 mb-2">Filter</p>
              <button
                type="button"
                class="w-full text-left px-3 py-2 rounded-lg border border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-800/50 hover:border-cyan-400 transition-colors"
                @click="emit('clear')"
              >
                <span class="font-medium text-gray-900 dark:text-gray-100">Everyone</span>
                <span class="block text-xs text-gray-500 dark:text-gray-400">
                  Review photos of all people — the default feed
                </span>
              </button>
            </div>

            <!-- Person list -->
            <div>
              <p class="text-sm font-semibold text-gray-800 dark:text-gray-100 mb-2">People</p>
              <div v-if="loading" class="flex items-center justify-center py-6">
                <div class="w-8 h-8 border-4 border-cyan-500 border-t-transparent rounded-full animate-spin"></div>
              </div>
              <div v-else-if="error" class="p-4 rounded-lg bg-red-50 text-red-700 border border-red-200">
                {{ error }}
              </div>
              <div v-else-if="sortedPeople.length === 0" class="p-4 rounded-lg bg-gray-50 dark:bg-gray-800/50 text-sm text-gray-500 dark:text-gray-400">
                No people found. Face recognition may be disabled on this Immich server — the normal review feed is unaffected.
              </div>
              <div v-else class="grid sm:grid-cols-2 gap-2">
                <button
                  v-for="person in sortedPeople"
                  :key="person.id"
                  type="button"
                  class="text-left px-3 py-2 rounded-lg border border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-800/50 flex items-center gap-3 hover:border-cyan-400 transition-colors"
                  @click="emit('select', person.id)"
                >
                  <img
                    :src="thumbnailUrlFn(person.id)"
                    alt=""
                    class="w-9 h-9 rounded-full object-cover bg-gray-200 dark:bg-gray-700"
                    loading="lazy"
                    @error="($event.target as HTMLImageElement).style.visibility = 'hidden'"
                  />
                  <span class="flex items-center gap-2 min-w-0">
                    <span class="block font-medium text-gray-900 dark:text-gray-100 truncate">{{ person.name }}</span>
                    <span v-if="person.isHidden" class="shrink-0 px-2 py-0.5 rounded-full text-[10px] font-medium bg-gray-300 dark:bg-gray-700 text-gray-700 dark:text-gray-300">
                      hidden
                    </span>
                  </span>
                  <span class="ml-auto shrink-0 px-3 py-1 rounded-full text-sm font-medium bg-cyan-600 text-white">
                    Select
                  </span>
                </button>
              </div>
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
