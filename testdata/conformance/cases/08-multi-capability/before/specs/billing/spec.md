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
