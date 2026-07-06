## ADDED Requirements
### Requirement: Trial account signup
The system SHALL allow a new user to create a trial account without providing a payment method.

#### Scenario: New user starts a trial without a payment method
- **GIVEN** a visitor with no existing account
- **WHEN** the visitor signs up for a trial without entering payment details
- **THEN** the system SHALL create an account flagged as trial
