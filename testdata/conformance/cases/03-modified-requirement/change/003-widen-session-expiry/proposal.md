# Widen session expiry window

## Why
24 hours is too aggressive for users on shared internal tooling; support
teams asked for a longer inactivity window with a remember-me option.

## What Changes
- **auth:** MODIFIED - widen the session expiry window and add a remember-me scenario.

## Impact
- Modifies the existing `auth` capability; no other capabilities affected.
