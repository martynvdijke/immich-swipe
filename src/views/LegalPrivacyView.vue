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
  <LegalLayout eyebrow="Legal" title="Privacy policy" :updated="lastUpdated">
    <p class="legal-lead">
      What {{ legal.appName }} stores about you, why it stores it, who else sees it, and how to
      get it back or delete it.
    </p>

    <aside class="legal-card" aria-label="Key privacy details">
      <p class="legal-card-label">At a glance</p>
      <dl class="mt-4 grid gap-x-6 gap-y-4 sm:grid-cols-2">
        <div>
          <dt class="text-[11px] uppercase tracking-wide" :class="uiStore.isDarkMode ? 'text-neutral-500' : 'text-neutral-500'">
            Controller
          </dt>
          <dd class="mt-1 text-sm font-medium" :class="uiStore.isDarkMode ? 'text-white' : 'text-neutral-900'">
            {{ legal.controller }}
          </dd>
        </div>
        <div>
          <dt class="text-[11px] uppercase tracking-wide" :class="uiStore.isDarkMode ? 'text-neutral-500' : 'text-neutral-500'">
            Hosted in
          </dt>
          <dd class="mt-1 text-sm font-medium" :class="uiStore.isDarkMode ? 'text-white' : 'text-neutral-900'">
            {{ legal.hostingRegion }}
          </dd>
        </div>
        <div>
          <dt class="text-[11px] uppercase tracking-wide" :class="uiStore.isDarkMode ? 'text-neutral-500' : 'text-neutral-500'">
            Privacy contact
          </dt>
          <dd class="mt-1 text-sm font-medium" :class="uiStore.isDarkMode ? 'text-white' : 'text-neutral-900'">
            <a v-if="legal.contactEmail" :href="`mailto:${legal.contactEmail}`">{{ legal.contactEmail }}</a>
            <span v-else>Through your account or the administrator</span>
          </dd>
        </div>
        <div>
          <dt class="text-[11px] uppercase tracking-wide" :class="uiStore.isDarkMode ? 'text-neutral-500' : 'text-neutral-500'">
            Your photos
          </dt>
          <dd class="mt-1 text-sm font-medium" :class="uiStore.isDarkMode ? 'text-white' : 'text-neutral-900'">
            Stay on your Immich server
          </dd>
        </div>
      </dl>
    </aside>

    <h2>Who is responsible</h2>
    <p>
      <strong>{{ legal.controller }}</strong> is the controller for the account data described in
      this policy and runs this instance of {{ legal.appName }}. The instance is hosted in
      {{ legal.hostingRegion }}.
    </p>
    <template v-if="legal.contactEmail">
      <p>
        For any privacy question, or to exercise a right below, write to
        <a :href="`mailto:${legal.contactEmail}`">{{ legal.contactEmail }}</a>.
        <template v-if="legal.contactAddress">Postal address: {{ legal.contactAddress }}.</template>
      </p>
    </template>
    <p v-else>
      This instance has not published a dedicated privacy address. Requests can be made from the
      account itself (see <a href="/account/delete" @click="follow($event, '/account/delete')">Delete your account</a>), or
      through the administrator who runs this instance.
    </p>

    <h2>Two different services: Swipe and your Immich server</h2>
    <p>
      {{ legal.appName }} is a client that connects to an Immich server. The default build points
      at the hosted backend
      <template v-if="legal.defaultServerUrl"><code>{{ legal.defaultServerUrl }}</code></template>
      <template v-else>you choose at sign-in</template>, and you may instead enter your own Immich
      server or another self-hosted instance.
    </p>
    <p>
      Your photos, albums, faces, search results and the Immich account itself live on the Immich
      server you connect to. That server is operated by whoever runs it — a separate controller (or
      processor, where it acts for you). {{ legal.appName }} does not store your photo library; it
      proxies requests to your chosen Immich server using the API key or access token you provide.
    </p>
    <p>
      Deleting your {{ legal.appName }} account does <strong>not</strong> delete your Immich account
      or any photos. See <a href="/account/delete" @click="follow($event, '/account/delete')">Delete your account</a> for how
      to remove each of them.
    </p>

    <h2>What is stored</h2>
    <ul>
      <li>
        <strong>Account.</strong> Your user name, an optional email address, and a password hash if
        you set a Swipe password. Passwords are hashed with PBKDF2-HMAC-SHA256 (600,000 iterations,
        random salt) and are never stored or sent in plain text.
      </li>
      <li>
        <strong>Immich credentials.</strong> The Immich API key you enter, or the Immich access
        token issued by Immich when you sign in through its OAuth/SSO flow. These are kept only so
        the server can proxy requests to your Immich server on your behalf.
      </li>
      <li>
        <strong>Sessions.</strong> A random session token in an <code>HttpOnly</code> cookie named
        <code>swipe_session</code>. The matching server-side session record also holds the Immich
        credentials above, and, for OAuth sessions, the Immich user id and email address returned by
        Immich.
      </li>
      <li>
        <strong>Counters.</strong> How many photos you kept or marked for deletion. These are
        per-account statistics used by the app and its optional e-ink display endpoint.
      </li>
      <li>
        <strong>Technical logs.</strong> Requests reach the server with an IP address and usually a
        user agent. Reverse-proxy and application logs may retain these for rate limiting, abuse
        prevention and diagnosing failures, and then rotate.
      </li>
      <li v-if="legal.smtpConfigured">
        <strong>Email delivery.</strong> This instance has SMTP configured, so password-reset and
        test messages are sent through an email provider. That provider sees the recipient address
        and message content. SMTP credentials belong to the operator, not to your account.
      </li>
      <li>
        <strong>Optional analytics and tracing.</strong> The web app lets a signed-in user switch on
        Umami page-view analytics and/or OpenTelemetry browser tracing, pointing at servers they
        configure; both are off unless enabled and can be disabled at any time in Settings. Their
        configuration is stored in your browser, and any data goes to the endpoint you chose.
        <template v-if="legal.telemetryConfigured">
          The server operator has also enabled server-side OpenTelemetry for request timings; those
          traces describe server operations and are not used to build a profile of you.
        </template>
        <template v-else>
          This server does not run server-side OpenTelemetry tracing.
        </template>
        There are no advertising cookies, third-party tracking pixels or cross-site trackers.
      </li>
    </ul>

    <h2>Why it is stored, and the legal basis</h2>
    <ul>
      <li>
        <strong>To run your account and the review tool</strong> — signing you in, proxying your
        requests to your Immich server, and showing your keep/delete counters. Legal basis:
        performance of a contract with you.
      </li>
      <li>
        <strong>To keep the service secure</strong> — rate limiting, abuse prevention and diagnosing
        failures. Legal basis: legitimate interest in a working, safe service.
      </li>
      <li>
        <strong>To answer legal requests and keep required records</strong> — for example a record
        that an account was deleted. Legal basis: legal obligation.
      </li>
      <li>
        <strong>Optional analytics, if you switch it on</strong> — understanding usage through the
        endpoint you configure. Legal basis: your consent, withdrawable in Settings at any time.
      </li>
    </ul>

    <h2>Who else sees it</h2>
    <ul>
      <li>
        <strong>Your Immich server</strong> receives the requests you make through the app, together
        with the API key or access token used to authenticate them. Its operator has their own
        privacy policy; this policy does not cover it.
      </li>
      <li><strong>The hosting provider</strong> running this server and its backups.</li>
      <li v-if="legal.smtpConfigured">
        <strong>An email provider</strong>, which delivers password-reset and test messages.
      </li>
      <li>
        <strong>Optional analytics/tracing providers</strong>, only if you enable them and only to
        the endpoint configured.
      </li>
    </ul>
    <p>
      Account data is not sold, rented or shared for advertising. Some providers may process data
      outside the European Economic Area; where that happens, transfers rely on an adequacy decision
      or the provider's standard contractual clauses.
    </p>

    <h2>Cookies</h2>
    <p>
      The web app sets two first-party cookies. <code>swipe_session</code> is
      <code>HttpOnly</code> and <code>SameSite=Lax</code> and keeps you signed in.
      <code>swipe_csrf</code> is readable by the app and holds a random token echoed in a header to
      block cross-site request forgery; it contains no personal data. The app also stores UI
      preferences and per-user review history in your browser's local storage; those stay on your
      device and are removed when you clear site data.
    </p>
    <p>
      There is no advertising cookie and no consent banner, because nothing optional is set without
      your action.
    </p>

    <h2>How long it is kept</h2>
    <p>
      Account data is kept while your account exists. Sessions expire after 24 hours of inactivity
      and are extended while you use the app. Per-user counters are kept until you delete your
      account. Request logs and rate-limit records rotate on the schedule of the server and its
      reverse proxy.
    </p>
    <p>
      When you delete your account, the account row (user name, email, password hash and Immich API
      key), all of its sessions (including Immich access tokens) and its counters are erased
      immediately. Backups taken before the deletion may still contain the old data until they
      rotate out; they are not used to restore individual accounts and are protected as a whole.
    </p>

    <h2>Your rights</h2>
    <p>
      Under the GDPR you can ask to:
    </p>
    <ul>
      <li><strong>access</strong> your data — use the JSON export in Settings, or ask the contact above;</li>
      <li><strong>rectify</strong> it — most fields are editable in Settings;</li>
      <li>
        <strong>erase</strong> it — use
        <a href="/account/delete" @click="follow($event, '/account/delete')">Delete your account</a>; it takes effect
        immediately, without a waiting period;
      </li>
      <li><strong>restrict</strong> processing, or <strong>object</strong> to processing based on legitimate interest;</li>
      <li><strong>port</strong> it — the Settings export is machine-readable JSON;</li>
      <li>
        <strong>complain</strong> to a supervisory authority — the data protection authority in the
        EU member state where you live, work, or where an alleged infringement occurred.
      </li>
    </ul>
    <div class="legal-card">
      <p class="legal-card-label">Exercising these rights</p>
      <p v-if="legal.contactEmail" class="mt-2">
        Requests that are not covered by self-service can be sent to
        <a :href="`mailto:${legal.contactEmail}`">{{ legal.contactEmail }}</a>; we answer within one
        month, as the GDPR requires.
      </p>
      <p v-else class="mt-2">
        Requests that are not covered by self-service can be made through the administrator who
        runs this instance; we answer within one month, as the GDPR requires.
      </p>
    </div>

    <h2>Children</h2>
    <p>
      This service is not intended for anyone under 16, and accounts for younger users are not
      knowingly created. If you believe a child has created an account, contact the operator so it
      can be removed.
    </p>

    <h2>Changes</h2>
    <p>
      If this policy changes in a way that affects you, the change is announced in the app before it
      takes effect. The current version is always available at
      <a href="/privacy" @click="follow($event, '/privacy')">/privacy</a>.
    </p>

    <p>
      See also the <a href="/terms" @click="follow($event, '/terms')">terms of use</a> and
      <a href="/account/delete" @click="follow($event, '/account/delete')">account deletion instructions</a>.
    </p>
  </LegalLayout>
</template>
