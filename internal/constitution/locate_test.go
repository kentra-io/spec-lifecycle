package constitution

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocateOverride(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "constitution")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Locate(bin)
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if got != bin {
		t.Errorf("Locate = %q, want %q", got, bin)
	}
}

func TestLocateOverrideMissing(t *testing.T) {
	if _, err := Locate(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Fatal("Locate: error = nil, want an error for a missing override path")
	}
}

func TestLocateEnvOverride(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "constitution")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvBinOverride, bin)
	got, err := Locate("")
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if got != bin {
		t.Errorf("Locate = %q, want %q", got, bin)
	}
}

func TestLocateNotFoundOnPATH(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := Locate(""); err == nil {
		t.Fatal("Locate: error = nil, want an error when \"constitution\" is not on PATH")
	}
}
