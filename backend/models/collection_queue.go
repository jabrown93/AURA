package models

import "time"

// CollectionQueueItem is a queued request to download and apply one or more
// MediUX images to a native media-server Collection (e.g. "James Bond
// Collection"). Unlike DBSavedItem it is not tied to a movie/show MediaItem or a
// MediUX poster set: a Collection only supports poster and backdrop artwork,
// which is applied directly to the Collection's rating key via
// mediaserver.ApplyCollectionImage.
//
// Collection queue entries live in their own download-queue/collections
// subfolder so the media-item queue code (which unmarshals every file into a
// DBSavedItem) never sees them.
type CollectionQueueItem struct {
	CollectionItem CollectionItem `json:"collection_item"`
	Images         []ImageFile    `json:"images"`

	// Queue-only, transient failure metadata. The collection queue processor
	// populates these when it moves an entry to the error_/warning_ state so the
	// UI can show why it failed. They are omitted from every other response via
	// omitempty and are never persisted anywhere else.

	// QueueErrors lists the fatal reasons the entry failed to apply.
	QueueErrors []string `json:"queue_errors,omitempty"`
	// QueueWarnings lists non-fatal issues recorded while processing the entry.
	QueueWarnings []string `json:"queue_warnings,omitempty"`
	// FailedAt is when the entry was moved to its terminal error/warning state.
	FailedAt *time.Time `json:"failed_at,omitempty"`
}
