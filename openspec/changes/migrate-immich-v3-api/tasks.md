## 1. Type Definitions

- [x] 1.1 Update `ImmichAsset` type: remove `deviceAssetId`, `deviceId`, `resized`; change `duration` to `number | null`; add `visibility`, `checksum`, `isEdited`, `stack`, `people`, `tags`, `duplicateId`
- [x] 1.2 Verify `ImmichAlbum` type needs no changes (removed fields `owner`, `ownerId`, `assets` not used by app)

## 2. Random Asset Endpoint

- [x] 2.1 Change `GET /assets/random?count=N` to `POST /search/random` with body `{ size: N }` in `fetchRandomAsset`
- [x] 2.2 Add server-side `type: 'IMAGE'` filter in random search body when `skipVideos` is enabled
- [x] 2.3 Simplify retry logic: remove client-side video skip loop, let server filter handle it

## 3. Album Listing Endpoint

- [x] 3.1 Change `GET /albums?shared=true` to `GET /albums?isShared=true` in album fetch

## 4. Chronological Search Endpoint

- [x] 4.1 Replace `take`/`skip` with `page`/`size` in `POST /search/metadata` body
- [x] 4.2 Replace `assetType: ['IMAGE']` with `type: 'IMAGE'` (single value)
- [x] 4.3 Handle `nextPage` (string|null) instead of `hasNextPage` (boolean) in response

## 5. Verification

- [x] 5.1 Run `npm run type-check` and fix any TypeScript errors
- [x] 5.2 Run `npm run build` to confirm successful production build
