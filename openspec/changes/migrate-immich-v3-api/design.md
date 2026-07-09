## Context

The app connects to Immich's backend via direct API calls from the browser. Immich v3.0.0 changed several endpoint signatures and response shapes. All changes are confined to the API interface layer (`src/composables/useImmich.ts` and `src/types/immich.ts`) — no UI components, stores, or routing logic needs modification.

Current state: 3 API call sites need updating, plus TypeScript types. No architectural changes needed beyond the API composable.

## Goals / Non-Goals

**Goals:**
- All existing API calls work with Immich v3.0.0+
- TypeScript strict mode passes without errors
- Server-side video filtering for the random asset endpoint (new v3 capability)
- Minimal diff — no behavioral changes to the UI

**Non-Goals:**
- Adding new features (favorite filtering, album browsing enhancements, etc.)
- Replacing the auth layer
- Supporting backward compatibility with Immich v2.x
- Refactoring the composable architecture

## Decisions

### Decision 1: Use POST body for /search/random instead of query params
**Choice**: Replace `GET /assets/random?count=N` with `POST /search/random` with `{ size: N }` body.
**Rationale**: The new endpoint accepts a `RandomSearchDto` body. Using the existing `apiRequest` helper with a POST body is straightforward. The return type is still `AssetResponseDto[]` — same shape as before.

### Decision 2: Server-side IMAGE filter replaces client-side video skip loop
**Choice**: When `skipVideos` is enabled, pass `{ type: 'IMAGE' }` in the random search body instead of fetching everything and retrying client-side.
**Rationale**: The new endpoint supports `type` filtering, which is more efficient (single request instead of looping). When `skipVideos` is off, omit `type` to get all asset types. This simplifies the retry logic.

### Decision 3: Use `page`/`size` pagination params for metadata search
**Choice**: Replace `take`/`skip` with `page`/`size` in the `POST /search/metadata` body.
**Rationale**: Immich v3 removed `take`/`skip` in favor of `page`/`size`. The app currently sends `take: 1` which maps to `size: 1`. `page` starts at 1 (not 0-indexed like `skip`).

### Decision 4: Handle nextPage token for continuation detection
**Choice**: Check `nextPage !== null` instead of `hasNextPage === true` to detect more results.
**Rationale**: Immich v3 replaced `hasNextPage: boolean` with `nextPage: string | null`. A non-null `nextPage` means more results exist.

### Decision 5: Keep ImmichAlbum type unchanged
**Choice**: No changes to `ImmichAlbum` type.
**Rationale**: The removed fields (`owner`, `ownerId`, `assets`) are not used anywhere in the app. TypeScript won't complain about removed fields on a type that's only used for reading — they'll just be absent from the response, which is fine.

### Decision 6: Remove deprecated fields from ImmichAsset type
**Choice**: Remove `deviceAssetId`, `deviceId`, and `resized`; change `duration` type from `string` to `number | null`; add new optional fields.
**Rationale**: These fields are removed from the API response. TypeScript strict mode has `noUnusedLocals`/`noUnusedParameters` — but unused type fields don't cause errors. However, the app destructures assets: if it references removed fields, it will get `undefined`. Audit showed the app only uses `id`, `isFavorite`, `type`, `thumbnailPath` — so the removed fields are safe to delete from the type.

## Risks / Trade-offs

- **[Risk] Missed API change**: If Immich changed other endpoints the app uses (`PUT /assets/:id`, `DELETE /assets`, `POST /trash/restore/assets`, `PUT /albums/:id/assets`) — these were verified unchanged in v3 source code.
- **[Risk] Type mismatch on `duration`**: The app doesn't use `duration` in any component, so changing `string` → `number | null` is safe. If a future feature uses it, the type will be correct.
- **[Trade-off] No backward compatibility**: After this change, the app will not work with Immich v2.x. This is intentional — v3 has been stable and widely adopted.
