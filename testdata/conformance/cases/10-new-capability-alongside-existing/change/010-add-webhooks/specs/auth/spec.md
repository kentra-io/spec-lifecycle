## ADDED Requirements
### Requirement: Webhook signing key rotation
The system SHALL allow an account owner to rotate the webhook signing key without invalidating in-flight deliveries signed under the previous key for 24 hours.

#### Scenario: Signing key rotation invalidates old signatures
- **GIVEN** an account that rotates its webhook signing key
- **WHEN** 24 hours elapse after rotation
- **THEN** the system SHALL reject deliveries signed under the previous key
