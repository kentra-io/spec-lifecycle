## ADDED Requirements
### Requirement: Trial invoicing exemption
The system SHALL NOT generate an invoice for a subscription while its account is flagged as trial.

#### Scenario: Trial subscriptions are not invoiced
- **GIVEN** an active subscription whose account is flagged as trial
- **WHEN** the 30-day billing cycle elapses
- **THEN** the system SHALL NOT generate an invoice
