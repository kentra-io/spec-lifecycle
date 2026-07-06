# Add outbound webhooks

## Why
Customers want to subscribe to account events. This introduces a new
`webhooks` capability and a small supporting requirement in `auth` so
webhook signing keys can be rotated through the existing auth surface.

## What Changes
- **webhooks:** ADDED - introduce the `webhooks` capability with an event-delivery requirement.
- **auth:** ADDED - webhook signing key rotation requirement.

## Impact
- New capability: `webhooks`.
- New requirement in the existing `auth` capability.
