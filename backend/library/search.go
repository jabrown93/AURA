package library

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// The search works like searchItems in the frontend, with Y:2020:, L:4K Movies: and ID:603: filters

var (
	yearFilterRe    = regexp.MustCompile(`[Yy]:(\d{4}):`)
	libraryFilterRe = regexp.MustCompile(`[Ll]:(.+?):`)
	idFilterRe      = regexp.MustCompile(`[Ii][Dd]:(.+?):`)
)

// search is a parsed search
type search struct {
	exactTitle string   // Set when the title is wrapped in quotes
	titleWords []string // Every word must be in the title
	year       int
	library    string
	id         string
	empty      bool
}

func parseSearch(query string) search {
	query = strings.TrimSpace(query)
	if query == "" {
		return search{empty: true}
	}

	s := search{}
	if m := yearFilterRe.FindStringSubmatch(query); m != nil {
		s.year, _ = strconv.Atoi(m[1])
	}
	if m := libraryFilterRe.FindStringSubmatch(query); m != nil {
		s.library = normalize(strings.TrimSpace(m[1]))
	}
	if m := idFilterRe.FindStringSubmatch(query); m != nil {
		s.id = strings.TrimSpace(m[1])
	}

	// The title is what is left once the filters are taken out, one of each
	title := replaceFirst(yearFilterRe, query)
	title = replaceFirst(libraryFilterRe, title)
	title = strings.TrimSpace(replaceFirst(idFilterRe, title))

	if inner, quoted := unquote(title); quoted {
		s.exactTitle = normalize(inner)
		return s
	}
	if title != "" {
		s.titleWords = strings.Fields(normalize(title))
	}
	return s
}

func (s search) matches(item Item) bool {
	if s.empty {
		return true
	}
	title := normalize(item.Title)
	if s.exactTitle != "" && title != s.exactTitle {
		return false
	}
	for _, word := range s.titleWords {
		if !strings.Contains(title, word) {
			return false
		}
	}
	if s.year != 0 && item.Year != s.year {
		return false
	}
	if s.library != "" && !strings.Contains(normalize(item.LibraryTitle), s.library) {
		return false
	}
	if s.id != "" {
		id := item.TMDB_ID
		if id == "" {
			id = item.RatingKey
		}
		if id != s.id {
			return false
		}
	}
	return true
}

// normalize keeps ASCII letters, digits, underscores and spaces, in lower case, like the frontend
func normalize(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'):
			b.WriteRune(unicode.ToLower(r))
		case unicode.IsSpace(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// unquote returns the text inside matching quotes
func unquote(s string) (string, bool) {
	runes := []rune(s)
	if len(runes) < 2 {
		return "", false
	}
	pairs := map[rune]rune{'"': '"', '\'': '\'', '‘': '’', '“': '”'}
	if closing, ok := pairs[runes[0]]; ok && runes[len(runes)-1] == closing {
		return string(runes[1 : len(runes)-1]), true
	}
	return "", false
}

func replaceFirst(re *regexp.Regexp, s string) string {
	loc := re.FindStringIndex(s)
	if loc == nil {
		return s
	}
	return s[:loc[0]] + s[loc[1]:]
}
