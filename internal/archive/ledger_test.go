package archive

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEmptyImageSHAIsTheKnownEmptyByteStringHash(t *testing.T) {
	// The well-known sha256("") value — also visible in
	// testdata/conformance/manifest.json against a 0-byte fixture file, so
	// this package's "empty" sentinel is the same value the corpus
	// already treats as "the hash of nothing" (doc.go).
	const want = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if emptyImageSHA != want {
		t.Errorf("emptyImageSHA = %q, want %q", emptyImageSHA, want)
	}
}

func TestManifestSHADeterministicAndContentSensitive(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "hello")
	writeFile(t, filepath.Join(dir, "sub", "b.txt"), "world")

	got1, err := ManifestSHA(dir)
	if err != nil {
		t.Fatalf("ManifestSHA: %v", err)
	}
	got2, err := ManifestSHA(dir)
	if err != nil {
		t.Fatalf("ManifestSHA (2nd call): %v", err)
	}
	if got1 != got2 {
		t.Errorf("ManifestSHA is not deterministic: %q != %q", got1, got2)
	}

	writeFile(t, filepath.Join(dir, "a.txt"), "hello, mutated")
	got3, err := ManifestSHA(dir)
	if err != nil {
		t.Fatalf("ManifestSHA (after mutation): %v", err)
	}
	if got3 == got1 {
		t.Error("ManifestSHA did not change after file content changed")
	}
}

func TestAppendRecordsAssignsMonotonicSeq(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "openspec"), 0o755); err != nil {
		t.Fatal(err)
	}

	first, err := AppendRecords(root, []Record{{Change: "a", Capability: "x"}})
	if err != nil {
		t.Fatalf("AppendRecords (1st): %v", err)
	}
	if len(first) != 1 || first[0].Seq != 1 {
		t.Fatalf("first append = %+v, want a single record with Seq 1", first)
	}

	second, err := AppendRecords(root, []Record{
		{Change: "b", Capability: "y"},
		{Change: "b", Capability: "z"},
	})
	if err != nil {
		t.Fatalf("AppendRecords (2nd): %v", err)
	}
	if len(second) != 2 || second[0].Seq != 2 || second[1].Seq != 3 {
		t.Fatalf("second append = %+v, want Seq 2 then 3", second)
	}

	all, err := ReadAll(root)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("ReadAll returned %d records, want 3", len(all))
	}
	for i, want := range []int{1, 2, 3} {
		if all[i].Seq != want {
			t.Errorf("all[%d].Seq = %d, want %d", i, all[i].Seq, want)
		}
	}
}

func TestReadAllOnMissingLedgerIsEmptyNotError(t *testing.T) {
	root := t.TempDir()
	all, err := ReadAll(root)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if all != nil {
		t.Errorf("ReadAll on a missing ledger = %+v, want nil", all)
	}
}

func TestReadAllPropagatesNonNotExistReadError(t *testing.T) {
	root := t.TempDir()
	// The ledger path is itself a directory, not a file.
	if err := os.MkdirAll(LedgerPath(root), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadAll(root); err == nil {
		t.Fatal("ReadAll: want error when the ledger path is a directory, got nil")
	}
}

func TestReadAllPropagatesMalformedLedgerLine(t *testing.T) {
	root := t.TempDir()
	writeFile(t, LedgerPath(root), "not json at all\n")

	if _, err := ReadAll(root); err == nil {
		t.Fatal("ReadAll: want error for a malformed ledger line, got nil")
	}
}

func TestAppendRecordsEmptyIsNoop(t *testing.T) {
	root := t.TempDir()
	out, err := AppendRecords(root, nil)
	if err != nil {
		t.Fatalf("AppendRecords(nil): %v", err)
	}
	if out != nil {
		t.Errorf("AppendRecords(nil) = %+v, want nil", out)
	}
	if _, statErr := os.Stat(LedgerPath(root)); !os.IsNotExist(statErr) {
		t.Errorf("ledger file was created for an empty append (stat err = %v)", statErr)
	}
}

func TestAppendRecordsPropagatesNonNotExistReadError(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(LedgerPath(root), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := AppendRecords(root, []Record{{Change: "a"}}); err == nil {
		t.Fatal("AppendRecords: want error when the ledger path is a directory, got nil")
	}
}

func TestAppendRecordsPropagatesMalformedExistingLedger(t *testing.T) {
	root := t.TempDir()
	writeFile(t, LedgerPath(root), "not json at all\n")

	if _, err := AppendRecords(root, []Record{{Change: "a"}}); err == nil {
		t.Fatal("AppendRecords: want error when the existing ledger is malformed, got nil")
	}
}

func TestAppendRecordsPropagatesWriteFailure(t *testing.T) {
	root := t.TempDir() // openspec/ does not exist, so the atomic write's
	// temp-file creation has nowhere to go.

	if _, err := AppendRecords(root, []Record{{Change: "a"}}); err == nil {
		t.Fatal("AppendRecords: want error when openspec/ does not exist, got nil")
	}
}
