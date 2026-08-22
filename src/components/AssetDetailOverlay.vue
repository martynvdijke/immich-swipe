<script setup lang="ts">
import { computed, ref, watch, onBeforeUnmount } from 'vue'
import { useAuthStore } from '@/stores/auth'
import type { ImmichAsset } from '@/types/immich'

const props = defineProps<{
  open: boolean
  asset: ImmichAsset | null
  /** Preloaded original blob URL for videos (from the swipe card), if available. */
  videoSrc?: string | null
}>()

const emit = defineEmits<{
  close: []
}>()

const authStore = useAuthStore()

const isVideo = computed(() => props.asset?.type === 'VIDEO')

// Media state: original blob for images (fallback preview), video reuses the
// card's preloaded stream when available.
const imageBlobUrl = ref<string | null>(null)
const usingPreviewFallback = ref(false)
const loading = ref(false)
let abortController: AbortController | null = null

function buildAssetUrl(path: string): string {
  const id = props.asset?.id
  return `/api/assets/${id}/${path}`
}

function revokeImage() {
  if (imageBlobUrl.value) {
    URL.revokeObjectURL(imageBlobUrl.value)
    imageBlobUrl.value = null
  }
}

function stopFetch() {
  abortController?.abort()
  abortController = null
}

async function fetchBlob(url: string, signal: AbortSignal): Promise<Blob> {
  const response = await fetch(url, { headers: authStore.authHeader, signal })
  if (!response.ok) throw new Error(`HTTP ${response.status}`)
  return response.blob()
}

async function loadMedia() {
  if (!props.open || !props.asset) return
  stopFetch()
  revokeImage()
  usingPreviewFallback.value = false

  if (isVideo.value) {
    // Video playback is handled in the template via videoSrc (or a fresh fetch below).
    if (!props.videoSrc) {
      loading.value = true
      const controller = new AbortController()
      abortController = controller
      try {
        const blob = await fetchBlob(buildAssetUrl('original'), controller.signal)
        imageBlobUrl.value = URL.createObjectURL(blob)
      } catch (e) {
        if (!controller.signal.aborted) console.error('Failed to load video original:', e)
      } finally {
        if (abortController === controller) abortController = null
        loading.value = false
      }
    }
    return
  }

  loading.value = true
  const controller = new AbortController()
  abortController = controller
  try {
    const blob = await fetchBlob(buildAssetUrl('original'), controller.signal)
    imageBlobUrl.value = URL.createObjectURL(blob)
  } catch (e) {
    if (controller.signal.aborted) return
    console.error('Failed to load original, falling back to preview:', e)
    // Graceful fallback: preview thumbnail instead of an error wall.
    try {
      const blob = await fetchBlob(buildAssetUrl('thumbnail?size=preview'), controller.signal)
      imageBlobUrl.value = URL.createObjectURL(blob)
      usingPreviewFallback.value = true
    } catch (e2) {
      console.error('Failed to load preview fallback:', e2)
    }
  } finally {
    if (abortController === controller) abortController = null
    loading.value = false
  }
}

watch(
  () => [props.open, props.asset?.id],
  () => {
    resetZoom()
    loadMedia()
  },
  { immediate: true }
)

onBeforeUnmount(() => {
  stopFetch()
  revokeImage()
})

// --- Info panel ---

const showInfo = ref(typeof window === 'undefined' ? true : window.innerWidth >= 640)

const formattedDateTime = computed(() => {
  const raw = props.asset?.exifInfo?.dateTimeOriginal || props.asset?.localDateTime || props.asset?.fileCreatedAt
  if (!raw) return ''
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
})

const cameraLabel = computed(() => {
  const make = props.asset?.exifInfo?.make?.trim()
  const model = props.asset?.exifInfo?.model?.trim()
  return [make, model].filter(Boolean).join(' · ')
})

const resolutionLabel = computed(() => {
  const w = props.asset?.exifInfo?.exifImageWidth
  const h = props.asset?.exifInfo?.exifImageHeight
  if (!w || !h) return ''
  return `${w} × ${h}`
})

