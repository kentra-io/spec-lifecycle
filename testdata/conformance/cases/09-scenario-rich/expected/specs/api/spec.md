# api Specification

## Purpose
TBD - created by archiving change 009-rate-limit-api. Update Purpose after archive.
## Requirements
### Requirement: Public API rate limiting
The system SHALL enforce a per-client token-bucket rate limit of 100 requests
per minute on the public API, and MUST reject requests that exceed the
budget with a structured error body.

Configuration (illustrative, not authoritative):

```yaml
rateLimit:
  algorithm: token-bucket
  capacity: 100
  refillPerMinute: 100
  keyedBy: client_id
```

Notes on precedence:
1. A per-route override, if configured, wins over the global default.
2. Otherwise the global default above applies.
3. A client with no identifier is rejected before rate limiting even runs
   (see the dedicated scenario below).

#### Scenario: Request within budget succeeds
- **GIVEN** a client that has made 42 requests in the current 1-minute window
- **WHEN** the client makes another request
- **THEN** the system SHALL process the request normally
  AND the system SHALL decrement the client's remaining budget by 1

#### Scenario: Request over budget is rejected with 429
- **GIVEN** a client that has already made 100 requests in the current window
- **WHEN** the client makes one more request
- **THEN** the system SHALL reject the request with HTTP 429
  AND the response body SHALL be:
  ```json
  {
    "error": "rate_limited",
    "retryAfterSeconds": 42
  }
  ```

#### Scenario: Rate limit resets after the window elapses
-    **GIVEN**   a client that exhausted its budget in the previous window
-  **WHEN** the current 1-minute window begins
- **THEN** the system SHALL reset the client's available budget to 100
    - this includes clients that were mid-backoff when the window rolled over
    - and clients that made zero requests in the prior window

#### Scenario: Malformed client identifier is rejected before rate limiting
- **GIVEN** a request whose client identifier fails validation
- **WHEN** the request reaches the rate limiter
- **THEN** the system SHALL reject the request with HTTP 400
  AND the system SHALL NOT consume any rate-limit budget

