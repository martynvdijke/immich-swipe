## ADDED Requirements

### Requirement: Person listing
The system SHALL fetch the person list from Immich and present it in a picker showing each person's name and, when available, their face thumbnail.

#### Scenario: Person picker opens
- **WHEN** the user opens the person picker
- **THEN** the system SHALL display people fetched from `GET /people`, each with name and face thumbnail when available

#### Scenario: No people available
- **WHEN** the person list is empty (face recognition disabled or no faces detected)
- **THEN** the picker SHALL show a message explaining that no people were found and normal library review SHALL remain available

### Requirement: Person-scoped feed
Selecting a person SHALL restrict the review feed to assets depicting that person, in both random and chronological order, combined with the skip-videos setting.

#### Scenario: Review a person's photos chronologically
- **WHEN** the user selects a person and the review order is chronological
- **THEN** `POST /search/metadata` SHALL include the person filter and only assets depicting that person SHALL be presented

#### Scenario: Review a person's photos randomly
- **WHEN** the user selects a person and the review order is random
- **THEN** only assets depicting that person SHALL be presented

#### Scenario: Skip videos applies within person scope
- **WHEN** skip-videos is enabled while a person scope is active
- **THEN** the person filter SHALL be combined with the image-only filter

### Requirement: Person identity display
While a person scope is active, the system SHALL display the selected person's name in the review UI.

#### Scenario: Card shows active person
- **WHEN** the user reviews within a person scope
- **THEN** the card or header SHALL show the selected person's name

### Requirement: Person scope persistence
The selected person SHALL be persisted per server:user.

#### Scenario: Person scope survives reload
- **WHEN** the user selects a person and reloads the page
- **THEN** the app SHALL restore the same person scope

### Requirement: Clearing person scope
The user SHALL be able to clear the person scope and return to the previous (or library) feed.

#### Scenario: Back to library
- **WHEN** the user clears the person scope
- **THEN** the feed SHALL revert to whole-library behavior

### Requirement: Person scope change resets the flow
Changing the selected person SHALL reset the in-memory review flow.

#### Scenario: Switch person mid-session
- **WHEN** the user selects a different person while reviewing
- **THEN** the pending queue, chronological page, and undo history SHALL be cleared and the feed SHALL reload from the first asset of the new person
