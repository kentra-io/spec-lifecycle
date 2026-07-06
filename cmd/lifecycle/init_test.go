package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/kentra-io/spec-lifecycle/internal/config"
)

func TestNormalizeRuntimes(t *testing.T) {
	tests := []struct {
		name    string
		given   []string
		want    []string
		wantErr string
	}{
		{name: "empty stays nil", given: nil, want: nil},
		{
			name:  "valid values kept in order",
			given: []string{config.RuntimeCodex, config.RuntimeClaudeCode},
			want:  []string{config.RuntimeCodex, config.RuntimeClaudeCode},
		},
		{
			name:  "duplicates collapsed, first occurrence order kept",
			given: []string{config.RuntimeCursor, config.RuntimeCursor, config.RuntimeClaudeCode},
			want:  []string{config.RuntimeCursor, config.RuntimeClaudeCode},
		},
		{
			name:    "unknown value rejected",
			given:   []string{"vscode"},
			wantErr: `unknown value "vscode"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeRuntimes(tt.given)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("normalizeRuntimes(%v) err = %v, want it to contain %q", tt.given, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeRuntimes(%v): %v", tt.given, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("normalizeRuntimes(%v) = %v, want %v", tt.given, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("normalizeRuntimes(%v) = %v, want %v", tt.given, got, tt.want)
				}
			}
		})
	}
}

func TestNormalizeRuntimes_ErrorCarriesCouldNotRunExitCode(t *testing.T) {
	_, err := normalizeRuntimes([]string{"bogus"})
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := exitCode(err); got != initExitCouldNotRun {
		t.Errorf("exitCode(err) = %d, want %d", got, initExitCouldNotRun)
	}
}

func TestInteractiveConfirm(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "y accepts", input: "y\n", want: true},
		{name: "yes accepts", input: "yes\n", want: true},
		{name: "YES accepts case-insensitively", input: "YES\n", want: true},
		{name: "blank line declines", input: "\n", want: false},
		{name: "n declines", input: "n\n", want: false},
		{name: "anything else declines", input: "sure why not\n", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = r.Close() })
			if _, err := w.WriteString(tt.input); err != nil {
				t.Fatal(err)
			}
			_ = w.Close()

			var out bytes.Buffer
			confirm := interactiveConfirm(r, &out)
			got, err := confirm("overwrite?")
			if err != nil {
				t.Fatalf("confirm: %v", err)
			}
			if got != tt.want {
				t.Errorf("confirm(%q) = %v, want %v", tt.input, got, tt.want)
			}
			if !strings.Contains(out.String(), "overwrite? [y/N] ") {
				t.Errorf("prompt not written to out: %q", out.String())
			}
		})
	}
}
