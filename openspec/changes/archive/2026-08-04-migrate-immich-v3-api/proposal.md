## Why

Immich v3.0.0 introduced breaking API changes that break the app's core functionality (random asset fetch, chronological search, album listing, and type definitions). The app currently targets v2.x APIs. Without this migration, the app cannot connect to Immich v3.0.0+ instances.

## What Changes

- **BREAKING**: `GET /assets/random?count=N` → `POST /search/random` with body `{ size: N }`
- **BREAKING**: Rename query param `shared` → `isShared` on `GET /albums`
- **BREAKING**: Rename/restructure `assetType` (array) → `type` (single value) and `take`/`skip` → `page`/`size` on `POST /search/metadata`
- **BREAKING**: `hasNextPage` response field → `nextPage` (string|null token)
- **BREAKING**: `ImmichAsset` type changes: remove `deviceAssetId`, `deviceId`; change `duration` from `string` to `number | null`; add `visibility`, `checksum`, `isEdited`, `stack`, `people`, `tags`, `duplicateId`
- New: server-side video filtering via `POST /search/random` body `{ type: 'IMAGE' }` replaces client-side skip loop

## Capabilities

### New Capabilities

- `immich-v3-api`: Core API compatibility layer for Immich v3.0.0 endpoints, types, and DTOs

### Modified Capabilities

<!-- No existing specs to modify — openspec/specs/ is empty -->

## Impact

- `src/composables/useImmich.ts`: 3 API call sites + URL-building logic
- `src/types/immich.ts`: Type definitions for `ImmichAsset`
- TypeScript strict mode will catch unused/removed fields — clean them up
- No changes needed to `src/stores/auth.ts`, views, or components
