# Rate limit the public API

## Why
Unbounded request rates from a single client have caused incidents; add a
token-bucket rate limiter with a documented response contract.

## What Changes
- **api:** ADDED - introduce the `api` capability with a rate-limiting requirement.

## Impact
- New capability: `api`.
