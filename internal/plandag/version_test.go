package plandag

import "testing"

func TestCheckVersion(t *testing.T) {
	tests := []struct {
		name           string
		stdout         string
		pin            string
		wantCertain    bool
		wantCompatible bool
		wantWarning    bool
	}{
		{
			name: "empty pin always compatible", stdout: "milestoned-plan-dag version (devel)\n", pin: "",
			wantCertain: true, wantCompatible: true, wantWarning: false,
		},
		{
			name: "matching pin", stdout: "milestoned-plan-dag version 0.1.5 (abcdef012345)\n", pin: "0.1.x",
			wantCertain: true, wantCompatible: true, wantWarning: false,
		},
		{
			name: "mismatched pin", stdout: "milestoned-plan-dag version 0.2.0 (abcdef012345)\n", pin: "0.1.x",
			wantCertain: true, wantCompatible: false, wantWarning: true,
		},
		{
			name: "dev build cannot confirm", stdout: "milestoned-plan-dag version (devel)\n", pin: "0.1.x",
			wantCertain: false, wantCompatible: false, wantWarning: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bin := fakeBin(t, 0, tt.stdout, "")
			pf, err := CheckVersion(bin, tt.pin)
			if err != nil {
				t.Fatalf("CheckVersion: %v", err)
			}
			if pf.Certain != tt.wantCertain {
				t.Errorf("Certain = %v, want %v", pf.Certain, tt.wantCertain)
			}
			if pf.Compatible != tt.wantCompatible {
				t.Errorf("Compatible = %v, want %v", pf.Compatible, tt.wantCompatible)
			}
			if (pf.Warning != "") != tt.wantWarning {
				t.Errorf("Warning = %q, want non-empty = %v", pf.Warning, tt.wantWarning)
			}
		})
	}
}

func TestVersionStripsPrefix(t *testing.T) {
	bin := fakeBin(t, 0, "milestoned-plan-dag version 1.2.3 (deadbeef)\n", "")
	v, err := Version(bin)
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v != "1.2.3 (deadbeef)" {
		t.Errorf("Version = %q, want the prefix stripped", v)
	}
}
