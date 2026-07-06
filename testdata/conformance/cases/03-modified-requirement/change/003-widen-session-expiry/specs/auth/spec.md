## MODIFIED Requirements
### Requirement: Session expiry
The system SHALL expire an authenticated session after 72 hours of inactivity, unless the user selected "remember me" at login.

#### Scenario: Session expires after inactivity
- **GIVEN** an authenticated session with no activity for 72 hours
- **WHEN** the user makes a new request
- **THEN** the system SHALL require re-authentication

#### Scenario: Remember-me extends session lifetime
- **GIVEN** a session created with "remember me" selected
- **WHEN** 72 hours of inactivity elapse
- **THEN** the system SHALL keep the session valid
