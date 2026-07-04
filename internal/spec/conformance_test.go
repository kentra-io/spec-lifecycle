package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// conformanceManifest mirrors the shape of
// testdata/conformance/manifest.json: a flat list of every corpus file,
// path-relative to testdata/conformance/, with its expected sha256 and byte
// count.
type conformanceManifest struct {
	Files []struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
		Bytes  int64  `json:"bytes"`
	} `json:"files"`
}

const conformanceRoot = "../../testdata/conformance"

// TestConformanceManifestIntegrity re-hashes every file the manifest lists
// and fails on any drift — a corrupted or hand-edited corpus fixture must
// not silently pass conformance_test.go's fold/render assertions below.
func TestConformanceManifestIntegrity(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(conformanceRoot, "manifest.json"))
	if err != nil {
		t.Fatalf("reading manifest.json: %v", err)
	}
	var m conformanceManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parsing manifest.json: %v", err)
	}
	if len(m.Files) == 0 {
		t.Fatal("manifest.json lists no files")
	}
	for _, f := range m.Files {
		full := filepath.Join(conformanceRoot, f.Path)
		b, err := os.ReadFile(full)
		if err != nil {
			t.Errorf("%s: %v", f.Path, err)
			continue
		}
		if int64(len(b)) != f.Bytes {
			t.Errorf("%s: %d bytes, manifest says %d", f.Path, len(b), f.Bytes)
		}
		sum := sha256.Sum256(b)
		got := hex.EncodeToString(sum[:])
		if got != f.SHA256 {
			t.Errorf("%s: sha256 %s, manifest says %s", f.Path, got, f.SHA256)
		}
	}
}

// TestConformanceCorpus is the M1 DoD's core proof: for every case in
// testdata/conformance/cases/, parse the pre-archive living spec (base) and
// the change's per-capability delta, Fold them, Render the result, and
// byte-compare against the oracle's own post-archive expected/specs/ output
// — no whitespace normalization, no TrimSpace anywhere in this test.
func TestConformanceCorpus(t *testing.T) {
	casesDir := filepath.Join(conformanceRoot, "cases")
	entries, err := os.ReadDir(casesDir)
	if err != nil {
		t.Fatalf("reading cases dir: %v", err)
	}

	var slugs []string
	for _, e := range entries {
		if e.IsDir() {
			slugs = append(slugs, e.Name())
		}
	}
	sort.Strings(slugs)
	if len(slugs) == 0 {
		t.Fatal("no conformance cases found")
	}

	for _, slug := range slugs {
		t.Run(slug, func(t *testing.T) {
			runConformanceCase(t, filepath.Join(casesDir, slug))
		})
	}
}

func runConformanceCase(t *testing.T, caseDir string) {
	t.Helper()

	changeRoot := filepath.Join(caseDir, "change")
	changeEntries, err := os.ReadDir(changeRoot)
	if err != nil {
		t.Fatalf("reading change dir: %v", err)
	}
	var changeName string
	for _, e := range changeEntries {
		if e.IsDir() {
			if changeName != "" {
				t.Fatalf("expected exactly one change folder under %s, found a second: %s", changeRoot, e.Name())
			}
			changeName = e.Name()
		}
	}
	if changeName == "" {
		t.Fatalf("no change folder found under %s", changeRoot)
	}

	deltaSpecsRoot := filepath.Join(changeRoot, changeName, "specs")
	capEntries, err := os.ReadDir(deltaSpecsRoot)
	if err != nil {
		t.Fatalf("reading delta specs dir: %v", err)
	}

	var capabilities []string
	for _, e := range capEntries {
		if e.IsDir() {
			capabilities = append(capabilities, e.Name())
		}
	}
	sort.Strings(capabilities)
	if len(capabilities) == 0 {
		t.Fatal("change delta declares no capabilities")
	}

	for _, cap := range capabilities {
		t.Run(cap, func(t *testing.T) {
			deltaPath := filepath.Join(deltaSpecsRoot, cap, "spec.md")
			deltaBytes, err := os.ReadFile(deltaPath)
			if err != nil {
				t.Fatalf("reading delta %s: %v", deltaPath, err)
			}
			delta, err := ParseDelta(deltaBytes)
			if err != nil {
				t.Fatalf("ParseDelta(%s): %v", deltaPath, err)
			}

			var base *RequirementSet
			beforePath := filepath.Join(caseDir, "before", "specs", cap, "spec.md")
			if beforeBytes, err := os.ReadFile(beforePath); err == nil {
				base, err = ParseRequirementSet(beforeBytes)
				if err != nil {
					t.Fatalf("ParseRequirementSet(%s): %v", beforePath, err)
				}
			} else if !os.IsNotExist(err) {
				t.Fatalf("reading before spec %s: %v", beforePath, err)
			}

			folded, err := Fold(cap, changeName, base, delta)
			if err != nil {
				t.Fatalf("Fold(%s, %s): %v", cap, changeName, err)
			}
			got := folded.Render()

			expectedPath := filepath.Join(caseDir, "expected", "specs", cap, "spec.md")
			want, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatalf("reading expected %s: %v", expectedPath, err)
			}

			if string(got) != string(want) {
				t.Errorf("fold+render mismatch for capability %q\n--- got ---\n%s\n--- want ---\n%s", cap, got, want)
			}
		})
	}
}
