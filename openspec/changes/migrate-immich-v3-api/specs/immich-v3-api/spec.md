## ADDED Requirements

### Requirement: Random asset search via POST /search/random
The system SHALL fetch random assets using `POST /search/random` with a JSON body containing `size` (number of assets). The endpoint SHALL support optional `type` filtering (`IMAGE`, `VIDEO`, `AUDIO`, `OTHER`). When `skipVideos` is enabled, the system SHALL pass `{ type: 'IMAGE' }` to only fetch images. When `skipVideos` is disabled, the system SHALL omit the `type` field to include all asset types.

#### Scenario: Fetch random assets
- **WHEN** the app requests random assets
- **THEN** it SHALL call `POST /search/random` with `{ size: N }`
- **THEN** the response SHALL be `AssetResponseDto[]`

#### Scenario: Fetch only images from random search
- **WHEN** the user has "skip videos" enabled
- **THEN** the request body SHALL include `type: 'IMAGE'`

#### Scenario: Fetch all types from random search
- **WHEN** the user has "skip videos" disabled
- **THEN** the request body SHALL omit the `type` field

### Requirement: Album listing with isShared param
The system SHALL list shared albums using `GET /albums?isShared=true` (was `shared=true`).

#### Scenario: List shared albums
- **WHEN** the app requests shared albums
- **THEN** it SHALL call `GET /albums?isShared=true`

### Requirement: Metadata search with page/size pagination
The system SHALL perform chronological metadata search using `POST /search/metadata` with `page` and `size` fields (was `skip`/`take`). The `type` field SHALL be a single value (was array). The response SHALL use `nextPage` (string|null) instead of `hasNextPage` (boolean) to indicate more results.

#### Scenario: Search next chronological asset
- **WHEN** the app requests the next chronological asset
- **THEN** it SHALL send `{ page: 1, size: 1, order: 'desc', type: 'IMAGE' }` (or omit type for all)
- **THEN** the response SHALL be checked for `nextPage !== null` to determine if more results exist

### Requirement: ImmichAsset type with v3 fields
The `ImmichAsset` TypeScript type SHALL reflect the v3 API response: remove `deviceAssetId` and `deviceId`; change `duration` from `string` to `number | null`; add optional fields `visibility`, `checksum`, `isEdited`, `stack`, `people`, `tags`, `duplicateId`.

#### Scenario: v3 asset type is valid
- **WHEN** the app receives an asset from the v3 API
- **THEN** the response SHALL conform to the updated `ImmichAsset` type without referencing removed fields

## REMOVED Requirements

### Requirement: GET /assets/random endpoint usage
**Reason**: Replaced by `POST /search/random` in Immich v3
**Migration**: Use `POST /search/random` with body `{ size, type? }`

### Requirement: shared query param on GET /albums
**Reason**: Renamed to `isShared` in Immich v3
**Migration**: Use `GET /albums?isShared=true`

### Requirement: take/skip pagination on POST /search/metadata
**Reason**: Replaced by page/size in Immich v3
**Migration**: Use `page` and `size` fields instead of `take`/`skip`

### Requirement: hasNextPage response field
**Reason**: Replaced by `nextPage` (string|null) in Immich v3
**Migration**: Check `response.assets.nextPage !== null` instead of `response.assets.hasNextPage === true`

### Requirement: assetType array on POST /search/metadata
**Reason**: Replaced by single-value `type` field in Immich v3
**Migration**: Use `type: 'IMAGE'` instead of `assetType: ['IMAGE']`
