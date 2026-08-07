## ADDED Requirements

### Requirement: Total reviewable count
The system SHALL determine the total number of assets matching the active feed configuration (review order, scope, skip-videos) via the metadata search `total` field.

#### Scenario: Library total
- **WHEN** the user reviews with no scope and skip-videos disabled
- **THEN** the system SHALL compute the total as the count of all assets in the library

#### Scenario: Skip-videos changes the total
- **WHEN** skip-videos is enabled
- **THEN** the total SHALL count only images

#### Scenario: Scope changes the total
- **WHEN** a scope (album, date range, favorites, person) is active
- **THEN** the total SHALL count only assets within that scope

### Requirement: Progress display
The system SHALL display review progress as a bar and/or counter showing reviewed assets against the total.

#### Scenario: Header shows progress
- **WHEN** the user is reviewing
- **THEN** the header SHALL show the number of reviewed assets and the total, with a progress bar capped at 100%

### Requirement: Progress accuracy
The reviewed count SHALL reflect the recorded keep/delete decisions for the active server:user.

#### Scenario: Keep and delete increase progress
- **WHEN** the user keeps or deletes an asset
- **THEN** the reviewed count SHALL increase by one

#### Scenario: Undo decreases progress
- **WHEN** the user undoes the last action
- **THEN** the reviewed count SHALL decrease by one

#### Scenario: Progress includes earlier sessions
- **WHEN** the user returns after a previous session
- **THEN** progress SHALL include assets already reviewed in the earlier session

### Requirement: Completion state
The system SHALL indicate when the active feed is fully reviewed.

#### Scenario: Library fully reviewed
- **WHEN** the reviewed count equals the total and the total is greater than zero
- **THEN** the system SHALL show a completion indication for the active feed

### Requirement: Progress refresh
Progress SHALL refresh when the feed configuration changes and periodically during review.

#### Scenario: Refresh on config change
- **WHEN** the user changes the review order, scope, or skip-videos
- **THEN** the total and progress SHALL be recomputed

#### Scenario: Periodic refresh
- **WHEN** the user performs review actions
- **THEN** the total SHALL be refreshed at least every 25 actions
