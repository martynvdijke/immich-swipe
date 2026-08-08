## ADDED Requirements

### Requirement: Proxy-based keep/delete counting

The system SHALL observe Immich requests flowing through its reverse proxy and update a per-(server URL, user name) stats store according to the following rules:
- `DELETE /api/assets` with body `{"ids": [...], "force": false}` (or absent `force`) SHALL increment the deleted count by the number of asset IDs
- `POST /api/trash/restore/assets` with body `{"ids": [...]}` SHALL decrement the deleted count by the number of asset IDs
- `PUT /api/albums/<albumId>/assets` with body `{"ids": [...]}` SHALL increment the kept count by the number of asset IDs
- `PUT /api/assets/<assetId>` with body `{"isFavorite": true}` SHALL increment the kept count by one

All other requests (GET/HEAD, other paths, `force: true`, non-parseable bodies) SHALL be forwarded to Immich unchanged and SHALL NOT alter any counters. The request body SHALL be fully available to the upstream after counting (the proxy SHALL rewind the body before forwarding).

#### Scenario: Delete request increments deleted count
- **WHEN** an authenticated session issues `DELETE /api/assets` with body `{"ids": ["a1","a2"], "force": false}`
- **THEN** the stats entry for that server and user has its deleted count increased by 2 and the upstream receives the identical request body

#### Scenario: Restore decrements deleted count
- **WHEN** a session issues `POST /api/trash/restore/assets` with body `{"ids": ["a1"]}`
- **THEN** the deleted count for that server and user is decreased by 1

#### Scenario: Keep-to-album increments kept count
- **WHEN** a session issues `PUT /api/albums/album123/assets` with body `{"ids": ["a1","a2","a3"]}`
- **THEN** the kept count for that server and user is increased by 3

#### Scenario: Favorite-as-keep increments kept count
- **WHEN** a session issues `PUT /api/assets/a1` with body `{"isFavorite": true}`
- **THEN** the kept count for that server and user is increased by 1

#### Scenario: Non-countable requests are ignored
- **WHEN** a session issues a GET request, a DELETE with `{"force": true}`, or a request with malformed JSON
- **THEN** no counters are changed and the request is proxied to Immich without modification

### Requirement: Success-gated counter updates

The system SHALL apply counter changes only when the proxied Immich request completes with a 2xx status. If the upstream responds with an error status or fails entirely, the counters SHALL NOT change.

#### Scenario: Successful delete counts
- **WHEN** the proxied `DELETE /api/assets` returns 204 from Immich
- **THEN** the deleted count is incremented by the number of IDs

#### Scenario: Failed delete does not count
- **WHEN** the proxied `DELETE /api/assets` returns 401 or 500 from Immich
- **THEN** no counters change

#### Scenario: A single request is not double-counted
- **WHEN** a countable request completes successfully
- **THEN** its effect is applied exactly once, regardless of retries or follow-up processing

### Requirement: Public stats polling endpoint

The system SHALL expose `GET /api/trmnl/stats` without authentication, registered as a first-class route that SHALL NOT be proxied to Immich. The response SHALL be JSON with `keptCount`, `deletedCount`, `totalCount` (sum of both), `serverUrl`, `updatedAt` (RFC3339 timestamp of the last counter change, or the server start time if none), and a `users` array where each entry has `userName`, `keptCount`, and `deletedCount`. Totals SHALL be the sum across all tracked server/user entries. When no stats exist, the endpoint SHALL return zeros, an empty `users` array, and a valid `updatedAt`.

#### Scenario: Stats exist for one user
- **WHEN** one user has kept 5 and deleted 2, and `GET /api/trmnl/stats` is called
- **THEN** the response contains `keptCount: 5`, `deletedCount: 2`, `totalCount: 7`, and a `users` array with one entry for that user

#### Scenario: Stats are aggregated across users
- **WHEN** user A has kept 3 and user B has deleted 4, and `GET /api/trmnl/stats` is called
- **THEN** the response totals are `keptCount: 3`, `deletedCount: 4`, `totalCount: 7`, and `users` contains both users with their individual counts

#### Scenario: No stats recorded yet
- **WHEN** `GET /api/trmnl/stats` is called before any countable request
- **THEN** the response has zero counts, an empty `users` array, and a valid RFC3339 `updatedAt`

#### Scenario: Endpoint is not proxied
- **WHEN** `GET /api/trmnl/stats` is called
- **THEN** the request is served by the Go backend itself and never forwarded to an Immich server

### Requirement: Optional stats persistence

The system SHALL persist the stats store to the file path configured by the `TRMNL_STATS_FILE` environment variable. When the variable is set, the store SHALL load existing counts from the file at startup and SHALL write the full store atomically (write to a temporary file, then rename) after each counter change. When the variable is unset or empty, the store SHALL remain in-memory only and counters SHALL reset when the process restarts. If the file cannot be read or written, the system SHALL log a warning and continue with in-memory operation.

#### Scenario: Persistence enabled
- **WHEN** `TRMNL_STATS_FILE=/data/stats.json` is set and a delete is counted
- **THEN** the updated store is written atomically to `/data/stats.json`, and a subsequent process restart loads the counts back

#### Scenario: Persistence disabled
- **WHEN** `TRMNL_STATS_FILE` is unset and the process restarts
- **THEN** the stats store starts empty (all counters zero)

#### Scenario: Unwritable stats file
- **WHEN** `TRMNL_STATS_FILE` points to a path that cannot be created
- **THEN** the system logs a warning and continues serving with in-memory counters

### Requirement: Trmnl e-ink plugin templates

The system SHALL ship a `trmnl/` plugin directory containing `settings.yml` and four Liquid templates (`full.liquid`, `half_horizontal.liquid`, `half_vertical.liquid`, `quadrant.liquid`). The templates SHALL render the stats returned by the polling endpoint (`IDX_0.keptCount`, `IDX_0.deletedCount`, `IDX_0.totalCount`, and optionally the `users` breakdown) using the standard Trmnl e-ink CSS classes (`value--large`, `value--medium`, `label`, `title_bar`), with a keep/delete summary and the standard `title_bar` footer. `settings.yml` SHALL declare strategy `polling`, a `polling_url` placeholder pointing to `/api/trmnl/stats`, an e-ink-appropriate `refresh_interval`, and a custom field for the user's instance URL.

#### Scenario: Full template renders counters
- **WHEN** the device renders `full.liquid` with `IDX_0` containing kept and deleted counts
- **THEN** the template displays both counts and their total using large e-ink value styling plus a `title_bar`

#### Scenario: All four layouts are shipped
- **WHEN** a user inspects the `trmnl/` directory
- **THEN** it contains `settings.yml` and all four layout templates, matching the sandwitches plugin layout so any layout zone can be configured on the device
