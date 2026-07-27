package plandag

import "testing"

func TestValidatePlanValid(t *testing.T) {
	bin := fakeBin(t, 0, "plan is valid\n", "")
	report, valid, err := Validate(bin, "plan.yaml")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !valid {
		t.Errorf("valid = false, want true for a 0-exit report (%q)", report)
	}
}

func TestValidatePlanInvalidSurfacesMessage(t *testing.T) {
	bin := fakeBin(t, 1, "", "milestone 3: dependency cycle A -> B -> A\n")
	report, valid, err := Validate(bin, "plan.yaml")
	if err != nil {
		t.Fatalf("Validate: %v (a non-zero exit is an invalid plan, not a run failure)", err)
	}
	if valid {
		t.Fatal("valid = true, want false for a non-zero exit")
	}
	if report != "milestone 3: dependency cycle A -> B -> A" {
		t.Errorf("report = %q, want the primitive's message surfaced verbatim", report)
	}
}

func TestResolveParsesMilestones(t *testing.T) {
	bin := fakeBin(t, 0, `milestones:
  - id: 1
    title: First
    done: true
  - id: 2
    title: Second
    done: false
`, "")
	ms, err := Resolve(bin, "plan.yaml")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(ms) != 2 {
		t.Fatalf("milestones = %+v, want 2", ms)
	}
	if ms[0].ID != 1 || ms[0].Title != "First" || !ms[0].Done {
		t.Errorf("ms[0] = %+v, want {1 First true}", ms[0])
	}
	if ms[1].ID != 2 || ms[1].Title != "Second" || ms[1].Done {
		t.Errorf("ms[1] = %+v, want {2 Second false}", ms[1])
	}
}

func TestResolveNonZeroExitIsError(t *testing.T) {
	bin := fakeBin(t, 2, "", "cannot read plan\n")
	if _, err := Resolve(bin, "plan.yaml"); err == nil {
		t.Fatal("Resolve: err = nil, want an error on a non-zero exit")
	}
}
