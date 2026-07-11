package downloadqueue

import (
	"regexp"
	"strings"
)

// unsafeQueueSegmentChars matches any run of characters that must not appear in a
// queue filename segment. Anything outside this allow-list — most importantly the
// path separators "/" and "\" — is collapsed to a single underscore.
var unsafeQueueSegmentChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// sanitizeQueueSegment reduces a user-provided value (a Collection's library
// title or rating key) to a single safe path segment for use in a collection
// download-queue filename. The Collection is supplied verbatim in the request
// body, so its fields are attacker-controlled; collapsing path separators and
// trimming leading/trailing dots guarantees the value cannot traverse out of the
// queue folder (CodeQL go/path-injection).
//
// AddCollectionToQueue, RemoveCollectionFromQueue, and RetryCollectionFromQueue
// all run request values through this helper so the filename that is written and
// the patterns that later match it stay in sync.
func sanitizeQueueSegment(s string) string {
	s = strings.ReplaceAll(s, " ", "_")
	s = unsafeQueueSegmentChars.ReplaceAllString(s, "_")
	s = strings.Trim(s, ".")
	if s == "" {
		return "unknown"
	}
	return s
}
