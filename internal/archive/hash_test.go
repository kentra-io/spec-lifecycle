package archive

import (
	"path/filepath"
	"testing"
)

func TestHashFilePropagatesReadError(t *testing.T) {
	if _, err := hashFile(filepath.Join(t.TempDir(), "nope.txt")); err == nil {
		t.Fatal("hashFile: want error for a missing file, got nil")
	}
}

func TestManifestSHAPropagatesWalkError(t *testing.T) {
	if _, err := ManifestSHA(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("ManifestSHA: want error for a missing directory, got nil")
	}
}
