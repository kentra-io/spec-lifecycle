## ADDED Requirements
### Requirement: Signed event delivery
The system SHALL deliver account events to subscribed endpoints with an HMAC signature header.

#### Scenario: Subscribed endpoint receives a signed event
- **GIVEN** an endpoint subscribed to the `account.updated` event
- **WHEN** an account is updated
- **THEN** the system SHALL POST the event to the endpoint with a valid HMAC signature header
