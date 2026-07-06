package scaffold

import (
	"fmt"
	"io/fs"
	"path"
	"sort"

	root "github.com/kentra-io/spec-lifecycle"
	"github.com/kentra-io/spec-lifecycle/internal/config"
)

// Managed-block interiors (implementation-plan.md §2.9 step h). Unlike the
// constitution's single constitution.md, spec-lifecycle's living state is
// spread across many openspec/specs/<capability>/spec.md files plus the
// per-change gate records — there is no one file for CLAUDE.md to
// `@import` — so both targets get a short textual pointer (mirroring the
// constitution's own AGENTS.md interior shape) rather than an `@import`.
const (
	claudeInterior = "This project uses `lifecycle` (spec-lifecycle) for staged, gated planning. " +
		"Before touching related code, read the relevant `openspec/changes/<change>/` artifacts and " +
		"run `lifecycle status` to see gate state; approve gates only via `lifecycle approve`, never by hand-editing `approval-state.json`."
	agentsInterior = "This project uses `lifecycle` (spec-lifecycle) for staged, gated planning — see `openspec/`. " +
		"Run `lifecycle status` for gate state; approve gates only via `lifecycle approve`, never by hand-editing `approval-state.json`."
)

// BuildBlockTargets returns the managed pointer-block targets `lifecycle
// init` always writes: CLAUDE.md and AGENTS.md (implementation-plan.md §2.9
// step h). Unlike the constitution's agentInstructions.targets, this is not
// config-selectable in v1 — spec-lifecycle.md §9.1/§10 documents no such
// per-target toggle for the pointer blocks (only the skill fan-out trees,
// via `runtimes:`, are selectable — see BuildSkillItems).
func BuildBlockTargets() []BlockTarget {
	return []BlockTarget{
		{Rel: "CLAUDE.md", Interior: claudeInterior},
		{Rel: "AGENTS.md", Interior: agentsInterior},
	}
}

// runtimeTree maps a lifecycle.yml `runtimes:` entry (config.Runtime*) to
// the skill fan-out tree key (spec-lifecycle.md §9.2's mapping table):
// claude-code → .claude/skills/, cursor → .cursor/skills/, codex →
// .agents/skills/ (the cross-agent AGENTS.md convention).
func runtimeTree(runtime string) (string, bool) {
	switch runtime {
	case config.RuntimeClaudeCode:
		return SkillTreeClaude, true
	case config.RuntimeCursor:
		return SkillTreeCursor, true
	case config.RuntimeCodex:
		return SkillTreeAgents, true
	}
	return "", false
}

// BuildSkillItems returns the fanned-out SkillItem set for the given
// lifecycle.yml `runtimes:` list (implementation-plan.md §2.9 step g,
// spec-lifecycle.md §9.2): every embedded skills/<name>/SKILL.md, copied
// into each selected runtime's skill tree. An unrecognized runtime value is
// skipped (config.Load already refuses an unrecognized one at load time;
// this defensive skip only matters for a caller that bypassed validation).
func BuildSkillItems(runtimes []string) ([]SkillItem, error) {
	if len(runtimes) == 0 {
		return nil, nil
	}
	names, err := skillNames()
	if err != nil {
		return nil, err
	}
	var items []SkillItem
	for _, r := range runtimes {
		tree, ok := runtimeTree(r)
		if !ok {
			continue
		}
		dir, ok := TreeDir(tree)
		if !ok {
			continue
		}
		for _, name := range names {
			content, err := skillContent(name)
			if err != nil {
				return nil, err
			}
			items = append(items, SkillItem{
				Rel:     path.Join(dir, "skills", name, "SKILL.md"),
				Content: content,
			})
		}
	}
	return items, nil
}

// skillNames returns the sorted list of embedded skill directory names
// (root.SkillsFS's skills/<name>/ entries).
func skillNames() ([]string, error) {
	entries, err := fs.ReadDir(root.SkillsFS, "skills")
	if err != nil {
		return nil, fmt.Errorf("scaffold: reading embedded skills/: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// skillContent reads the embedded skills/<name>/SKILL.md body.
func skillContent(name string) ([]byte, error) {
	data, err := root.SkillsFS.ReadFile(path.Join("skills", name, "SKILL.md"))
	if err != nil {
		return nil, fmt.Errorf("scaffold: reading embedded skills/%s/SKILL.md: %w", name, err)
	}
	return data, nil
}
