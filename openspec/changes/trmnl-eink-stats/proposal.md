## Why

Immich Swipe already tracks keep/delete counters per server+user, but only in browser `localStorage` — invisible outside the app tab. A Trmnl e-ink display (like the sandwitches recipe plugin) cannot read browser state; it polls a public URL and renders Liquid templates. Today the Go backend exposes no such endpoint, so there is no way to surface swipe stats on a wall-mounted e-ink display.

## What Changes

- **Server-side stats tracking in the Go backend**: a new in-memory stats store (with optional JSON file persistence for restart survival) that counts keep/delete actions as they flow through the existing reverse proxy to Immich — no client-side changes, no new Immich API surface.
- **Proxy-based counting**: the reverse proxy observes Immich requests and updates counters:
  - `DELETE /api/assets` (trash, `force: false`) → increments deleted count by the number of asset IDs
  - `POST /api/trash/restore/assets` (undo delete) → decrements deleted count by the number of restored IDs
  - `PUT /api/albums/<id>/assets` (keep-to-album) → increments kept count by the number of asset IDs
  - `PUT /api/assets/<id>` with `{"isFavorite": true}` (favorite-as-keep) → increments kept count
  - Plain swipe-keeps (`keepPhoto`, no Immich request) are not counted server-side — documented limitation.
- **Public polling endpoint**: `GET /api/trmnl/stats` returns JSON (`keptCount`, `deletedCount`, `serverUrl`, `updatedAt`, optional per-user breakdown) for unauthenticated polling by a Trmnl device. Registered as a first-class route before the `/api/*` proxy catch-all.
- **Trmnl e-ink plugin**: a new `trmnl/` directory in the repo mirroring the sandwitches plugin layout — `settings.yml` plus `full`, `half_horizontal`, `half_vertical`, and `quadrant` Liquid templates rendering the stats in e-ink-friendly, high-contrast styling.
- **Docs**: README section documenting how to install the plugin on a Trmnl device and point its polling URL at the stats endpoint; `env.example` documents the optional stats persistence env var.

## Capabilities

### New Capabilities
- `eink-stats`: server-side swipe stats tracking (proxy-based counting + public polling endpoint) and the Trmnl Liquid templates that render them.

### Modified Capabilities
<!-- None — `openspec/specs/` currently contains no active specs. -->

## Impact

- **Backend**: `server/main.go` gains a stats store, proxy request counting, and the `GET /api/trmnl/stats` handler; `server/main_test.go` gains unit tests for counting rules and the endpoint.
- **New files**: `trmnl/settings.yml` and `trmnl/*.liquid` (4 templates).
- **Config**: optional `TRMNL_STATS_FILE` env var for persisting stats across restarts (default: in-memory only, stats reset on restart).
- **Docs**: `README.md`, `env.example`.
- **Not affected**: frontend (`src/`), auth flow, Docker build/CI, Immich itself — the proxy continues to forward requests unchanged, counting is side-effect free.
