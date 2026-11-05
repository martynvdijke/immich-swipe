# Legal pages and data protection

Immich Swipe ships the three pages a hosted, account-based service needs: a
privacy policy, terms of use, and a deletion page. They are **public** routes
reachable without signing in, because a store reviewer and a prospective user
both need to read them before they have an account.

| Route | View | Purpose |
| --- | --- | --- |
| `GET /privacy` | `src/views/LegalPrivacyView.vue` | What is stored, why, who processes it, retention, transfers, rights |
| `GET /terms` | `src/views/LegalTermsView.vue` | Account rules, acceptable use, availability, liability |
| `GET /account/delete` | `src/views/LegalDeleteView.vue` | How to delete, what is removed, and what stays on Immich |

The views read their facts from the public `GET /api/legal` endpoint
(`server/legal.go`), so the text cannot claim a processor (SMTP, tracing) that
this instance is not running.

## Configuration

The pages render sensible text with no configuration at all, but an instance
that collects real data should name its operator. All four values are env-only
(restart to apply), never database-backed:

```sh
LEGAL_CONTROLLER_NAME="Example Photo Cooperative"
LEGAL_CONTACT_EMAIL=privacy@example.com
LEGAL_CONTACT_ADDRESS="1 Example Street, 1000 AA Amsterdam"
LEGAL_HOSTING_REGION="the Netherlands"
```

- `LEGAL_CONTROLLER_NAME` is the GDPR "controller". Empty falls back to
  "the operator of this Immich Swipe instance".
- `LEGAL_CONTACT_EMAIL` adds a `mailto:` contact and enables the email fallback
  on the deletion page. Empty removes both rather than inventing an address.
- `LEGAL_CONTACT_ADDRESS` is optional; a postal address is good practice but not
  mandatory in the text.
- `LEGAL_HOSTING_REGION` describes where the server runs (default "the European
  Union"), used by the transfer paragraph.

The policy also adapts to the instance: the email-delivery paragraph appears
only when SMTP is configured (`SMTP_HOST` or DB-backed email settings), and the
tracing paragraph only when `OTEL_EXPORTER_OTLP_ENDPOINT` or
`OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` is set. An instance cannot accidentally
claim a processor it is not running. The links are relative (`/privacy`,
`/terms`, `/account/delete`), so the public origin only needs to be correct in
the browser.

## What export and deletion actually do

Both endpoints require a signed-in Swipe session (opaque Bearer token or the
`swipe_session` cookie) and are CSRF-protected by the server.

### Export — `GET /api/auth/account/export`

Returns a JSON attachment (`immich-swipe-account.json`) containing:

- user name, server URL, optional email, account creation time;
- whether a password and an Immich API key are set (**booleans only**);
- keep/delete counters for that server/user;
- non-secret metadata for each live session (mode, expiry, Immich user id/email).

It deliberately contains **no secrets and no photo data**: no session token, no
Immich API key, no access token. Photos and the Immich account live on the
Immich server at the URL shown in the export.

### Deletion — `DELETE /api/auth/account`

The caller must type their user name as an explicit confirmation
(`confirmation_required` / `confirmation_mismatch` otherwise). The sequence
(`server/account_delete.go`) is:

1. Best-effort `POST /auth/logout` against Immich for each access-token session,
   revoking those Immich access tokens.
2. Delete **all** Swipe sessions for that server/user (and therefore every
   stored Immich API key or access token).
3. Delete the local account row (user name, email, password hash, Immich API
   key) and the per-user keep/delete counters.
4. Clear the session cookie.
5. Log only `Account deleted: server=<host>` — no user name, no credential — so
   server logs do not become a residual PII store.

It does **not** delete the Immich account, albums, photos or trash: this app has
no administrative access to the user's Immich server and only proxies it. API
keys cannot be revoked by Swipe without their Immich-side id, so the local
reference is erased and the deletion page tells the user to revoke the key in
Immich itself. Change the page copy if this behaviour changes.

## Checklist for an EU launch

- [ ] Set all four `LEGAL_*` values, then open `/privacy` and confirm they render.
- [ ] Confirm the public origin is HTTPS; the policy links are relative.
- [ ] Identify the **controller** (the operator) and a contact for data-subject
      requests; record the **lawful basis** for each purpose (contract for the
      account and proxying, legal obligation for deletion records, consent for
      optional analytics/tracing).
- [ ] Sign a **data processing agreement** with the hosting provider, and with
      any SMTP/analytics/tracing provider the deployment uses.
- [ ] Check the hosting region matches `LEGAL_HOSTING_REGION`; transfers outside
      the EEA must rely on an adequacy decision or **SCCs**.
- [ ] Write down a **retention period** for account data, counters, logs and
      rotated backups. Sessions expire after 24h of inactivity; accounts and
      counters last until deletion; the policy promises no fixed backup window,
      so make the real one match.
- [ ] Confirm that backups are covered by the same retention and deletion
      commitments (a deleted row can persist in a backup until it rotates out).
- [ ] Keep a **record of processing activities** — for a single-instance
      deployment a page in your own docs is enough.
- [ ] Honour data-subject rights and answer requests **within one month**
      (access, rectification, erasure, portability, objection). The Settings
      export and delete buttons cover access/portability/erasure for app data.
- [ ] Publish the deletion URL in the Play listing (see
      [`docs/play-store.md`](play-store.md)).
- [ ] Review this page whenever a processor, SDK or storage location changes.

## Google Play deletion URL

Play Console → **App content → Data deletion** requires a public URL that,
without installing the app or signing in, explains how to request deletion and
what is deleted/retained. Use `https://<your-host>/account/delete`. The same
operation is available in app at **Settings → Account → Delete account**. A
reviewer must be able to reach the page at review time; a URL that only works
behind the operator's VPN will fail.
