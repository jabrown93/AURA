package models

import "time"

// What is used to save a record into the database
// This contains the MediaItem details, as well as an array of PosterSets that are associated with it
type DBSavedItem struct {
	MediaItem  MediaItem           `json:"media_item"`
	PosterSets []DBPosterSetDetail `json:"poster_sets,omitempty"`

	// Queue-only, transient failure metadata. The download-queue processor
	// populates these when it moves an entry to the error_/warning_ state so the
	// UI can show why it failed. They are never persisted to the database
	// (UpsertSavedItem reads explicit columns, not the marshalled struct) and are
	// omitted from every other response via omitempty.

	// QueueErrors lists the fatal reasons the entry failed to download.
	QueueErrors []string `json:"queue_errors,omitempty"`
	// QueueWarnings lists non-fatal issues recorded while processing the entry.
	QueueWarnings []string `json:"queue_warnings,omitempty"`
	// FailedAt is when the entry was moved to the error/warning state.
	FailedAt *time.Time `json:"failed_at,omitempty"`
}

// PosterSetDetail groups poster set details per media item.
type DBPosterSetDetail struct {
	PosterSet
	LastDownloaded            time.Time     `json:"last_downloaded"`
	SelectedTypes             SelectedTypes `json:"selected_types"`
	AutoDownload              bool          `json:"auto_download"`
	AutoAddNewCollectionItems bool          `json:"auto_add_new_collection_items"`
	ToDelete                  bool          `json:"to_delete"` // Flag to indicate if the poster set should be deleted (Not used in DB)
}

type PosterSet struct {
	BaseSetInfo
	Images []ImageFile `json:"images"`
}

type DBFilter struct {
	ItemTMDB_ID       string   `json:"item_tmdb_id"`
	ItemLibraryTitle  string   `json:"item_library_title"`
	ItemYear          int      `json:"item_year"`
	ItemTitle         string   `json:"item_title"`
	SetID             string   `json:"set_id"`
	LibraryTitles     []string `json:"library_titles"`
	ImageTypes        []string `json:"image_types"`
	Autodownload      string   `json:"autodownload"`
	MultiSetOnly      bool     `json:"multiset_only"`
	Usernames         []string `json:"usernames"`
	MediaItemOnServer string   `json:"media_item_on_server"`
	ItemsPerPage      int      `json:"items_per_page"`
	PageNumber        int      `json:"page_number"`
	SortOption        string   `json:"sort_option"`
	SortOrder         string   `json:"sort_order"`
}
