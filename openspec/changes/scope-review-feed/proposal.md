## Why

Every review mode today swipes the entire library (random, chronological asc/desc). Real-world curation is rarely whole-library: users want to clean up a specific album, a date range (e.g., a trip), or whittle down favorites. There is currently no way to constrain the feed, so targeted cleanup of a large library is impractical and the reviewed-cache is the only tool for skipping what has already been seen.

## What Changes

- **NEW**: A review scope — the feed SHALL be constrainable to one of: whole library (default), a single album, a date range (from/to), or favorites-only.
- **NEW**: A scope picker UI (modal, reusing the AlbumPicker pattern) to set and clear the active scope; the header shows a badge with the active scope.
- **MODIFIED**: Chronological feed queries SHALL apply the active scope as search filters (`albumId`, `isFavorite`, `takenAfter`/`takenBefore`) on `POST /search/metadata`.
- **MODIFIED**: Random feed SHALL only return assets matching the active scope (filters where the random endpoint supports them; otherwise scoped metadata fetch with client-side pick — see design).
- **MODIFIED**: Changing the scope SHALL reset the review flow (chronological queue, page, pending assets, undo history) and reload from the first matching asset.
- **MODIFIED**: The active scope SHALL persist per server:user in preferences.
- **MODIFIED**: `MetadataSearchRequest` type SHALL be extended with the scope filter fields.

## Capabilities

### New Capabilities

- `review-feed-scoping`: Constraining the review feed to a subset of the library (album, date range, favorites) and combining that scope with the existing review modes (random/chronological) and skip-videos.

### Modified Capabilities

None.

## Impact

- `src/stores/preferences.ts` — new scope state + persistence in `StoredPreferences`
- `src/composables/useImmich.ts` — apply scope filters in `fetchChronologicalBatch` and the random path; reset flow on scope change
- `src/types/immich.ts` — extend `MetadataSearchRequest` with `albumId`, `isFavorite`, `takenAfter`, `takenBefore`; add a `ReviewScope` type
- `src/components/ReviewScopePicker.vue` (new) — scope selection modal
- `src/components/AppHeader.vue` / `src/views/HomeView.vue` — scope entry point + active-scope badge; reload on scope change
- No backend changes (filters are forwarded via the existing proxy)
