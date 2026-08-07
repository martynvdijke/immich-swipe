## Context

The app reviews one asset at a time from an unfiltered feed. `useImmich` has two feed paths: random (`POST /search/random`) and chronological (`POST /search/metadata` with `page`/`size`/`order`). Preferences are persisted per server:user in `localStorage` via `StoredPreferences`. Albums are already fetched and cached (`fetchAlbums`) for the album-hotkey / keep-to-album features, and `AlbumPicker.vue` is an existing modal pattern to reuse.

## Goals / Non-Goals

**Goals:**
- Let users constrain the review feed to an album, a date range, or favorites-only.
- Keep the scope working with both existing review orders and skip-videos.
- Persist the scope per server:user and reset the flow cleanly on change.

**Non-Goals:**
- Person/face scoping (separate change `review-by-person`).
- Stacking multiple scope dimensions at once (e.g., album + date range) — v1 selects one dimension.
- Server-side scope persistence or cross-device sync.

## Decisions

### Decision 1: Scope is a single dimension, not a filter stack

The active scope is a discriminated union:

```
ReviewScope = { kind: 'library' }
            | { kind: 'album', albumId: string }
            | { kind: 'dateRange', from: string, to: string }
            | { kind: 'favorites' }
```

Rationale: one dimension keeps the picker, persistence, and reset logic simple. Stacks (album AND date) can be added later by making the scope a record of filters without changing the feed plumbing.
Alternative considered: multi-select filter chips — rejected for v1; more UI surface and more edge cases for marginal benefit.

### Decision 2: Chronological feed applies filters directly to /search/metadata

Immich's metadata search accepts `albumId`, `isFavorite`, `takenAfter`, `takenBefore` alongside `page`/`size`/`order`/`type`. `fetchChronologicalBatch` only needs to merge the active scope's fields into the request body via a single `buildSearchFilters()` helper.
Note: exact filter field names should be confirmed against the deployed Immich version (see Open Questions); they are isolated in one type and one builder function so a rename is a one-line change.

### Decision 3: Random feed falls back to a scoped metadata fetch for non-library scopes

`POST /search/random` supports `size`/`type` but not reliably album/date/favorite filters across Immich versions. Options:
- A. Pass filters to `/search/random` — only works if the deployed version supports them; unverifiable across versions.
- B. For non-library scopes, fetch a scoped page via `/search/metadata` (order asc, page size 50) and pick a candidate client-side, paging further when candidates are already reviewed.

Decision: B for all non-library scopes. Trade-off: true randomness is reduced to "random within a page of 50"; acceptable because the page is re-fetched as it is consumed and the reviewed-cache filter still applies. For the `library` scope, keep the existing `/search/random` path unchanged.

### Decision 4: Scope changes reset the review flow

Changing scope clears `chronologicalQueue`, `chronologicalPage`, `pendingAssets`, and `actionHistory`, then calls `loadInitialAsset()`. Same pattern already used when `reviewOrder` changes (see the `watch` in `HomeView.vue`).
Rationale: assets from a different scope must not leak across the boundary; undo across scopes would be misleading.

### Decision 5: Persistence via StoredPreferences

Add `scope` to `StoredPreferences`; the existing `storageKey` (per server:user) already isolates users. Load defaults to `{ kind: 'library' }`.

## Risks / Trade-offs

- [Risk] Random-mode randomness is reduced for scoped feeds → page size 50 with client-side pick; accepted and documented.
- [Risk] Filter field names drift across Immich versions (`takenAfter` vs `fileCreatedAfter`, etc.) → isolated in one type + one builder; verified against the deployed version during implementation (task).
- [Risk] Changing scope discards undo history → accepted; the scope change is an explicit user action and the header badge makes the switch visible.
- [Risk] Large album lists make the picker heavy → reuses the existing album fetch/caching; no new requests per open.

## Open Questions

- Exact search filter field names on the deployed Immich version (`takenAfter`/`takenBefore` vs `fileCreatedAfter`/`fileCreatedBefore`; `albumId`; `isFavorite`) — verify during implementation.
- Date-range picker: native `<input type="date">` vs custom calendar — default to native.
