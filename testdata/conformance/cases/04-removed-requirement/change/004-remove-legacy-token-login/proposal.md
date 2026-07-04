# Remove legacy token login

## Why
Legacy API tokens are a long-standing security liability and are no longer
used by any active client.

## What Changes
- **auth:** REMOVED - drop the legacy token login requirement.

## Impact
- Removes a requirement from the existing `auth` capability.
