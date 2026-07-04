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
	if len(def.Artifacts) != 4 {
		t.Fatalf("len(def.Artifacts) = %d, want 4", len(def.Artifacts))
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
		{"tasks", "tasks.md"},
		{"nonexistent", ""},
	}
	for _, tt := range tests {
		if got := def.Generates(tt.id); got != tt.want {
			t.Errorf("Generates(%q) = %q, want %q", tt.id, got, tt.want)
		}
	}
}
