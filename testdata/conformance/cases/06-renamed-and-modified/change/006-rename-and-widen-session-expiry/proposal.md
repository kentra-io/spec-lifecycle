# Rename and widen session expiry

## Why
"Session expiry" should be named for the policy it encodes ("Session
inactivity timeout"), and the window itself needs widening to 72 hours in
the same change.

## What Changes
- **auth:** RENAMED - "Session expiry" becomes "Session inactivity timeout".
- **auth:** MODIFIED - widen the (renamed) requirement's inactivity window to 72 hours.

## Impact
- Renames and modifies one requirement in the existing `auth` capability.
