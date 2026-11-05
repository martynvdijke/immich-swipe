# Google Play submission

What the Play Console asks for, and the answers that match what Immich Swipe
actually does. Update this file whenever a permission, SDK or server feature
changes — the Data safety form is a legal declaration, not a formality.

Immich Swipe is a **client to a self-hosted (or hosted) backend**. The Android
app stores only the session token, server URL, user name and review state on the
device; the backend operator holds the account, credentials and counters. The operator of the
deployment is therefore the party that must answer for its data practices and
keep the policy and the Data safety form true. A change to the backend (new
SMTP provider, analytics, tracing, storage) can change the answers below.

## Before the first upload

| Requirement | Where | Immich Swipe answer |
| --- | --- | --- |
| App must exist | Play Console → Create app | Name, default language, "App" + "Free" |
| Privacy policy URL | Store listing → Privacy policy | `https://<your-host>/privacy` — must load without signing in |
| Account deletion URL | App content → Data deletion | `https://<your-host>/account/delete` |
| Data deletion in app | App content → Data deletion | Yes: **Settings → Account → Delete account** |
| Sign-in details | App content → App access | Demo account credentials (see below), or "all functionality is restricted" if the instance is invite-only |
| Ads | Store listing | No advertising SDK and no ads by default |
| Target audience | App content | Not designed for children; do not opt into the Families programme |

Play requires the deletion URL to be reachable **and** to explain what happens,
not just to exist. `/account/delete` does both. The URL is served by the same
Go backend as the app, so it is public and does not need a session.

## Demo account for app access

A reviewer cannot create an Immich account or supply an API key, so enable the
optional demo login on the instance they will use:

```bash
DEMO_USER_ENABLED=true
DEMO_USER_NAME=demo
DEMO_USER_PASSWORD=<a review-only password>
DEMO_USER_API_KEY=<API key of a dedicated Immich user>
# DEMO_USER_SERVER_URL=https://immich.example.com   # default: IMMICH_SERVER_URL
# DEMO_USER_EMAIL=review@example.com               # optional, shown to the reviewer
```

`GET /api/auth/config` then returns `demoEnabled`, `demoUserName`,
`demoPassword` and `demoServerUrl`, and the login page offers the credentials.
Enter them in **App access → Sign-in details** in the Play Console.

> **Security:** enabling the demo publishes its user name **and password** to
> anyone who can reach the instance. Use a dedicated Immich user with a
> non-sensitive library and its own API key — never a personal or admin key —
> and turn the demo off after review.

If the instance is invite-only and no demo should exist, answer "all
functionality is restricted" and explain how a reviewer obtains access; an
unreachable login will otherwise fail review.

## Data safety answers

The Android app talks to the backend's `/api` proxy only. These answers assume
the default build: no crash reporting, no advertising SDK, no third-party
analytics or push SDK.

**Does your app collect or share any of the required user data types?** Yes.

| Data type | Collected | Shared | Purpose | Optional? | Ephemeral? |
| --- | --- | --- | --- | --- | --- |
| Personal info → Name (user name/handle) | Yes | No | Account management, app functionality | No (required to sign in) | No |
| Personal info → Email address | Yes, if the account has one | No | Account management, password reset | Yes (email is optional) | No |
| Personal info → User IDs | Yes (account and session identifiers) | No | Account management, authentication | No | No |
| App activity → Other actions | Yes (keep/delete/favorite/album actions, keep-delete counters) | No | App functionality, review history | No | No |
| App info and performance → Other app performance data / diagnostics | Yes (server request/IP and error logs, when the operator keeps them) | No | Security, abuse prevention, debugging | No | No |
| Photos and videos → Photos | **No** — displayed from the user's Immich server, not stored by Swipe | No | — | — | — |

Notes that matter during review:

- **Photos are not collected by the app.** Immich Swipe proxies the user's own
  Immich server: the photos, albums and the Immich account stay there. Swipe
  stores only the app account (user name, optional email, password **hash**),
  the Immich API key or access token needed to proxy requests, the Swipe session
  token, and keep/delete counters. Do not tick "Photos" for this app.
- **Credentials are server-side.** The Android app keeps only the Swipe session
  token, server URL, user name and reviewed asset IDs in Android secure storage.
  Immich API keys/access tokens are held by the backend, never by the device.
- **No ads and nothing is sold.** "Contains ads" is No; "sharing for
  advertising" is No everywhere.
- **No optional analytics in the default build.** The web app can be pointed at
  a self-hosted Umami instance and/or an OpenTelemetry collector by a signed-in
  user under Settings, and the server can export OTLP traces via `OTEL_*` env
  vars. Those are **operator/user-configured**, not built-in SDKs; if a
  deployment enables them, the operator must amend the policy and this table.
- **Technical logs.** The Go backend logs requests and deletion events without
  raw credentials or the user name. If the operator retains logs, list them as
  diagnostics as above.
- **SSO** uses an in-app browser (`webview_flutter`) to the user's identity
  provider. The app itself receives only the resulting swipe session; the IdP's
  own data practices are outside this app's declaration.

## Permissions and platform details

The Android manifest requests **only `INTERNET`**. There is no camera, storage,
location, notification or contacts permission, and the app does not declare an
advertising ID.

- Package: `xyz.vandijke.immich_swipe`
- Min SDK 24, target/compile SDK 36
- `android:allowBackup` is not relied on for credentials: the session token is
  held in `flutter_secure_storage`, not in shared preferences.
- Media is fetched with the app's `Authorization` header and rendered from
  memory; no photo is written to shared storage.

## Testing requirements

Personal developer accounts created after November 2023 must run a closed test
with **at least 12 testers for 14 continuous days** before production access is
granted. Internal testing does not count towards it.

- Internal testing: up to 100 testers, no review; link from the Testers tab.
- Closed testing: `https://play.google.com/apps/testing/<package>`, up to 1,000
  testers, needs a review pass.

Because every tester needs a reachable backend, run the closed test against a
hosted instance with the demo account (or give each tester their own backend URL
and account). Verify the demo credentials still work on the day the track opens.

## Pre-upload checklist

- [ ] `/privacy` and `/account/delete` load from a clean browser with no session.
- [ ] `LEGAL_CONTROLLER_NAME`, `LEGAL_CONTACT_EMAIL` and
      `LEGAL_HOSTING_REGION` are set and render on `/privacy`.
- [ ] The deletion page explains that the Immich account and photos are **not**
      deleted by Swipe.
- [ ] Demo login is enabled for review, or App access explains the restriction.
- [ ] Demo credentials are in App access and were tested on the review build.
- [ ] `DEMO_USER_API_KEY` belongs to a dedicated, non-admin Immich user.
- [ ] Data safety answers match the table above and the deployed backend.
- [ ] No analytics/tracing/ads SDK has been added without updating this file.
- [ ] Version code is higher than the previous upload (`pubspec.yaml` build
      number; Flutter defaults to `+1`).
- [ ] The deletion URL is public HTTPS and the certificate is valid.
