## ADDED Requirements

### Requirement: Settings view accessible from the app

The system SHALL provide a `/settings` route guarded by the same authentication as the home view, reachable from a settings entry in the app header. The view SHALL group configuration into an Umami section and an OpenTelemetry section.

#### Scenario: Authenticated user opens settings
- **WHEN** an authenticated user navigates to `/settings` or clicks the settings entry in the header
- **THEN** the Settings view renders with Umami and OpenTelemetry sections showing the current persisted configuration

#### Scenario: Unauthenticated user opens settings
- **WHEN** a user without an active session requests `/settings`
- **THEN** the router redirects them to the login flow

### Requirement: Settings persist across sessions per server and user

The system SHALL persist observability settings in `localStorage` under a key scoped to the active server and user, mirroring the existing preferences store. Settings SHALL be restored when the page loads and SHALL survive browser restarts.

#### Scenario: Settings survive reload
- **WHEN** the user saves observability settings and reloads the page
- **THEN** the Settings view shows the same saved values and the configured integrations are active

#### Scenario: Different server or user has independent settings
- **WHEN** the user switches to a different server or user account
- **THEN** the observability settings of the previous server/user are not applied; the new server/user starts from its own saved settings or defaults

### Requirement: Umami settings validation

The system SHALL require, when Umami is enabled: a non-empty website ID and an http(s) server URL. The server URL SHALL be normalized to remove trailing slashes before use. Invalid values SHALL be rejected with inline error text and the previously persisted configuration SHALL remain active.

#### Scenario: Invalid Umami URL rejected
- **WHEN** the user enables Umami and enters a server URL that is not a valid http(s) URL
- **THEN** the form shows an inline error, nothing is persisted, and the previously saved configuration stays active

#### Scenario: Empty website ID rejected
- **WHEN** the user enables Umami without entering a website ID
- **THEN** the form shows an inline error and saving is blocked

### Requirement: OpenTelemetry settings validation

The system SHALL require, when OpenTelemetry is enabled: a valid http(s) OTLP endpoint URL. The sampling ratio SHALL be a number between 0 and 100 (percent). Invalid values SHALL be rejected with inline error text and the previously persisted configuration SHALL remain active.

#### Scenario: Invalid OTLP endpoint rejected
- **WHEN** the user enables OpenTelemetry and enters an endpoint that is not a valid http(s) URL
- **THEN** the form shows an inline error, nothing is persisted, and the previously saved configuration stays active

#### Scenario: Sampling ratio out of range rejected
- **WHEN** the user enters a sampling ratio outside 0–100
- **THEN** the form shows an inline error and saving is blocked

### Requirement: Settings apply immediately

The system SHALL apply saved settings immediately without a page reload: enabling an integration loads it, disabling or reconfiguring it tears down or re-initializes it.

#### Scenario: Enabling Umami applies without reload
- **WHEN** the user saves an enabled Umami configuration
- **THEN** the Umami script is injected and page-view tracking starts without reloading the page

#### Scenario: Disabling an integration stops it without reload
- **WHEN** the user disables Umami or OpenTelemetry and saves
- **THEN** no further tracking or tracing network requests are sent from that browser session

### Requirement: Disabled or misconfigured integrations degrade gracefully

The system SHALL leave the application fully functional when observability is disabled or its configuration is invalid: no tracking code SHALL load, no network requests to tracking endpoints SHALL be made, no errors SHALL be surfaced to the user, and review functionality SHALL be unaffected.

#### Scenario: Default state is observability off
- **WHEN** a user opens the app without ever configuring observability
- **THEN** no Umami script or OTel SDK is loaded and no tracking/tracing requests are sent

#### Scenario: Tracking script failure is silent
- **WHEN** the configured Umami script fails to load
- **THEN** the app continues to work normally, no error is shown to the user, and the failure is only visible as a status hint in the Settings view
