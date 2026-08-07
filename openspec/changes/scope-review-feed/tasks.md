## 1. Type definitions

- [x] 1.1 Extend `MetadataSearchRequest` in `src/types/immich.ts` with `albumId?`, `isFavorite?`, `takenAfter?`, `takenBefore?`
- [x] 1.2 Add a `ReviewScope` discriminated union type (`library` | `album` | `dateRange` | `favorites`) to `src/types/immich.ts`

## 2. Preferences

- [x] 2.1 Add `scope` state to `src/stores/preferences.ts`, defaulting to `{ kind: 'library' }`
- [x] 2.2 Persist `scope` in `StoredPreferences` and load it in `loadFromStorage`
- [x] 2.3 Add a `setScope(scope)` setter

## 3. Feed plumbing

- [x] 3.1 Add a `buildSearchFilters()` helper in `useImmich` that merges the active scope fields (`albumId` / `isFavorite` / `takenAfter` / `takenBefore`) into the search body
- [x] 3.2 Use `buildSearchFilters()` in `fetchChronologicalBatch`
- [x] 3.3 In the random path, when the scope is not `library`, fetch a scoped page via `/search/metadata` (page size 50) and pick a reviewable candidate client-side, paging further when consumed
- [x] 3.4 Reset the flow (queue/page/pending/actionHistory) when the scope changes and reload the first asset

## 4. UI

- [x] 4.1 Create `src/components/ReviewScopePicker.vue` (modal) with three sections: album list (reuse `fetchAlbums`), date-range inputs, favorites toggle
- [x] 4.2 Add a scope entry button and an active-scope badge in `src/components/AppHeader.vue`
- [x] 4.3 Wire scope confirm/clear to `preferencesStore.setScope` and `loadInitialAsset()`

## 5. Verification

- [x] 5.1 Run `npm run type-check` and fix errors
- [x] 5.2 Run `npm run build`
- [x] 5.3 Manual: album scope + chronological → only album assets; random scope → only scoped assets; switch scope mid-session → flow resets; reload → scope restored; clear scope → library feed; skip-videos respected within scope
