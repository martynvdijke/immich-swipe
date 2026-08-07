## 1. Type definitions

- [x] 1.1 Add `ImmichPerson` type (`id`, `name`, `thumbnailPath?`, `isHidden?`) to `src/types/immich.ts`
- [x] 1.2 Add `personIds?: string[]` to `MetadataSearchRequest`

## 2. Data plumbing

- [x] 2.1 Add `fetchPeople()` to `useImmich` via `apiRequest('/people')` with a session cache (mirror `albumsCache`)
- [x] 2.2 Add a person thumbnail URL helper with graceful fallback when unavailable
- [x] 2.3 Merge `personIds` into the search body in `fetchChronologicalBatch` via the shared filter builder
- [x] 2.4 Random mode: use the scoped-metadata fallback when a person is selected (page + client-side pick)

## 3. Preferences

- [x] 3.1 Add `selectedPersonId` state to `src/stores/preferences.ts` with persistence
- [x] 3.2 Add a `setSelectedPerson(id | null)` setter

## 4. UI

- [x] 4.1 Create `src/components/PersonPicker.vue` (modal): person list with thumbnails, empty state
- [x] 4.2 Add a person entry button and active-person badge in `src/components/AppHeader.vue`
- [x] 4.3 Add a person-name badge to `src/components/SwipeCard.vue` (fallback to selected person name; best-effort `asset.people` enrichment)
- [x] 4.4 Wire selection changes to reset the flow + `loadInitialAsset()`

## 5. Verification

- [x] 5.1 Run `npm run type-check` and fix errors
- [x] 5.2 Run `npm run build`
- [x] 5.3 Manual: pick person → only their photos in chronological + random; skip-videos respected; badge shows name; reload restores scope; clear → library feed; empty people list → clear message
