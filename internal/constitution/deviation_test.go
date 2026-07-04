package constitution

import (
	"path/filepath"
	"testing"
)

func TestDeviationValidateValid(t *testing.T) {
	bin := fakeConstitutionBin(t, 0, "deviation validate: valid.json is valid\n", "")
	root := t.TempDir()
	res, err := DeviationValidate(bin, root, filepath.Join(root, "valid.json"))
	if err != nil {
		t.Fatalf("DeviationValidate: %v", err)
	}
	if !res.Valid() || res.ExitCode != DeviationValid {
		t.Errorf("res = %+v, want ExitCode 0 / Valid() true", res)
	}
}

func TestDeviationValidateInvalid(t *testing.T) {
	bin := fakeConstitutionBin(t, 1, "", "deviation validate: at /deviations/0/adrId: missing property 'adrId'\n")
	root := t.TempDir()
	res, err := DeviationValidate(bin, root, filepath.Join(root, "bad.json"))
	if err != nil {
		t.Fatalf("DeviationValidate: %v", err)
	}
	if res.Valid() || res.ExitCode != DeviationInvalid {
		t.Errorf("res = %+v, want ExitCode 1 / Valid() false", res)
	}
	if res.Stderr == "" {
		t.Error("Stderr is empty, want the schema error line")
	}
}

func TestDeviationValidateCouldNotRun(t *testing.T) {
	bin := fakeConstitutionBin(t, 2, "", "deviation validate: no constitution.yml in /tmp/empty; run from a constitution project root\n")
	root := t.TempDir()
	res, err := DeviationValidate(bin, root, filepath.Join(root, "valid.json"))
	if err != nil {
		t.Fatalf("DeviationValidate: %v", err)
	}
	if res.ExitCode != DeviationCouldNotRun {
		t.Errorf("ExitCode = %v, want DeviationCouldNotRun", res.ExitCode)
	}
}

func TestDeviationValidateBinNotExecutable(t *testing.T) {
	root := t.TempDir()
	_, err := DeviationValidate(filepath.Join(root, "does-not-exist"), root, filepath.Join(root, "valid.json"))
	if err == nil {
		t.Fatal("DeviationValidate with a nonexistent binary: error = nil, want an error")
	}
}
