## 1. Reviewed store

- [x] 1.1 Add a `reviewedCount` computed to `src/stores/reviewed.ts` (`kept.size + deleted.size`) and expose it

## 2. Total fetching

- [x] 2.1 Add `fetchReviewTotal()` to `useImmich` — `POST /search/metadata` with the active feed filters and `size: 1`, read `assets.total`
- [x] 2.2 Add a `reviewTotal` ref exposed from `useImmich`
- [x] 2.3 Call `fetchReviewTotal()` on `loadInitialAsset` and on review-order / scope / skip-videos changes
- [x] 2.4 Re-fetch total every 25 review actions (counter in `useImmich`)

## 3. UI

- [x] 3.1 Add a progress bar + "X / Y" text to `src/components/AppHeader.vue` (capped at 100%, hidden when total is 0 or unknown)
- [x] 3.2 Add a completion indication ("Library reviewed") when reviewed >= total > 0, with the existing reset action to restart
- [x] 3.3 Handle the reviewed > total edge case (upstream deletions) by capping at 100%

## 4. Verification

- [x] 4.1 Run `npm run type-check` and fix errors
- [x] 4.2 Run `npm run build`
- [x] 4.3 Manual: progress shows X/Y and matches kept+deleted; undo decrements; skip-videos/scope/order changes recompute the total; reaching reviewed == total shows completion; reset clears it
