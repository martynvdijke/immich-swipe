<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import LegalLayout from '@/components/LegalLayout.vue'
import { useLegalStore } from '@/stores/legal'
import { useUiStore } from '@/stores/ui'

const legalStore = useLegalStore()
const uiStore = useUiStore()
const legal = computed(() => legalStore.config)
const lastUpdated = '1 November 2026'
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
  <LegalLayout eyebrow="Legal" title="Terms of use" :updated="lastUpdated">
    <p class="legal-lead">
      The short version: use the app on an Immich server you are allowed to access, keep your
      credentials to yourself, and do not abuse the service.
    </p>

    <h2>Your account</h2>
    <p>
      You need an account to use {{ legal.appName }}. Keep one account per person, use an address you
      can receive mail at if you set one, and keep your password and Immich API key private. You are
      responsible for activity under your account; if you think someone else has access, change your
      password and revoke the API key in Immich, then sign out everywhere.
    </p>
    <p>
      You can delete your account at any time from
      <a href="/account/delete" @click="follow($event, '/account/delete')">Delete your account</a>. The operator may suspend or
      remove accounts that break these terms or put the service or other users at risk.
    </p>

    <h2>Your Immich server and your content</h2>
    <p>
      {{ legal.appName }} does not host your photo library. It connects to the Immich server you
      choose and acts with the credentials you provide. You must have the right to access that server
      and the photos in it. Your use of the Immich server is governed by its operator's terms and
      privacy policy as well as these terms.
    </p>
    <p>
      You keep ownership of your photos and other content. This app claims no licence over them
      beyond what is needed to proxy your requests and display the review interface. Actions you take
      (keeping, trashing, adding to albums or favourites) are sent to your Immich server and are
      subject to its behaviour, including retention and trash settings.
    </p>

    <h2>What is not allowed</h2>
    <ul>
      <li>Accessing an Immich server or another person's photos without permission.</li>
      <li>Scraping, bulk-downloading or reselling data you do not own.</li>
      <li>Automated account creation, evading rate limits, or probing the service for vulnerabilities without asking first.</li>
      <li>Uploading malware, or using the service to break the law or infringe someone else's rights.</li>
      <li>Impersonating another person or a brand you do not represent.</li>
    </ul>

    <h2>Availability and changes</h2>
    <p>
      This is a small, self-hostable service offered as it is. It can go down for maintenance, lose
      data if a backup fails, or shut down entirely. Do not rely on it as the only copy of anything
      you care about — your Immich server is the source of truth, and you can export your app account
      data from Settings.
    </p>
    <p>
      Features may change or be withdrawn. Material changes to these terms are announced in the app
      before they take effect; continuing to use the service after that means you accept the updated
      terms.
    </p>

    <h2>Liability</h2>
    <p>
      To the extent the law allows, the operator is not liable for indirect or consequential loss
      arising from use of the service, including loss of photos or other data on your Immich server.
      Nothing here limits liability that cannot be limited by law, including for death, personal
      injury or fraud, or your statutory consumer rights.
    </p>

    <h2>Governing law</h2>
    <p>
      These terms are governed by the law of the country in which the operator of this instance is
      established, without affecting consumer protections you have where you live.
    </p>

    <h2>Contact</h2>
    <div class="legal-card">
      <p class="legal-card-label">Questions about these terms</p>
      <p v-if="legal.contactEmail" class="mt-2">
        Write to <a :href="`mailto:${legal.contactEmail}`">{{ legal.contactEmail }}</a>.
      </p>
      <p v-else class="mt-2">
        Questions about these terms can be raised with the administrator who runs this instance.
      </p>
      <p class="mt-2" :class="uiStore.isDarkMode ? 'text-neutral-400' : 'text-neutral-600'">
        The <a href="/privacy" @click="follow($event, '/privacy')">privacy policy</a> explains what data is stored and how
        to remove it.
      </p>
    </div>
  </LegalLayout>
</template>