const locationLabel = computed(() => {
  const city = props.asset?.exifInfo?.city?.trim()
  const country = props.asset?.exifInfo?.country?.trim()
  return [city, country].filter(Boolean).join(', ')
})

const hasInfo = computed(() =>
  Boolean(formattedDateTime.value || cameraLabel.value || resolutionLabel.value || locationLabel.value)
)

// --- Zoom & pan ---

const scale = ref(1)
const tx = ref(0)
const ty = ref(0)
const MIN_SCALE = 1
const MAX_SCALE = 8
const DBLCLICK_SCALE = 2.5

const transformStyle = computed(() => ({
  transform: `translate(${tx.value}px, ${ty.value}px) scale(${scale.value})`,
}))

function clampScale(value: number): number {
  return Math.min(MAX_SCALE, Math.max(MIN_SCALE, value))
}

function resetZoom() {
  scale.value = 1
  tx.value = 0
  ty.value = 0
}

/** Zoom by `factor` keeping the point (clientX, clientY) stationary. */
function zoomAt(factor: number, clientX: number, clientY: number) {
  const container = mediaRef.value
  if (!container) return
  const rect = container.getBoundingClientRect()
  // Pointer position relative to the container center (the transform origin).
  const px = clientX - (rect.left + rect.width / 2)
  const py = clientY - (rect.top + rect.height / 2)
  const prev = scale.value
  const next = clampScale(prev * factor)
  if (next === prev) return
  // Keep pointer anchored: adjust translation proportionally.
  tx.value = px - ((px - tx.value) * next) / prev
  ty.value = py - ((py - ty.value) * next) / prev
  scale.value = next
  if (next === MIN_SCALE) {
    tx.value = 0
    ty.value = 0
  }
}

function onWheel(e: WheelEvent) {
  e.preventDefault()
  zoomAt(e.deltaY < 0 ? 1.15 : 1 / 1.15, e.clientX, e.clientY)
}

function onDblClick(e: MouseEvent) {
  if (scale.value > MIN_SCALE) {
    resetZoom()
  } else {
    zoomAt(DBLCLICK_SCALE, e.clientX, e.clientY)
  }
}

// Pointer tracking for pinch (two pointers) and pan (drag while zoomed).
const activePointers = new Map<number, { x: number; y: number }>()
let pinchStartDistance: number | null = null
let pinchStartScale = 1
let panStart: { x: number; y: number; tx: number; ty: number } | null = null

const mediaRef = ref<HTMLElement | null>(null)

function onPointerDown(e: PointerEvent) {
  activePointers.set(e.pointerId, { x: e.clientX, y: e.clientY })
  if (activePointers.size === 2) {
    const [p1, p2] = Array.from(activePointers.values())
    pinchStartDistance = Math.hypot(p1.x - p2.x, p1.y - p2.y)
    pinchStartScale = scale.value
    panStart = null
  } else if (activePointers.size === 1 && scale.value > MIN_SCALE) {
    panStart = { x: e.clientX, y: e.clientY, tx: tx.value, ty: ty.value }
  }
}

function onPointerMove(e: PointerEvent) {
  if (!activePointers.has(e.pointerId)) return
  activePointers.set(e.pointerId, { x: e.clientX, y: e.clientY })

  if (activePointers.size === 2 && pinchStartDistance != null) {
    const [p1, p2] = Array.from(activePointers.values())
    const distance = Math.hypot(p1.x - p2.x, p1.y - p2.y)
    scale.value = clampScale(pinchStartScale * (distance / pinchStartDistance))
    if (scale.value === MIN_SCALE) {
      tx.value = 0
      ty.value = 0
    }
    return
  }

  if (panStart && scale.value > MIN_SCALE) {
    tx.value = panStart.tx + (e.clientX - panStart.x)
    ty.value = panStart.ty + (e.clientY - panStart.y)
  }
}

function onPointerUp(e: PointerEvent) {
  activePointers.delete(e.pointerId)
  if (activePointers.size < 2) pinchStartDistance = null
  if (activePointers.size === 0) panStart = null
}
</script>

