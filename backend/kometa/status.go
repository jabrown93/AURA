package kometa

import (
	"sync"
	"time"
)

// FolderOutcome captures what happened to a single asset folder during an import.
type FolderOutcome struct {
	Folder         string `json:"folder"`
	Outcome        string `json:"outcome"` // matched, unmatched, collection, error
	Detail         string `json:"detail,omitempty"`
	ImagesUploaded int    `json:"images_uploaded"`
	ImagesFailed   int    `json:"images_failed"`
	RegisteredInDB bool   `json:"registered_in_db"`
	ManagedByAura  bool   `json:"managed_by_aura"` // matched but DB registration skipped to protect MediUX selections
}

// ImportResult is the summary of the most recent import run.
type ImportResult struct {
	StartedAt            time.Time       `json:"started_at"`
	FinishedAt           time.Time       `json:"finished_at"`
	FoldersScanned       int             `json:"folders_scanned"`
	Matched              int             `json:"matched"`
	Collections          int             `json:"collections"`
	UnmatchedFolders     int             `json:"unmatched_folders"`
	ImagesUploaded       int             `json:"images_uploaded"`
	ImagesFailed         int             `json:"images_failed"`
	ItemsRegistered      int             `json:"items_registered"`
	SkippedManagedByAura int             `json:"skipped_managed_by_aura"`
	Error                string          `json:"error,omitempty"`
	Folders              []FolderOutcome `json:"folders,omitempty"`
}

var (
	statusMu   sync.Mutex
	running    bool
	lastResult *ImportResult
)

// Status returns whether an import is currently running and the most recent result (if any).
func Status() (isRunning bool, result *ImportResult) {
	statusMu.Lock()
	defer statusMu.Unlock()
	return running, lastResult
}

// tryStart marks an import as running. It returns false if one is already in progress.
func tryStart() bool {
	statusMu.Lock()
	defer statusMu.Unlock()
	if running {
		return false
	}
	running = true
	return true
}

// finish records the result and clears the running flag.
func finish(result *ImportResult) {
	statusMu.Lock()
	defer statusMu.Unlock()
	running = false
	lastResult = result
}
