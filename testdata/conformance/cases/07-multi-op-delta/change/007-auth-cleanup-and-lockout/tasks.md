## Milestone 1: Remove legacy token login
**Goal** — delete the legacy token authentication path.
**Deliverables** — legacy token handler removed.
**Validation contract**
  - Legacy token login integration test is deleted
**Steps**
- [x] Remove legacy token handler

## Milestone 2: Tighten session expiry
**Goal** — reduce the inactivity window to 12 hours.
**Deliverables** — updated session TTL config.
**Validation contract**
  - Scenario "Session expires after inactivity" passes with the new 12h window
**Steps**
- [x] Update session TTL config

## Milestone 3: Account lockout
**Goal** — lock accounts out after repeated failed logins.
**Deliverables** — failed-login counter, lockout enforcement.
**Validation contract**
  - Scenario "Account locks out after five failed logins" passes
**Steps**
- [x] Implement failed-login counter
- [x] Implement lockout enforcement