<template>
  <div v-if="open && asset" class="fixed inset-0 z-40 bg-black/95">
    <!-- Backdrop click closes -->
    <div class="absolute inset-0" @click="emit('close')"></div>

    <!-- Close button -->
    <button
      type="button"
      class="absolute top-3 right-3 z-20 inline-flex items-center justify-center w-10 h-10 rounded-full bg-black/50 text-white backdrop-blur-sm transition hover:bg-black/70 active:scale-95 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/80 cursor-pointer"
      aria-label="Close detail view"
      title="Close (Esc)"
      @click="emit('close')"
    >
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
      </svg>
    </button>

    <!-- Media area -->
    <div
      class="absolute inset-0 z-10 flex items-center justify-center overflow-hidden touch-none select-none"
      :class="!isVideo ? 'cursor-zoom-in' : ''"
      @wheel="onWheel"
      @dblclick="onDblClick"
      @pointerdown="onPointerDown"
      @pointermove="onPointerMove"
      @pointerup="onPointerUp"
      @pointercancel="onPointerUp"
    >
      <!-- Loading -->
      <div v-if="loading" class="flex items-center justify-center">
        <div class="w-8 h-8 border-4 border-white/50 border-t-transparent rounded-full animate-spin"></div>
      </div>

      <!-- Video -->
      <video
        v-else-if="isVideo && (videoSrc || imageBlobUrl)"
        :src="(videoSrc || imageBlobUrl) ?? undefined"
        class="max-w-full max-h-full object-contain"
        playsinline
        webkit-playsinline
        loop
        controls
        autoplay
      />

      <!-- Image -->
      <img
        v-else-if="imageBlobUrl"
        :src="imageBlobUrl"
        :alt="asset.originalFileName"
        class="max-w-full max-h-full object-contain will-change-transform"
        :style="transformStyle"
        draggable="false"
      />

      <!-- No metadata hint -->
      <p v-if="hasInfo === false && !loading && !imageBlobUrl && !isVideo" class="text-gray-500 text-sm px-6 text-center">
        Could not load this photo at full resolution.
      </p>
    </div>

    <!-- Preview-fallback notice -->
    <div
      v-if="usingPreviewFallback"
      class="absolute top-3 left-3 z-20 px-3 py-1 rounded-full text-xs font-medium bg-black/50 text-white/90 backdrop-blur-sm"
    >
      Preview quality — original unavailable
    </div>

    <!-- Info panel -->
    <div v-if="hasInfo" class="absolute bottom-0 left-0 right-0 z-20">
      <button
        type="button"
        class="w-full flex items-center justify-center py-1.5 text-xs font-medium text-white/70 hover:text-white transition-colors cursor-pointer"
        :aria-expanded="showInfo ? 'true' : 'false'"
        @click="showInfo = !showInfo"
      >
        {{ showInfo ? 'Hide info ▾' : 'Show info ▴' }}
      </button>
      <div
        v-if="showInfo"
        class="px-4 pb-[max(1rem,env(safe-area-inset-bottom))] pt-1 bg-gradient-to-t from-black/85 to-transparent"
      >
        <dl class="flex flex-wrap gap-x-6 gap-y-1 justify-center text-xs sm:text-sm text-white/90 max-w-3xl mx-auto">
          <div v-if="formattedDateTime" class="flex gap-1">
            <dt class="text-white/50">Taken</dt>
            <dd>{{ formattedDateTime }}</dd>
          </div>
          <div v-if="cameraLabel" class="flex gap-1">
            <dt class="text-white/50">Camera</dt>
            <dd>{{ cameraLabel }}</dd>
          </div>
          <div v-if="resolutionLabel" class="flex gap-1">
            <dt class="text-white/50">Size</dt>
            <dd>{{ resolutionLabel }}</dd>
          </div>
          <div v-if="locationLabel" class="flex gap-1">
            <dt class="text-white/50">Place</dt>
            <dd>{{ locationLabel }}</dd>
          </div>
        </dl>
      </div>
    </div>
  </div>
</template>
