package archive

import (
	"os"
	"path/filepath"
	"sort"
)

// discoverCapabilities returns the sorted set of capability names a change
// folder's delta touches: every direct subdirectory of changeDir/specs
// that contains a spec.yaml file (the owned YAML delta source, change 007
// M8 — spec.md is a one-way projection, never a delta source). A missing
// specs/ directory (a delta-less bug) yields a nil slice, not an error.
func discoverCapabilities(changeDir string) ([]string, error) {
	root := filepath.Join(changeDir, "specs")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var caps []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, e.Name(), "spec.yaml")); err != nil {
			continue
		}
		caps = append(caps, e.Name())
	}
	sort.Strings(caps)
	return caps, nil
}
