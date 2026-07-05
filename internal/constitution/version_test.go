package constitution

import "testing"

func TestLeadingDottedNumeric(t *testing.T) {
	tests := []struct {
		in     string
		want   []string
		wantOk bool
	}{
		{"0.1.5", []string{"0", "1", "5"}, true},
		{"v0.1.5", []string{"0", "1", "5"}, true},
		{"0.1.5 (abcdef012345)", []string{"0", "1", "5"}, true},
		{"(devel) (7e24d2aee640-dirty)", nil, false},
		{"unknown", nil, false},
	}
	for _, tt := range tests {
		got, ok := leadingDottedNumeric(tt.in)
		if ok != tt.wantOk {
			t.Errorf("leadingDottedNumeric(%q) ok = %v, want %v", tt.in, ok, tt.wantOk)
			continue
		}
		if !ok {
			continue
		}
		if len(got) != len(tt.want) {
			t.Fatalf("leadingDottedNumeric(%q) = %v, want %v", tt.in, got, tt.want)
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("leadingDottedNumeric(%q) = %v, want %v", tt.in, got, tt.want)
			}
		}
	}
}

func TestVersionSatisfiesPin(t *testing.T) {
	tests := []struct {
		version, pin string
		want         bool
	}{
		{"0.1.5", "0.1.x", true},
		{"0.1.5", "0.1.5", true},
		{"0.2.0", "0.1.x", false},
		{"0.1", "0.1.5", false}, // pin more specific than the reported version
		{"0.1.5", "0", true},
	}
	for _, tt := range tests {
		vparts, ok := leadingDottedNumeric(tt.version)
		if !ok {
			t.Fatalf("leadingDottedNumeric(%q) unexpectedly failed", tt.version)
		}
		got := versionSatisfiesPin(vparts, splitPin(tt.pin))
		if got != tt.want {
			t.Errorf("versionSatisfiesPin(%q, %q) = %v, want %v", tt.version, tt.pin, got, tt.want)
		}
	}
}

func splitPin(pin string) []string {
	if pin == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i < len(pin); i++ {
		if pin[i] == '.' {
			out = append(out, pin[start:i])
			start = i + 1
		}
	}
	out = append(out, pin[start:])
	return out
}

func TestMinorPin(t *testing.T) {
	tests := []struct {
		in     string
		want   string
		wantOk bool
	}{
		{"0.3.2", "0.3.x", true},
		{"v1.2.0-rc1 (abcdef012345)", "1.2.x", true},
		{"0.1.5 (abcdef012345)", "0.1.x", true},
		{"(devel) (7e24d2aee640-dirty)", "", false},
		{"5", "", false}, // only one numeric component — not enough for major.minor
	}
	for _, tt := range tests {
		got, ok := MinorPin(tt.in)
		if ok != tt.wantOk {
			t.Errorf("MinorPin(%q) ok = %v, want %v", tt.in, ok, tt.wantOk)
			continue
		}
		if ok && got != tt.want {
			t.Errorf("MinorPin(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

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
			name: "empty pin always compatible", stdout: "constitution version (devel) (abc-dirty)\n", pin: "",
			wantCertain: true, wantCompatible: true, wantWarning: false,
		},
		{
			name: "matching pin", stdout: "constitution version 0.1.5 (abcdef012345)\n", pin: "0.1.x",
			wantCertain: true, wantCompatible: true, wantWarning: false,
		},
		{
			name: "mismatched pin", stdout: "constitution version 0.2.0 (abcdef012345)\n", pin: "0.1.x",
			wantCertain: true, wantCompatible: false, wantWarning: true,
		},
		{
			name: "dev build cannot confirm", stdout: "constitution version (devel) (abc-dirty)\n", pin: "0.1.x",
			wantCertain: false, wantCompatible: false, wantWarning: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bin := fakeConstitutionBin(t, 0, tt.stdout, "")
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
