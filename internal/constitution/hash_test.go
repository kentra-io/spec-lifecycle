package constitution

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHash(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "constitution", "constitution.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# Constitution\n\nNo rules yet.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	hash, ok, err := Hash(root)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !ok {
		t.Fatal("Hash: ok = false, want true")
	}
	if hash == "" || hash[:7] != "sha256:" {
		t.Errorf("Hash = %q, want a sha256: prefix", hash)
	}

	hash2, ok2, err2 := Hash(root)
	if err2 != nil || !ok2 || hash2 != hash {
		t.Errorf("Hash is not deterministic: (%q,%v) vs (%q,%v)", hash, ok, hash2, ok2)
	}
}

func TestHashMissingConstitutionMD(t *testing.T) {
	root := t.TempDir()
	hash, ok, err := Hash(root)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if ok || hash != "" {
		t.Errorf("Hash on a project with no constitution.md = (%q, %v), want (\"\", false)", hash, ok)
	}
}

func TestHashesEqual(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"sha256:abc123", "sha256:ABC123", true},
		{"sha256:abc123", "sha256-abc123", true},
		{"sha256:abc123", "abc123", true},
		{"sha256:abc123", "sha256:def456", false},
	}
	for _, tt := range tests {
		if got := HashesEqual(tt.a, tt.b); got != tt.want {
			t.Errorf("HashesEqual(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
