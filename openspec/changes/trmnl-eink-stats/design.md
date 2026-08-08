## Context

Immich Swipe is a Vue SPA + Go backend. The Go server (`server/main.go`) serves the static build, holds in-memory sessions, and reverse-proxies `/api/*` to Immich, attaching per-session credentials. Keep/delete counters live only in browser `localStorage` (`immich-swipe-stats:<server>:<user>`), incremented by the frontend stores; the backend is stateless apart from sessions.

The sandwitches project ships a Trmnl e-ink plugin (`trmnl/settings.yml` + `full/half_horizontal/half_vertical/quadrant.liquid`) that polls public JSON endpoints and renders Liquid templates on a 800×480 e-ink display. We replicate that pattern: a public stats endpoint + Liquid templates, with counting derived server-side so no frontend changes are needed.

## Goals / Non-Goals

**Goals:**
- Server-side keep/delete counters tracked by observing reverse-proxy traffic to Immich.
- A public, unauthenticated `GET /api/trmnl/stats` polling endpoint returning e-ink-friendly JSON.
- A `trmnl/` plugin directory (settings + 4 Liquid templates) mirroring the sandwitches layout.
- Counters survive process restarts when configured via an env var.
- Zero frontend changes; proxy forwarding behavior unchanged.

**Non-Goals:**
- No client→server reporting of counters (browser keeps acting as today; plain `keepPhoto` keeps are invisible to the server and not counted).
- No per-asset detail, timestamps, or historical trends on the display — only aggregate counters and optional per-user breakdown.
- No auth on the stats endpoint (Trmnl devices poll unauthenticated, mirroring sandwitches); it exposes only counts, never credentials or asset metadata.
- No changes to the Immich API surface or to how requests are proxied beyond passive observation.

## Decisions

### D1: Count in the reverse proxy, on success only
Counting happens in `proxyHandler` (server/main.go:624). For relevant methods (DELETE `/api/assets`, POST `/api/trash/restore/assets`, PUT `/api/albums/<id>/assets`, PUT `/api/assets/<id>`), the request body is read before forwarding, parsed, and rewound with `io.NopCloser(bytes.NewReader(body))` so Immich still receives it. Parsed counts are attached to the request context; a `ModifyResponse` hook applies them to the store only when the upstream status is 2xx, then clears them so a request is never double-counted.
- **Why**: optimistic counting would over-count on auth expiry / upstream errors. Success-gated counting via context + `ModifyResponse` is cheap and correct for the common case.
- **Alternatives considered**: counting optimistically at entry (rejected: wrong on failures); a wrapper `http.RoundTripper` (rejected: body is streamed, harder to inspect; context approach keeps changes local to `proxyHandler`).

### D2: StatsStore keyed by (serverURL, userName), aggregated for the endpoint
A new `StatsStore` (mutex-guarded map) records `keptCount`/`deletedCount` per `(serverURL, userName)` taken from the session. The polling endpoint returns global totals plus a `users` array with per-user counts.
- **Why**: multi-user deployments get a per-user breakdown on the display; the public endpoint still works when sessions span multiple Immich servers.
- **Alternatives considered**: single global counter (rejected: useless for multi-user setups); per-server files (overkill).

### D3: Public endpoint registered before the proxy catch-all
`GET /api/trmnl/stats` is added as an explicit case in `ServeHTTP` (server/main.go:194) **before** the `strings.HasPrefix(path, "/api/")` proxy branch, with no auth middleware. Response shape (flat, template-friendly):

```json
{
  "keptCount": 42,
  "deletedCount": 13,
  "totalCount": 55,
  "serverUrl": "https://immich.example",
  "updatedAt": "2026-08-08T10:00:00Z",
  "users": [ { "userName": "Alice", "keptCount": 40, "deletedCount": 10 } ]
}
```

- **Why**: Trmnl Liquid templates address `IDX_0.<field>` directly, so a single flat object is the least-friction contract (same pattern as sandwitches' recipe payload).
- **Alternatives considered**: nested `{stats:{...}}` (rejected: complicates templates); per-user query params (rejected: public endpoint cannot authenticate who is asking).

### D4: Counting rules (exact request mapping)
| Request | Condition | Effect |
|---|---|---|
| `DELETE /api/assets` | body `{"ids": [...], "force": false}` (or absent force) | deleted += len(ids) |
| `POST /api/trash/restore/assets` | body `{"ids": [...]}` | deleted -= len(ids) |
| `PUT /api/albums/<id>/assets` | body `{"ids": [...]}` | kept += len(ids) |
| `PUT /api/assets/<id>` | body `{"isFavorite": true}` | kept += 1 |

Non-matching requests (GET/HEAD, other paths, `force: true`, malformed JSON) are forwarded untouched and never counted. This exactly mirrors the frontend actions in `useImmich.ts` (`deletePhoto` → trash, `undoLastAction` delete → restore, `keepPhotoToAlbum`, `toggleFavorite` keep-direction). Plain `keepPhoto` makes no Immich call and is therefore not counted — a documented limitation.

### D5: Optional file persistence via `TRMNL_STATS_FILE`
If `TRMNL_STATS_FILE` is set (e.g. `/data/trmnl-stats.json`), the store loads on startup and writes atomically (temp file + rename) on every change. Unset = in-memory only (counters reset on restart).
- **Why**: Docker deployments restart; a passive wall display should not lose accumulated counts. File-backed is simpler than a DB for this payload size.
- **Alternatives considered**: SQLite/embedded DB (rejected: overkill); always-on file writes (rejected: filesystem churn on environments where the file cannot be created — logging a warning instead).

### D6: Liquid templates mirror the sandwitches plugin
`trmnl/settings.yml` (strategy `polling`, `polling_url` placeholder `https://<your-immich-swipe-host>/api/trmnl/stats`, custom `url` field for the instance, `refresh_interval` ~60, dark_mode-friendly) and four templates using `IDX_0.keptCount` / `IDX_0.deletedCount` / `IDX_0.totalCount` with `value--large` numerals, a keep/delete ratio row, an optional per-user loop, and the standard `title_bar`. No images (stats are numeric; avoids dithering complexity).
- **Why**: consistent with the proven sandwitches plugin so a user familiar with one can install the other.

## Risks / Trade-offs

- **Plain swipe-keeps not counted** → Display under-reports kept. Mitigation: README documents that kept reflects album-adds + favorites; users who want exact keeps can favorite-or-album items (or accept the limitation).
- **Counters reset on restart without `TRMNL_STATS_FILE`** → Mitigation: default documented; file persistence is a one-line env config.
- **Failed upstream request counted if parsing succeeded but request fails later** → Mitigation: success-gated `ModifyResponse`; transient network errors between proxy and Immich could still lose a count (rare, harmless for stats).
- **Multi-instance/load-balanced deployments double-split counters** → Mitigation: documented single-instance assumption; per-instance counters are inherently local.
- **Malformed/abusive polling of a public endpoint** → Mitigation: response is tiny, static, and leak-free (counts only); rate limiting out of scope.

## Migration Plan

1. Ship backend change (stats store + counting + endpoint) and `trmnl/` plugin in one release.
2. Optional: set `TRMNL_STATS_FILE` in docker-compose environment (with a bind mount for the file) before first restart if persistence is desired.
3. User installs the plugin on a Trmnl device: set `polling_url` to `<host>/api/trmnl/stats`.
4. Rollback: remove the endpoint case and `trmnl/` dir; no data migration needed (stats are derived state, safe to lose).

## Open Questions

- Should the display show a keep/delete ratio bar or a percentage? (Resolvable at implementation time via template tweaks; contract stays the same.)
