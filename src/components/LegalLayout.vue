<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUiStore } from '@/stores/ui'

defineProps<{
  eyebrow: string
  title: string
  /** Publication date rendered under the title, e.g. "1 November 2026". */
  updated?: string
}>()

const uiStore = useUiStore()
const isDark = computed(() => uiStore.isDarkMode)
const route = useRoute()
const router = useRouter()

/** Current section, used to mark the active link. Falls back to the document
 *  path when the layout is rendered outside a router (tests, embeds). */
const currentPath = computed(() =>
  route?.path ?? (typeof window !== 'undefined' ? window.location.pathname : '')
)

/** Navigate in-app when a router exists, otherwise let the link load normally. */
function follow(event: MouseEvent, to: string) {
  if (!router || event.defaultPrevented) return
  if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
  event.preventDefault()
  router.push(to)
}

const links = [
  { to: '/privacy', label: 'Privacy' },
  { to: '/terms', label: 'Terms' },
  { to: '/account/delete', label: 'Delete account' },
]
</script>

<template>
  <div
    class="legal-page flex flex-col"
    :class="isDark ? 'legal-dark text-neutral-300' : 'text-neutral-700'"
  >
    <header
      class="sticky top-0 z-20 border-b backdrop-blur-md"
      :class="isDark ? 'border-white/10 bg-neutral-950/90' : 'border-neutral-200/80 bg-white/90'"
    >
      <div
        class="h-px w-full bg-gradient-to-r from-indigo-500/0 via-indigo-500/50 to-purple-500/0"
        aria-hidden="true"
      ></div>
      <div
        class="mx-auto flex max-w-2xl flex-wrap items-center gap-x-4 gap-y-2 px-5 py-3 sm:px-6"
      >
        <a
          href="/"
          class="flex items-center gap-2.5 rounded-lg focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500 focus-visible:ring-offset-2"
          :class="isDark
            ? 'text-white focus-visible:ring-offset-black'
            : 'text-neutral-900 focus-visible:ring-offset-white'"
          @click="follow($event, '/')"
        >
          <span
            class="inline-flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br from-indigo-500 to-purple-600 text-xs font-bold tracking-wide text-white shadow-sm"
            aria-hidden="true"
          >
            IS
          </span>
          <span class="text-sm font-semibold tracking-tight">Immich Swipe</span>
        </a>

        <nav
          class="order-3 -mx-1 flex w-full items-center gap-1 overflow-x-auto pb-0.5 sm:order-none sm:ml-auto sm:mx-0 sm:w-auto sm:pb-0"
          aria-label="Legal pages"
        >
          <a
            v-for="link in links"
            :key="link.to"
            :href="link.to"
            :aria-current="currentPath === link.to ? 'page' : undefined"
            class="whitespace-nowrap rounded-full px-3 py-1.5 text-xs font-medium transition-colors sm:text-[13px]"
            :class="currentPath === link.to
              ? (isDark ? 'bg-white text-black' : 'bg-neutral-900 text-white')
              : (isDark
                ? 'text-neutral-400 hover:bg-white/5 hover:text-white'
                : 'text-neutral-600 hover:bg-black/5 hover:text-neutral-900')"
            @click="follow($event, link.to)"
          >
            {{ link.label }}
          </a>
        </nav>

        <button
          type="button"
          class="ml-auto rounded-full border p-2 transition-colors sm:ml-0"
          :class="isDark
            ? 'border-white/10 bg-white/5 text-neutral-300 hover:bg-white/10 hover:text-white'
            : 'border-neutral-200 bg-white text-neutral-600 hover:bg-neutral-100 hover:text-neutral-900'"
          :aria-label="isDark ? 'Switch to light theme' : 'Switch to dark theme'"
          @click="uiStore.toggleDarkMode()"
        >
          <svg v-if="isDark" class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
          </svg>
          <svg v-else class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
          </svg>
        </button>
      </div>
    </header>

    <main class="mx-auto w-full max-w-2xl flex-1 px-5 pb-16 pt-10 sm:px-6 sm:pt-14">
      <div class="flex items-center gap-2">
        <span class="h-1.5 w-1.5 rounded-full bg-indigo-500" aria-hidden="true"></span>
        <p class="text-[11px] font-semibold uppercase tracking-[0.2em] text-indigo-500">
          {{ eyebrow }}
        </p>
      </div>
      <h1
        class="mt-4 font-serif text-[2rem] leading-[1.12] tracking-tight sm:text-4xl"
        :class="isDark ? 'text-white' : 'text-neutral-900'"
      >
        {{ title }}
      </h1>
      <p v-if="updated" class="mt-3 text-xs" :class="isDark ? 'text-neutral-500' : 'text-neutral-500'">
        Last updated {{ updated }}
      </p>
      <div class="legal-prose mt-9 sm:mt-10">
        <slot />
      </div>
    </main>

    <footer
      class="border-t"
      :class="isDark ? 'border-white/10 bg-black/20' : 'border-neutral-200 bg-neutral-50/70'"
    >
      <div
        class="mx-auto flex max-w-2xl flex-wrap items-center gap-x-3 gap-y-2 px-5 py-6 text-xs sm:px-6"
        :class="isDark ? 'text-neutral-500' : 'text-neutral-500'"
      >
        <span class="font-medium" :class="isDark ? 'text-neutral-400' : 'text-neutral-600'">
          Immich Swipe
        </span>
        <span class="opacity-40" aria-hidden="true">·</span>
        <a
          href="/privacy"
          class="rounded transition-colors hover:underline"
          :class="isDark ? 'hover:text-white' : 'hover:text-neutral-900'"
          @click="follow($event, '/privacy')"
        >
          Privacy policy
        </a>
        <a
          href="/terms"
          class="rounded transition-colors hover:underline"
          :class="isDark ? 'hover:text-white' : 'hover:text-neutral-900'"
          @click="follow($event, '/terms')"
        >
          Terms of use
        </a>
        <a
          href="/account/delete"
          class="rounded transition-colors hover:underline"
          :class="isDark ? 'hover:text-white' : 'hover:text-neutral-900'"
          @click="follow($event, '/account/delete')"
        >
          Delete your account
        </a>
        <a
          href="/login"
          class="ml-auto inline-flex items-center gap-1 font-medium transition-colors"
          :class="isDark ? 'text-neutral-300 hover:text-white' : 'text-neutral-700 hover:text-neutral-900'"
          @click="follow($event, '/login')"
        >
          Open the app
          <span aria-hidden="true">→</span>
        </a>
      </div>
    </footer>
  </div>
