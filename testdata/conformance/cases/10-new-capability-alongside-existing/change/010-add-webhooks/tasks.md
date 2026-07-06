## Milestone 1: Webhook delivery
**Goal** — deliver signed webhook events to subscribed endpoints.
**Deliverables** — event dispatcher, HMAC signing.
**Validation contract**
  - Scenario "Subscribed endpoint receives a signed event" passes
  - Scenario "Signing key rotation invalidates old signatures" passes
**Steps**
- [x] Implement event dispatcher
- [x] Implement HMAC signing and key rotation
