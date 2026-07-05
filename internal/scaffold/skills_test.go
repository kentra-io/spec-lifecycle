package scaffold

import (
	"path/filepath"
	"testing"

	"github.com/kentra-io/spec-lifecycle/internal/config"
)

func TestBuildBlockTargets(t *testing.T) {
	targets := BuildBlockTargets()
	if len(targets) != 2 {
		t.Fatalf("BuildBlockTargets() = %d targets, want 2", len(targets))
	}
	byRel := map[string]BlockTarget{}
	for _, tg := range targets {
		byRel[tg.Rel] = tg
	}
	if _, ok := byRel["CLAUDE.md"]; !ok {
		t.Error("missing CLAUDE.md target")
	}
	if _, ok := byRel["AGENTS.md"]; !ok {
		t.Error("missing AGENTS.md target")
	}
	for rel, tg := range byRel {
		if tg.Interior == "" {
			t.Errorf("%s: empty interior", rel)
		}
	}
}

func TestBuildSkillItems_AllFiveSkillsPerRuntime(t *testing.T) {
	items, err := BuildSkillItems([]string{config.RuntimeClaudeCode, config.RuntimeCursor, config.RuntimeCodex})
	if err != nil {
		t.Fatalf("BuildSkillItems: %v", err)
	}
	// 5 skills * 3 trees.
	if len(items) != 15 {
		t.Fatalf("BuildSkillItems() = %d items, want 15", len(items))
	}

	wantSkills := []string{"lifecycle-refine", "lifecycle-design", "lifecycle-plan", "lifecycle-bug", "lifecycle-archive"}
	wantDirs := map[string]string{
		config.RuntimeClaudeCode: ".claude",
		config.RuntimeCursor:     ".cursor",
		config.RuntimeCodex:      ".agents",
	}
	byRel := map[string][]byte{}
	for _, it := range items {
		byRel[it.Rel] = it.Content
	}
	for _, runtime := range []string{config.RuntimeClaudeCode, config.RuntimeCursor, config.RuntimeCodex} {
		for _, skill := range wantSkills {
			rel := filepath.ToSlash(filepath.Join(wantDirs[runtime], "skills", skill, "SKILL.md"))
			content, ok := byRel[rel]
			if !ok {
				t.Errorf("missing fanned-out skill item %s", rel)
				continue
			}
			if len(content) == 0 {
				t.Errorf("%s: empty content", rel)
			}
		}
	}
}

func TestBuildSkillItems_SingleRuntime(t *testing.T) {
	items, err := BuildSkillItems([]string{config.RuntimeCursor})
	if err != nil {
		t.Fatalf("BuildSkillItems: %v", err)
	}
	if len(items) != 5 {
		t.Fatalf("BuildSkillItems(cursor) = %d items, want 5", len(items))
	}
	for _, it := range items {
		if filepath.ToSlash(it.Rel)[:len(".cursor/")] != ".cursor/" {
			t.Errorf("item %s not under .cursor/", it.Rel)
		}
	}
}

func TestBuildSkillItems_EmptyRuntimesIsNoop(t *testing.T) {
	items, err := BuildSkillItems(nil)
	if err != nil {
		t.Fatalf("BuildSkillItems(nil): %v", err)
	}
	if items != nil {
		t.Fatalf("BuildSkillItems(nil) = %v, want nil", items)
	}
}

func TestSkillNamesAndContent(t *testing.T) {
	names, err := skillNames()
	if err != nil {
		t.Fatalf("skillNames: %v", err)
	}
	want := []string{"lifecycle-archive", "lifecycle-bug", "lifecycle-design", "lifecycle-plan", "lifecycle-refine"}
	if len(names) != len(want) {
		t.Fatalf("skillNames() = %v, want %v", names, want)
	}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("skillNames()[%d] = %q, want %q", i, n, want[i])
		}
		content, err := skillContent(n)
		if err != nil {
			t.Fatalf("skillContent(%q): %v", n, err)
		}
		if len(content) == 0 {
			t.Errorf("skillContent(%q): empty", n)
		}
	}
}
