## Why

The app is a black-box shuffle: users swipe without knowing how much of the library they have covered or what remains. A progress signal ("1,247 of 9,000 reviewed") makes large cleanups tractable and gives feedback that the loop is actually converging. The data needed already exists in the app (kept/deleted decisions) and in Immich search responses (`assets.total`), so this is a low-cost, high-feel addition.

## What Changes

- **NEW**: Total reviewable count — the app SHALL determine the total number of assets matching the active feed configuration (scope + skip-videos) from `POST /search/metadata` (`assets.total`).
- **NEW**: Progress indicator — the header SHALL show reviewed-vs-total progress (bar + "X of Y").
- **MODIFIED**: The reviewed store SHALL expose a count of reviewed assets (kept + deleted) for the active server:user.
- **NEW**: Progress refresh on feed load, scope/order changes, and periodically during review.
- **NEW**: Completion state — when reviewed equals total, the app SHALL indicate the active feed is fully reviewed.

## Capabilities

### New Capabilities

- `review-progress-tracking`: Computing and displaying progress through the review feed (reviewed vs. total, scope-aware, per server:user), including a completion state.

### Modified Capabilities

None.

## Impact

- `src/composables/useImmich.ts` — `fetchReviewTotal()` + refresh wiring
- `src/stores/reviewed.ts` — `reviewedCount` computed (kept + deleted sizes)
- `src/components/AppHeader.vue` — progress bar + counter (+ completion state)
- `src/views/HomeView.vue` — completion UI hook
- `src/types/immich.ts` — `assets.total` already typed; no change expected
- No backend changes
