package anidb

import (
	"encoding/json"
	"testing"
)

func TestFirstID(t *testing.T) {
	cases := map[string]string{
		`377543`:         "377543", // bare number (tvdb_id)
		`"377543"`:       "377543", // quoted string
		`["tt12343534"]`: "tt12343534",
		`[128]`:          "128",
		`[[42], [7]]`:    "42", // nested array
		`null`:           "",
		``:               "",
		`[]`:             "",
		`{"tv":95479}`:   `{"tv":95479}`, // objects aren't ids; returned trimmed (caller uses parseThemoviedbID)
	}
	for in, want := range cases {
		if got := firstID(json.RawMessage(in)); got != want {
			t.Errorf("firstID(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestParseThemoviedbID(t *testing.T) {
	cases := []struct {
		raw       string
		wantTV    string
		wantMovie string
	}{
		{`{"tv":95479}`, "95479", ""},
		{`{"movie":[128]}`, "", "128"},
		{`{"movie":128}`, "", "128"},
		{`null`, "", ""},
		{``, "", ""},
		{`{}`, "", ""},
	}
	for _, tc := range cases {
		tv, movie := parseThemoviedbID(json.RawMessage(tc.raw))
		if tv != tc.wantTV || movie != tc.wantMovie {
			t.Errorf("parseThemoviedbID(%q) = (%q, %q); want (%q, %q)", tc.raw, tv, movie, tc.wantTV, tc.wantMovie)
		}
	}
}
