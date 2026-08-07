## ADDED Requirements

### Requirement: Review scope selection
The system SHALL let the user constrain the review feed to one of: whole library (default), a single album, a date range (from/to), or favorites-only. The scope SHALL be selectable via a picker and the active scope SHALL be visible in the header.

#### Scenario: Select album scope
- **WHEN** the user opens the scope picker and selects an album
- **THEN** the review feed SHALL be restricted to assets in that album and the header SHALL show the album name

#### Scenario: Select date range scope
- **WHEN** the user opens the scope picker and sets a from/to date range
- **THEN** the review feed SHALL be restricted to assets captured within that range

#### Scenario: Select favorites scope
- **WHEN** the user selects the favorites-only scope
- **THEN** the review feed SHALL be restricted to assets marked as favorites

#### Scenario: Default scope is library
- **WHEN** no scope has been selected
- **THEN** the review feed SHALL behave as today (whole library)

### Requirement: Scope persistence
The active scope SHALL be persisted per server:user and restored on the next session.

#### Scenario: Scope survives reload
- **WHEN** the user selects an album scope and reloads the page
- **THEN** the app SHALL restore the same album scope and resume the feed within it

### Requirement: Feed applies the scope
The feed SHALL apply the active scope in both review orders.

#### Scenario: Chronological feed respects scope
- **WHEN** the review order is chronological and an album scope is active
- **THEN** `POST /search/metadata` SHALL include the album filter and only assets of that album SHALL be presented

#### Scenario: Random feed respects scope
- **WHEN** the review order is random and a non-library scope is active
- **THEN** only assets matching the scope SHALL be presented

#### Scenario: Skip videos applies within scope
- **WHEN** skip-videos is enabled and any scope is active
- **THEN** the scope filter SHALL be combined with the image-only filter

### Requirement: Scope change resets the flow
Changing the scope SHALL reset the in-memory review flow and load the first matching asset.

#### Scenario: Switch scope mid-session
- **WHEN** the user changes the scope while reviewing
- **THEN** the pending queue, chronological page, and undo history SHALL be cleared and the feed SHALL reload from the first asset of the new scope

### Requirement: Clearing the scope
The user SHALL be able to clear the active scope and return to a whole-library feed.

#### Scenario: Back to library
- **WHEN** the user clears the active scope
- **THEN** the feed SHALL revert to whole-library behavior without a page reload
