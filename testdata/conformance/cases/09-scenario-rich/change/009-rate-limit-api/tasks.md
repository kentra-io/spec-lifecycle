## Milestone 1: Rate limiter
**Goal** — enforce a per-client token-bucket rate limit on the public API.
**Deliverables** — rate limiter middleware, `429` response contract.
**Validation contract**
  - Scenario "Request within budget succeeds" passes
  - Scenario "Request over budget is rejected with 429" passes
  - Scenario "Rate limit resets after the window elapses" passes
  - Scenario "Malformed client identifier is rejected before rate limiting" passes
**Steps**
- [x] Implement token-bucket middleware
- [x] Implement 429 response contract
- [x] Write scenario tests
