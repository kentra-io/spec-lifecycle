package schema

import "testing"

func TestLoad(t *testing.T) {
	def, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if def.Name != Name {
		t.Errorf("def.Name = %q, want %q", def.Name, Name)
	}
	if len(def.Artifacts) != 3 {
		t.Fatalf("len(def.Artifacts) = %d, want 3", len(def.Artifacts))
	}
}

func TestGenerates(t *testing.T) {
	def, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	tests := []struct {
		id   string
		want string
	}{
		{"proposal", "proposal.md"},
		{"specs", "specs/**/spec.md"},
		{"design", "design.md"},
		{"tasks", ""}, // retired (change 007 M6): no tasks artifact
		{"nonexistent", ""},
	}
	for _, tt := range tests {
		if got := def.Generates(tt.id); got != tt.want {
			t.Errorf("Generates(%q) = %q, want %q", tt.id, got, tt.want)
		}
	}
}
