## Why

Immich already runs face recognition and exposes people. Curating "all photos of a person" is a distinct, high-value cleanup task (e.g., collect every picture of grandma, or triage a person's face cluster) that neither whole-library nor album scoping can express. The app currently has no way to review along the person dimension, even though Immich's metadata search supports `personIds` filtering.

## What Changes

- **NEW**: Fetch the person list from Immich (`GET /people`) with name and face thumbnail.
- **NEW**: A person picker UI to choose which person to review.
- **NEW**: Person-scoped review feed — assets depicting the selected person, via `personIds` on `POST /search/metadata`.
- **NEW**: Identity display while person-scoped — the person's name shown on the card/header so the reviewer knows which face cluster they are in.
- **MODIFIED**: Preferences persist the selected person per server:user.
- **MODIFIED**: `src/types/immich.ts` extended with an `ImmichPerson` type and `personIds` on the search request.

## Capabilities

### New Capabilities

- `person-scoped-review`: Selecting a person and reviewing only the assets depicting that person, with identity display, compatible with the existing review orders and skip-videos.

### Modified Capabilities

None.

## Impact

- `src/composables/useImmich.ts` — `fetchPeople()`; `personIds` in the search filter builder; reset on person change
- `src/components/PersonPicker.vue` (new) — person selection modal
- `src/components/SwipeCard.vue` — person-name badge (fallback to the selected person's name; best-effort `asset.people` enrichment)
- `src/stores/preferences.ts` — selected-person state + persistence
- `src/types/immich.ts` — `ImmichPerson` type; `personIds` on `MetadataSearchRequest`
- `src/views/HomeView.vue` / `src/components/AppHeader.vue` — picker entry point + active-person badge
- No backend changes (proxy forwards everything)
