# Trial signup

## Why
Offer a 14-day free trial: signup must create an unauthenticated-to-trial
account path, and billing must skip invoicing during the trial window.

## What Changes
- **auth:** ADDED - trial account signup requirement.
- **billing:** ADDED - trial invoicing exemption requirement.

## Impact
- New requirement in `auth`.
- New requirement in `billing`.
