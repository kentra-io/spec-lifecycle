// Package plandag is spec-lifecycle's CLI process-boundary to the sibling
// milestoned-plan-dag primitive (design D6, change 007 plan-integration).
//
// The plan stage's gate delegates plan validation to `milestoned-plan-dag
// validate <plan.yaml>` and the archive step-completion gate reads
// milestone done-states from `milestoned-plan-dag resolve <plan.yaml>`.
// There is deliberately NO Go import of milestoned-plan-dag: the two
// binaries stay independently releasable, coupled only by a CLI/YAML
// contract, mirroring the same standalone-primitive posture the
// internal/constitution seam already uses.
//
// This package intentionally parallels internal/constitution: Locate
// (override arg > EnvBinOverride > PATH), Version/CheckVersion for the
// warn-not-fail preflight, and a documented binary the harness installs
// alongside `constitution`.
package plandag
