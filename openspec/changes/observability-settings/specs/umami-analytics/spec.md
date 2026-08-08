## ADDED Requirements

### Requirement: Umami script injection

When Umami is enabled, the system SHALL inject the Umami tracking script lazily into the document: `async`, `src` derived from the configured server URL (`<server>/script.js`), `data-website-id` from the settings, `data-auto-track="false"`, and `data-host-url` when configured. The script SHALL be injected at most once per configuration and SHALL NOT load when Umami is disabled.

#### Scenario: Script loads with correct attributes
- **WHEN** the user enables Umami with server URL `https://umami.example.com` and a website ID and the page is loaded
- **THEN** a script element with `src="https://umami.example.com/script.js"`, `data-website-id` set to the website ID, and `data-auto-track="false"` is present in the document

#### Scenario: Script not loaded when disabled
- **WHEN** Umami is disabled
- **THEN** no Umami script element is ever added to the document

#### Scenario: No duplicate injection on reconfigure
- **WHEN** the user changes Umami settings or re-navigates between views
- **THEN** the script is not injected more than once for the same configuration

### Requirement: Page-view tracking on route changes

The system SHALL track a page view through Umami on every router navigation after the script has loaded.

#### Scenario: Navigation between views tracked
- **WHEN** the user navigates from the home view to the settings view with Umami active
- **THEN** a page-view event for the new route is sent to the Umami server

#### Scenario: No page view before script readiness
- **WHEN** the Umami script has not finished loading or failed to load
- **THEN** no page-view event is sent and no error is raised

### Requirement: Custom review-event tracking

The system SHALL send Umami custom events for review actions with the following names and payloads:
- `swipe.keep` with `{ assetType }`
- `swipe.delete` with `{ assetType }`
- `swipe.undo` with `{ actionType }`
- `swipe.album_add` with `{ albumName, assetType }`
- `swipe.person_filter` with `{ personName }`

Events SHALL only be sent when the Umami script is loaded and functional.

#### Scenario: Keep action tracked
- **WHEN** the user keeps a photo while Umami is active
- **THEN** a `swipe.keep` event with the asset type is sent to the Umami server

#### Scenario: Person filter tracked
- **WHEN** the user selects a person in the person picker while Umami is active
- **THEN** a `swipe.person_filter` event with the person's name is sent

#### Scenario: No events when script missing
- **WHEN** a review action occurs but the Umami script never loaded
- **THEN** no custom event is sent and the review action completes normally
