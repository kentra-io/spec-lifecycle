# auth Specification

## Purpose
Provide user authentication for the product.
## Requirements
### Requirement: Password Login
The system SHALL allow a user to authenticate with a username and password.

#### Scenario: Valid credentials
- **WHEN** a user submits a correct username and password
- **THEN** the system SHALL grant access

### Requirement: Session Expiry
Sessions SHALL expire after 30 minutes of inactivity.

