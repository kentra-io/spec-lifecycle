package main

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"

	root "github.com/kentra-io/spec-lifecycle"
	"github.com/kentra-io/spec-lifecycle/internal/approve"
	"github.com/kentra-io/spec-lifecycle/internal/validate"
)

// The drift check: every `lifecycle …` invocation printed in a shipped skill
// must name a subcommand this binary registers, a flag that subcommand
// accepts, and — where the value is an enumeration the CLI owns in Go — a
// value the CLI accepts. It reads the command tree, never a copy of it, so
// it cannot itself go stale.
//
// Deliberately one-directional: it catches a skill instructing something the
// CLI rejects, not a CLI flag no skill mentions. Nothing is executed, so
// approve/archive/init are checked without being run.

var inlineSpan = regexp.MustCompile("`([^`\n]+)`")

// enums are the flag values the CLI validates against an exported Go slice.
// Only those appear here: hardcoding an enumeration that lives as string
// literals inside a command would just be a second copy to drift.
func enums() map[string]map[string][]string {
	validateStages := make([]string, 0, len(validate.Stages))
	for _, s := range validate.Stages {
		validateStages = append(validateStages, string(s))
	}
	approveStages := make([]string, 0, len(approve.Stages))
	for _, s := range approve.Stages {
		approveStages = append(approveStages, string(s))
	}
	return map[string]map[string][]string{
		"validate": {"stage": validateStages},
		"approve":  {"stage": approveStages},
	}
}

// invocations returns every line of md whose first word is bin, looking
// inside both fenced code blocks and inline code spans — skills instruct
// commands in both, and D6.1 lived in an inline span.
func invocations(md, bin string) []string {
	var out []string
	add := func(s string) {
		if strings.HasPrefix(strings.TrimSpace(s), bin+" ") {
			out = append(out, strings.TrimSpace(s))
		}
	}
	inFence := false
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			add(line)
			continue
		}
		for _, m := range inlineSpan.FindAllStringSubmatch(line, -1) {
			add(m[1])
		}
	}
	return out
}

// placeholder reports whether tok is documentation shorthand rather than a
// literal value: <change>, <owner>/<repo>, or an alternation like text|json.
func placeholder(tok string) bool {
	return tok == "" || strings.HasPrefix(tok, "<") || strings.Contains(tok, "|")
}

func normalize(tok string) string { return strings.Trim(tok, "[]`\"'") }

func TestSkillsNameRealCommands(t *testing.T) {
	tree := map[string]*cli.Command{}
	for _, c := range rootCommand().Commands {
		tree[c.Name] = c
	}
	table := enums()

	err := fs.WalkDir(root.SkillsFS, "skills", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, "SKILL.md") {
			return err
		}
		data, err := root.SkillsFS.ReadFile(p)
		if err != nil {
			return err
		}
		for _, inv := range invocations(string(data), "lifecycle") {
			checkInvocation(t, tree, table, p, inv)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking embedded skills: %v", err)
	}
}

func checkInvocation(t *testing.T, tree map[string]*cli.Command, table map[string]map[string][]string, file, inv string) {
	t.Helper()
	fields := strings.Fields(inv)
	if len(fields) < 2 {
		return
	}
	name := normalize(fields[1])
	if strings.HasPrefix(name, "-") {
		return // `lifecycle --version`
	}
	cmd, ok := tree[name]
	if !ok {
		t.Errorf("%s: %q names subcommand %q, which the CLI does not register", file, inv, name)
		return
	}

	accepted := map[string]bool{}
	for _, f := range cmd.Flags {
		for _, n := range f.Names() {
			accepted[n] = true
		}
	}

	rest := fields[2:]
	for i := 0; i < len(rest); i++ {
		tok := normalize(rest[i])
		if !strings.HasPrefix(tok, "--") {
			continue
		}
		flag := strings.TrimPrefix(tok, "--")
		value := ""
		if eq := strings.Index(flag, "="); eq >= 0 {
			flag, value = flag[:eq], flag[eq+1:]
		} else if i+1 < len(rest) && !strings.HasPrefix(normalize(rest[i+1]), "-") {
			value = normalize(rest[i+1])
		}
		if !accepted[flag] {
			t.Errorf("%s: %q passes --%s, which `lifecycle %s` does not accept", file, inv, flag, name)
			continue
		}
		allowed, checked := table[name][flag]
		if !checked || placeholder(value) {
			continue
		}
		if !contains(allowed, value) {
			t.Errorf("%s: %q passes --%s %s, but `lifecycle %s` accepts only %v",
				file, inv, flag, value, name, allowed)
		}
	}
}

func contains(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}
