# auth Specification

## Purpose
Authentication and session management for the product.
## Requirements
### Requirement: Password login
The system SHALL allow a registered user to authenticate with a username and password.

#### Scenario: Successful login with correct password
- **GIVEN** a registered user with a known password
- **WHEN** the user submits the correct username and password
- **THEN** the system SHALL grant an authenticated session

### Requirement: Session expiry
The system SHALL expire an authenticated session after 12 hours of inactivity.

#### Scenario: Session expires after inactivity
- **GIVEN** an authenticated session with no activity for 12 hours
- **WHEN** the user makes a new request
- **THEN** the system SHALL require re-authentication

### Requirement: Account lockout
The system SHALL lock an account out of password login for 15 minutes after five consecutive failed attempts.

#### Scenario: Account locks out after five failed logins
- **GIVEN** a user account with four prior consecutive failed login attempts
- **WHEN** the user submits a fifth consecutive incorrect password
- **THEN** the system SHALL lock the account out of password login for 15 minutes

