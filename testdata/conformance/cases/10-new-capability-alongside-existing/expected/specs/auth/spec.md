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

### Requirement: Webhook signing key rotation
The system SHALL allow an account owner to rotate the webhook signing key without invalidating in-flight deliveries signed under the previous key for 24 hours.

#### Scenario: Signing key rotation invalidates old signatures
- **GIVEN** an account that rotates its webhook signing key
- **WHEN** 24 hours elapse after rotation
- **THEN** the system SHALL reject deliveries signed under the previous key

