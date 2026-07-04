# billing Specification

## Purpose
Subscription billing and invoicing.
## Requirements
### Requirement: Monthly invoicing
The system SHALL generate an invoice for each active subscription every 30 days.

#### Scenario: Invoice generated on schedule
- **GIVEN** an active subscription due for renewal
- **WHEN** the 30-day billing cycle elapses
- **THEN** the system SHALL generate an invoice

### Requirement: Trial invoicing exemption
The system SHALL NOT generate an invoice for a subscription while its account is flagged as trial.

#### Scenario: Trial subscriptions are not invoiced
- **GIVEN** an active subscription whose account is flagged as trial
- **WHEN** the 30-day billing cycle elapses
- **THEN** the system SHALL NOT generate an invoice

