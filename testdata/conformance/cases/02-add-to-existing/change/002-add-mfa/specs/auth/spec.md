## ADDED Requirements
### Requirement: Multi-factor authentication
The system SHALL require a second authentication factor when MFA is enabled for an account.

#### Scenario: Login requires second factor when MFA is enabled
- **GIVEN** a user account with MFA enabled
- **WHEN** the user submits a correct username and password
- **THEN** the system SHALL prompt for a second factor before granting a session
