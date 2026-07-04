package archive

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kentra-io/spec-lifecycle/internal/testutil"
)

// --- discoverCapabilities ---

func TestDiscoverCapabilitiesNoSpecsDirYieldsNilNotError(t *testing.T) {
	changeDir := t.TempDir()
	caps, err := discoverCapabilities(changeDir)
	if err != nil {
		t.Fatalf("discoverCapabilities: %v", err)
	}
	if caps != nil {
		t.Errorf("caps = %v, want nil (a delta-less bug has no specs/ at all)", caps)
	}
}

func TestDiscoverCapabilitiesPropagatesNonNotExistReadDirError(t *testing.T) {
	testutil.SkipUnlessUnixFSErrors(t)

	changeDir := t.TempDir()
	// changeDir/specs is a regular file, not a directory: os.ReadDir on it
	// fails with a real (non-IsNotExist) error.
	if err := os.WriteFile(filepath.Join(changeDir, "specs"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := discoverCapabilities(changeDir); err == nil {
		t.Fatal("discoverCapabilities: want error when specs/ is a regular file, got nil")
	}
}

func TestDiscoverCapabilitiesSkipsStrayFilesAndCapsWithoutSpecMd(t *testing.T) {
	changeDir := t.TempDir()
	// A stray non-directory entry directly under specs/.
	writeFile(t, filepath.Join(changeDir, "specs", "README.md"), "not a capability")
	// A capability directory with no spec.md inside it.
	if err := os.MkdirAll(filepath.Join(changeDir, "specs", "empty-cap"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A genuine capability.
	writeFile(t, filepath.Join(changeDir, "specs", "auth", "spec.md"), "## ADDED Requirements\n")

	caps, err := discoverCapabilities(changeDir)
	if err != nil {
		t.Fatalf("discoverCapabilities: %v", err)
	}
	if len(caps) != 1 || caps[0] != "auth" {
		t.Errorf("caps = %v, want exactly [auth]", caps)
	}
}
