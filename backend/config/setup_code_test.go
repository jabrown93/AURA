package config

import (
	"strings"
	"testing"
)

func TestSetupCode(t *testing.T) {
	code := SetupCode()
	if len(code) != 9 || code[4] != '-' {
		t.Fatalf("SetupCode() = %q, want XXXX-XXXX", code)
	}
	if SetupCode() != code {
		t.Error("the setup code should stay the same for the whole run")
	}

	for _, attempt := range []string{code, strings.ToLower(code), strings.ReplaceAll(code, "-", ""), " " + code + " "} {
		if !CheckSetupCode(attempt) {
			t.Errorf("CheckSetupCode(%q) = false, want true", attempt)
		}
	}
	for _, attempt := range []string{"", "AAAA-AAAA", code + "X", code[:8]} {
		if CheckSetupCode(attempt) {
			t.Errorf("CheckSetupCode(%q) = true, want false", attempt)
		}
	}
}
