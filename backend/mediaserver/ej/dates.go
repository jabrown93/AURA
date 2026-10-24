package ej

import "time"

// unixSeconds returns the time in seconds, like the other media servers, or 0 if it isn't set
func unixSeconds(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}
