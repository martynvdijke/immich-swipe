## Context

`MetadataSearchResponse.assets.total` is already typed (`src/types/immich.ts`) but unused. The reviewed store keeps `kept`/`deleted` Sets per server:user. The feed already knows its own configuration (order, scope, skip-videos) — the denominator must mirror that configuration exactly or progress is misleading.

## Goals / Non-Goals

**Goals:**
- Show accurate, scope-aware progress (reviewed / total) with minimal requests.
- Surface a clear "done" state when the active feed is exhausted.
- Work for all existing modes (random, chronological, skip-videos) and, once landed, all scopes (`scope-review-feed`, `review-by-person`).

**Non-Goals:**
- Server-side or cross-device progress sync.
- Charts/history of past sessions (the per-session kept/deleted stats remain separate).
- Time-to-completion or "assets per hour" analytics.

## Decisions

### Decision 1: Denominator mirrors the active feed query exactly

The total request uses the exact same filter set as the feed: same scope filters (album/person/date/favorites once those changes have landed), same `type: 'IMAGE'` when skip-videos is on (order is irrelevant). Concretely: one `POST /search/metadata` with `size: 1` and `page: 1`, reading `assets.total`.
Rationale: progress must answer "how many assets would this feed show me?" — anything else misleads (e.g., counting videos while skip-videos is on).

### Decision 2: Numerator = reviewed decisions for the active server:user

`reviewedCount = kept.size + deleted.size` from the reviewed store. Undo decrements automatically because `unmarkReviewed` removes from both Sets. The counters in the ui store are session stats; the reviewed store is the source of truth for progress.
Note: assets reviewed in an earlier session count toward progress immediately — correct for "how much of the library is done".

### Decision 3: Refresh cadence is event-driven with a periodic nudge

Refresh total on: feed load (`loadInitialAsset`), and on review-order / scope / skip-videos changes. Additionally re-fetch total every 25 review actions (a cheap single request) so progress stays fresh during long sessions without hammering the API.
Trade-off: `total` is a snapshot; new uploads appear at the next refresh. Accepted.

### Decision 4: Progress display lives in the header

A slim bar under the header (or inside the stats chip) showing fill = reviewed/total (capped at 100%) plus text "X / Y". Completion: when reviewed >= total and total > 0, show a "Library reviewed" state; the existing reset action starts over. Guard against division by zero and against the reviewed set exceeding total (e.g., assets deleted upstream) by capping at 100%.

## Risks / Trade-offs

- [Risk] `total` can change (new uploads, deletions in Immich) → refresh cadence; progress is a snapshot and may briefly over/under-report.
- [Risk] Count endpoint cost on very large libraries → single `size: 1` request, throttled by the cadence above.
- [Risk] Skip-videos/scope toggles change the denominator → total always re-derived from the active config, never cached across config changes.
- [Risk] Done-state false positives when reviewed > total (upstream deletions) → cap at 100% and base "done" on a fresh fetch with reviewed >= total.

## Open Questions

- Should progress be per-scope or global once scoping (A/B) lands? Default: per active scope (matches "how much of this feed is done").
