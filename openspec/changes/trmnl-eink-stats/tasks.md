## 1. Stats store

- [x] 1.1 Add a `StatsEntry` struct (`UserName`, `ServerURL`, `KeptCount`, `DeletedCount`) and a mutex-guarded `StatsStore` (map keyed by `serverURL|userName`) to the Go server (new file `server/stats.go`), with `IncrementKept`/`IncrementDeleted` (accepting signed deltas), `Snapshot()` returning totals + per-user entries, and `Reset()`
- [x] 1.2 Implement `LoadFromFile`/`SaveToFile` using atomic write (temp file + `os.Rename`), keyed off the `TRMNL_STATS_FILE` env var; on any file error, log a warning and continue in-memory only
- [x] 1.3 Wire `TRMNL_STATS_FILE` into `loadConfig()` in `server/main.go` and add it to the `Config` struct; load the store at startup in `main()` / `NewServer` when the path is set
- [x] 1.4 Add unit tests in `server/stats_test.go`: increment/decrement, per-user isolation, snapshot aggregation, atomic persistence round-trip, unwritable file degrades gracefully

## 2. Proxy counting

- [x] 2.1 In `proxyHandler` (`server/main.go`), add a body-inspection step for countable methods: read the request body for `DELETE /api/assets`, `POST /api/trash/restore/assets`, `PUT /api/albums/<id>/assets`, and `PUT /api/assets/<id>`; parse JSON; rewind the body with `io.NopCloser(bytes.NewReader(body))` so upstream receives it unchanged
- [x] 2.2 Implement the counting rules from design D4: trash delete → deleted += len(ids); restore → deleted -= len(ids); album add → kept += len(ids); favorite `{"isFavorite": true}` → kept += 1; ignore everything else (including `force: true` and malformed bodies)
- [x] 2.3 Attach parsed counts to the request context and apply them in a `ModifyResponse` hook only when the upstream status is 2xx; clear the pending counts so a request is never double-counted
- [x] 2.4 Add unit tests in `server/main_test.go` (or `server/counting_test.go`): each rule increments/decrements correctly, non-countable requests are untouched, failed upstream status does not change counters, and the upstream request body is byte-identical after inspection

## 3. Public stats endpoint

- [x] 3.1 Add `case path == "/api/trmnl/stats"` to `ServeHTTP` **before** the `/api/` proxy catch-all, serving a GET-only handler with no auth middleware
- [x] 3.2 Implement the handler returning the D3 JSON shape: `keptCount`, `deletedCount`, `totalCount`, `serverUrl`, `updatedAt` (RFC3339 of last change or start time), and `users[]` with per-user counts; zero-valued response when the store is empty
- [x] 3.3 Add unit tests: aggregate totals across users, per-user entries, empty-store response, method-not-allowed for non-GET, and confirmation the request is not proxied (route precedence)

## 4. Trmnl plugin templates

- [x] 4.1 Create `trmnl/settings.yml`: strategy `polling`, `polling_url` placeholder `https://<your-immich-swipe-host>/api/trmnl/stats`, custom field for the instance URL, e-ink-friendly `refresh_interval`, name/description, modeled on `~/projects/sandwitches/trmnl/settings.yml`
- [x] 4.2 Create `trmnl/full.liquid` and `trmnl/half_horizontal.liquid`: render `IDX_0.keptCount`/`IDX_0.deletedCount`/`IDX_0.totalCount` as large values, a keep/delete summary row, optional `users` loop, and the `title_bar` footer
- [x] 4.3 Create `trmnl/half_vertical.liquid` and `trmnl/quadrant.liquid`: compact variants of the same counters suitable for smaller layout zones
- [x] 4.4 Validate the Liquid templates against Trmnl CSS class conventions used in the sandwitches plugin (`value--large`, `label`, `title_bar`, layout grid classes)

## 5. Docs and verification

- [x] 5.1 Update `README.md`: document the Trmnl e-ink plugin (install on device, set `polling_url` to `<host>/api/trmnl/stats`), the `TRMNL_STATS_FILE` env var with a docker-compose bind-mount example, the counted-vs-uncounted actions (plain keeps not counted), and the single-instance assumption
- [x] 5.2 Add `TRMNL_STATS_FILE` to `env.example` and to the `environment:` block in `docker-compose.yml`
- [x] 5.3 Run `cd server && go test ./...` and `go vet ./...`; fix any failures; confirm the proxy still forwards all request bodies byte-identically
