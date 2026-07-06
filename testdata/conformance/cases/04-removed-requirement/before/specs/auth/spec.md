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

### Requirement: Legacy token login
The system SHALL allow a registered user to authenticate with a long-lived legacy API token.

#### Scenario: Successful login with a legacy token
- **GIVEN** a registered user with a valid legacy token
- **WHEN** the user submits the token
- **THEN** the system SHALL grant an authenticated session

### Requirement: Session expiry
The system SHALL expire an authenticated session after 24 hours of inactivity.

#### Scenario: Session expires after inactivity
- **GIVEN** an authenticated session with no activity for 24 hours
- **WHEN** the user makes a new request
- **THEN** the system SHALL require re-authentication
