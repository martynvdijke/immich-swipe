## Context

`useImmich` builds search bodies for two feed paths (random and chronological). Preferences persist per server:user. The asset type already carries an optional `people` array. The person-scope feature overlaps with `review-feed-scoping` (change `scope-review-feed`) on the "scope" concept; this change is designed to compose with it: person scoping is an additional filter dimension that can combine with skip-videos and either review order.

## Goals / Non-Goals

**Goals:**
- Review all assets depicting a chosen person in random or chronological order.
- Show which person the current review session is about.
- Persist the selection per server:user.

**Non-Goals:**
- Managing people (renaming, merging, hiding) — that stays in Immich.
- Face-level review (reviewing individual face detections) — person-level only.
- Auto-tagging or bulk keep/delete per person (future work).

## Decisions

### Decision 1: Person is a filter dimension composed with the feed

Selected person → `personIds: [person.id]` merged into the metadata search body, exactly like album/date/favorites filters in `scope-review-feed`. Works with both orders and skip-videos.
Composition note: if `scope-review-feed` has landed, person scope is an additional, independently toggleable dimension (separate picker and badge). If it has not landed, this change implements its own minimal scope plumbing. Implementation order is handled at apply time; the spec only requires that a selected person restricts the feed.

### Decision 2: People source is GET /people

`GET /people` returns the person list (id, name, thumbnail, isHidden). Fetch once per session and cache (mirrors `albumsCache`). The picker shows name + thumbnail via the person thumbnail endpoint; if thumbnails are unavailable (e.g., recognition disabled), the picker degrades to a name-only list with an explanatory note.
Hidden people: show them but sort non-hidden first; hiding/renaming stays in Immich (out of scope).

### Decision 3: Identity display falls back to the selected person's name

Immich search results may or may not include a populated `people` array per asset depending on version. Instead of depending on that, the app shows the name it already knows — the selected person's name — as a badge on the card and in the header. `asset.people` is used only as best-effort enrichment (e.g., showing other people also present).
Rationale: guaranteed-correct display with zero dependency on the search-response shape.

### Decision 4: Random mode uses the scoped-metadata fallback

Same reasoning as `scope-review-feed` Decision 3: `/search/random` does not reliably support `personIds`. For person-scoped random, fetch a scoped page via `/search/metadata` and pick a candidate client-side, paging when consumed.

## Risks / Trade-offs

- [Risk] Face recognition disabled / no people on the server → `GET /people` returns empty; the picker shows a clear message and normal review is unaffected.
- [Risk] `asset.people` shape varies across versions → only best-effort enrichment; primary display is the known person name.
- [Risk] Very large person lists → single fetch, no pagination in v1; acceptable for typical libraries.
- [Risk] Person scope + feed scope interplay confusion → visually distinct badges; explicit clear action on each picker.

## Open Questions

- Exact response shape of `GET /people` on the deployed Immich version (field names: `thumbnailPath` vs `thumbnail`, `isHidden`) — verify during implementation.
- Whether `/search/random` accepts `personIds` on the deployed version (if yes, the random-mode fallback can be dropped).
