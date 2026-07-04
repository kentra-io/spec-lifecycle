# Auth cleanup and lockout

## Why
Consolidate three overdue auth changes into one: drop the legacy token
path, tighten session expiry, and add brute-force lockout.

## What Changes
- **auth:** REMOVED - drop the legacy token login requirement.
- **auth:** MODIFIED - tighten session expiry to 12 hours.
- **auth:** ADDED - lock an account out after repeated failed logins.

## Impact
- Removes, modifies, and adds requirements within the existing `auth` capability.
