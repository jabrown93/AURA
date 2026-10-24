package routes_auth

import "testing"

func TestHasAllowedGroup(t *testing.T) {
	allowed := []string{"arr-admins"}
	cases := []struct {
		name string
		raw  any
		want bool
	}{
		{"list match", []any{"users", "ARR-admins"}, true},
		{"string match", "arr-admins", true},
		{"no match", []any{"users"}, false},
		{"missing claim", nil, false},
		{"wrong type", 42, false},
	}
	for _, c := range cases {
		if got := hasAllowedGroup(c.raw, allowed); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