</template>

<style scoped>
/* The app shell locks body scrolling; legal pages are their own scroll
   surface so long-form text stays reachable on every screen size. */
.legal-page {
  height: 100vh;
  height: 100dvh;
  overflow-y: auto;
  overscroll-behavior-y: contain;
  -webkit-overflow-scrolling: touch;
}

.legal-page.legal-dark {
  background-color: #0a0a0a;
  background-image: radial-gradient(
    48rem 22rem at 50% -10rem,
    rgba(99, 102, 241, 0.18),
    transparent 72%
  );
}

.legal-page:not(.legal-dark) {
  background-color: #ffffff;
  background-image: radial-gradient(
    48rem 22rem at 50% -10rem,
    rgba(99, 102, 241, 0.1),
    transparent 72%
  );
}

@supports (padding: env(safe-area-inset-top)) {
  .legal-page {
    padding-top: env(safe-area-inset-top);
    padding-bottom: env(safe-area-inset-bottom);
  }
}

/* Readable long-form typography on top of the shared .legal-prose rules. */
.legal-prose :deep(p) {
  font-size: 0.9375rem;
  line-height: 1.75;
}

.legal-prose :deep(li) {
  font-size: 0.9375rem;
  line-height: 1.7;
  margin-top: 0.5rem;
}

.legal-prose :deep(li::marker) {
  color: #6366f1;
}

.legal-prose :deep(p + ul) {
  margin-top: 0.85rem;
}

.legal-prose :deep(ul + p) {
  margin-top: 1rem;
}

.legal-prose :deep(h2) {
  font-family: ui-serif, Georgia, Cambria, 'Times New Roman', serif;
  font-size: 1.375rem;
  font-weight: 600;
  line-height: 1.3;
  letter-spacing: -0.015em;
  margin-top: 2.5rem;
  margin-bottom: 0.7rem;
  scroll-margin-top: 6rem;
}

.legal-prose :deep(h2:first-child) {
  margin-top: 0;
}

.legal-prose :deep(.legal-lead) {
  font-size: 1.0625rem;
  line-height: 1.7;
}

.legal-prose :deep(a) {
  text-decoration-thickness: 1px;
  text-underline-offset: 3px;
  transition: color 120ms ease;
}

.legal-prose :deep(a:focus-visible) {
  outline: 2px solid #6366f1;
  outline-offset: 2px;
  border-radius: 2px;
}

.legal-dark .legal-prose :deep(a) {
  color: #a5b4fc;
}

.legal-dark .legal-prose :deep(a:hover) {
  color: #c7d2fe;
}

.legal-dark .legal-prose :deep(strong) {
  color: #f5f5f5;
}

.legal-dark .legal-prose :deep(code) {
  background: rgba(165, 180, 252, 0.14);
  color: #c7d2fe;
}

/* Small summary/contact blocks used across the legal pages. */
.legal-prose :deep(.legal-card) {
  margin: 1.75rem 0;
  border-radius: 1rem;
  border-width: 1px;
  padding: 1.25rem;
}

@media (min-width: 640px) {
  .legal-prose :deep(.legal-card) {
    padding: 1.5rem;
  }
}

.legal-page:not(.legal-dark) .legal-prose :deep(.legal-card) {
  border-color: #e5e5e5;
  background-color: #f8f8fc;
}

.legal-page.legal-dark .legal-prose :deep(.legal-card) {
  border-color: rgba(255, 255, 255, 0.1);
  background-color: rgba(255, 255, 255, 0.035);
}

.legal-prose :deep(.legal-card p) {
  font-size: 0.875rem;
  line-height: 1.65;
}

/* A link styled as a button must opt out of the prose link colour. */
.legal-prose :deep(.legal-button) {
  text-decoration: none;
}

.legal-page:not(.legal-dark) .legal-prose :deep(.legal-button),
.legal-page:not(.legal-dark) .legal-prose :deep(.legal-button:hover) {
  color: #ffffff;
}

.legal-page.legal-dark .legal-prose :deep(.legal-button),
.legal-page.legal-dark .legal-prose :deep(.legal-button:hover) {
  color: #0a0a0a;
}

.legal-prose :deep(.legal-card-label) {
  font-size: 0.6875rem;
  font-weight: 600;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: #6366f1;
}
</style>
