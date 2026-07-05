// Package lifecycle is spec-lifecycle's module-root library package. Its
// only job is embedding the repo's single-source Layer-2 skill bundles
// (skills/) so `lifecycle init` (internal/scaffold) can fan them out into a
// target repo's agent-skill trees (implementation-plan.md §2.9 step g,
// §3, §9.2). The same skills/ directory is also directly consumable by
// out-of-band tooling (e.g. `npx skills add kentra-io/spec-lifecycle`);
// embedding it makes the CLI self-contained — the fanned-out copies are
// real files, never symlinks. Mirrors the sibling adr-sourced-constitution
// primitive's own root embed.go exactly (implementation-plan.md §2.12).
//
// The embed directive must live in a package whose directory is an
// ancestor of skills/. skills/ sits at the repo root (the layout the plan
// pins and the path the npx tooling expects), so the embedding package is
// this root library package rather than an internal one;
// internal/scaffold consumes SkillsFS through it.
//
// M6 ships the five bundles as MINIMAL VALID skills — correct SKILL.md
// frontmatter and a body that points at the right verbs/docs, with an
// explicit note that the real prompt-engineered bodies land in M7
// (implementation-plan.md §8's milestone map). Do not mistake these stubs
// for the finished skill content.
package lifecycle

import "embed"

// SkillsFS holds the skills/ tree: one skills/<name>/SKILL.md per skill.
//
//go:embed skills
var SkillsFS embed.FS
