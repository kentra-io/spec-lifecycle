## RENAMED Requirements
- FROM: `### Requirement: Session expiry`
- TO: `### Requirement: Session inactivity timeout`

## MODIFIED Requirements
### Requirement: Session inactivity timeout
The system SHALL expire an authenticated session after 72 hours of inactivity.

#### Scenario: Session expires after inactivity
- **GIVEN** an authenticated session with no activity for 72 hours
- **WHEN** the user makes a new request
- **THEN** the system SHALL require re-authentication
