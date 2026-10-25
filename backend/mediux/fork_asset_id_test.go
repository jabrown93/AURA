package mediux

import "testing"

func TestValidAssetID(t *testing.T) {
	for id, want := range map[string]bool{
		"0b6a2c1e-7f3d-4c0a-9d6e-1a2b3c4d5e6f": true,
		"abc_123":                              true,
		"":                                     false,
		"../../config/AURA":                    false,
		"a/b":                                  false,
		"kometa|movies/Film/poster.jpg":        false,
	} {
		if got := validAssetID(id); got != want {
			t.Errorf("validAssetID(%q) = %v, want %v", id, got, want)
		}
	}
}
