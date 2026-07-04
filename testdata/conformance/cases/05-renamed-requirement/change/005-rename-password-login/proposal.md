# Rename password login requirement

## Why
"Password login" is ambiguous once passkeys ship; rename it to be explicit
about the credential type without changing behavior.

## What Changes
- **auth:** RENAMED - "Password login" becomes "Username and password login".

## Impact
- Renames a requirement in the existing `auth` capability; no behavior change.
