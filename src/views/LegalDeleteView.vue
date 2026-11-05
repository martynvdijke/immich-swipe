<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import LegalLayout from '@/components/LegalLayout.vue'
import { useLegalStore } from '@/stores/legal'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'

const legalStore = useLegalStore()
const authStore = useAuthStore()
const uiStore = useUiStore()
const legal = computed(() => legalStore.config)
const router = useRouter()

/** Keep in-app navigation when a router is present; plain links otherwise. */
function follow(event: MouseEvent, to: string) {
  if (!router || event.defaultPrevented) return
  if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
  event.preventDefault()
  router.push(to)
}

onMounted(() => legalStore.fetchLegal())
</script>

<template>
  <LegalLayout eyebrow="Account" title="Delete your account">
    <p class="legal-lead">
      Deleting your {{ legal.appName }} account is immediate. Your app account data is erased the
      moment you confirm; nothing is held for a waiting period.
    </p>

    <h2>Delete it yourself</h2>
    <p v-if="authStore.isLoggedIn">
      You are signed in as <strong>{{ authStore.currentUserName }}</strong>. Open
      <a href="/settings" @click="follow($event, '/settings')">Settings</a>, find <strong>Delete account</strong>, and
      type your user name to confirm. The app clears your local session and returns you to the sign-in
      screen.
    </p>
    <p v-else>
      <a href="/login" @click="follow($event, '/login')">Sign in</a>, then open <strong>Settings</strong> and choose
      <strong>Delete account</strong>. You will be asked to type your user name to confirm.
    </p>
    <div class="legal-card">
      <p class="legal-card-label">Where to find it</p>
      <p class="mt-2">
        Web app: <strong>Settings → Delete account</strong>. Android app: the account menu in the
        review screen. Both use the same authenticated, CSRF-protected endpoint with the same typed
        confirmation.
      </p>
      <a
        :href="authStore.isLoggedIn ? '/settings' : '/login'"
        class="legal-button mt-4 inline-flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition-colors"
        :class="uiStore.isDarkMode
          ? 'bg-white text-black hover:bg-neutral-200'
          : 'bg-neutral-900 text-white hover:bg-neutral-700'"
        @click="follow($event, authStore.isLoggedIn ? '/settings' : '/login')"
      >
        {{ authStore.isLoggedIn ? 'Open Settings' : 'Sign in to delete' }}
        <span aria-hidden="true">→</span>
      </a>
    </div>

    <h2>If you cannot sign in</h2>
    <p v-if="legal.contactEmail">
      Write to <a :href="`mailto:${legal.contactEmail}`">{{ legal.contactEmail }}</a> from the address
      on the account and ask for it to be deleted. Requests are handled within one month, usually
      much sooner.
    </p>
    <p v-else>
      Ask the administrator who runs this instance to delete the account. There is no self-service
      route that works without signing in.
    </p>

    <h2>What is removed</h2>
    <ul>
      <li>Your app account: user name, email address and password hash.</li>
      <li>The Immich API key stored with the account, and every stored Immich access token.</li>
      <li>Every browser and mobile session for the account, including the current one.</li>
      <li>Your per-user keep/delete counters.</li>
    </ul>
    <p>
      No app account row or profile is kept after deletion. Server logs and backups are handled as
      described below.
    </p>

    <h2>What is not removed</h2>
    <p>
      <strong>Your Immich account and your photos.</strong> They live on the Immich server you chose,
      not on this app. {{ legal.appName }} has no administrative access to that server, so it cannot
      delete the Immich user or the library. Deleting the app account only removes this app's copy of
      your credentials and session.
    </p>
    <p>To remove the Immich side as well:</p>
    <ul>
      <li>
        In Immich, open your account settings and delete the API key you created for this app, so the
        old key cannot be reused.
      </li>
      <li>
        If you signed in through Immich OAuth/SSO, remove the app's authorized login in Immich if
        your version offers that option, or ask the Immich administrator.
      </li>
      <li>
        If you want the Immich account and its photos gone, delete the user in Immich (as that user,
        or via the Immich administration panel). That action belongs to the Immich server's operator
        and its own retention rules.
      </li>
    </ul>

    <h2>Backups and logs</h2>
    <p>
      Backups taken before deletion may still contain the old app account data until they rotate out.
      They are protected as a whole, are not used to restore individual accounts, and are not made
      available to other users. Request and proxy logs rotate on the schedule configured by the
      operator. Analytics data, if you enabled an external endpoint, is retained by that endpoint's
      operator under their policy.
    </p>

    <h2>Before you delete</h2>
    <p>
      Download anything you need first. The app account export in
      <a href="/settings" @click="follow($event, '/settings')">Settings</a> produces JSON containing your profile, email,
      counters and session metadata (never credentials). Photos and albums must be exported or
      downloaded from Immich itself.
    </p>

    <p>
      The full detail is in the <a href="/privacy" @click="follow($event, '/privacy')">privacy policy</a>; the rules for
      using the service are in the <a href="/terms" @click="follow($event, '/terms')">terms of use</a>.
    </p>
  </LegalLayout>
</template>
