package mediaserver

import (
	"slices"
	"sync"
	"time"
)

// LibraryLoad is how far one library has loaded
type LibraryLoad struct {
	Title  string `json:"title"`
	Loaded int    `json:"loaded"` // Items loaded so far
	Total  int    `json:"total"`  // Items in the library, its size last time until the media server says
	Done   bool   `json:"done"`
}

// LibraryProgress is how far loading the main media server's libraries has got
type LibraryProgress struct {
	Running    bool          `json:"running"`
	Failed     bool          `json:"failed"`
	Libraries  []LibraryLoad `json:"libraries"`
	StartedAt  int64         `json:"started_at"`
	FinishedAt int64         `json:"finished_at"`
}

var (
	progressMu          sync.Mutex
	progress            = LibraryProgress{Libraries: []LibraryLoad{}}
	progressSubscribers = map[chan LibraryProgress]struct{}{}
)

// CurrentLibraryProgress returns how far loading the libraries has got
func CurrentLibraryProgress() LibraryProgress {
	progressMu.Lock()
	defer progressMu.Unlock()
	return copyProgress(progress)
}

// SubscribeLibraryProgress returns a channel that gets the progress every time it changes, and the progress now
func SubscribeLibraryProgress() (chan LibraryProgress, LibraryProgress) {
	progressMu.Lock()
	defer progressMu.Unlock()
	ch := make(chan LibraryProgress, 1)
	progressSubscribers[ch] = struct{}{}
	return ch, copyProgress(progress)
}

func UnsubscribeLibraryProgress(ch chan LibraryProgress) {
	progressMu.Lock()
	defer progressMu.Unlock()
	delete(progressSubscribers, ch)
}

func copyProgress(p LibraryProgress) LibraryProgress {
	p.Libraries = slices.Clone(p.Libraries)
	return p
}

// updateLibraryProgress changes the progress and sends it to every subscriber
func updateLibraryProgress(change func(p *LibraryProgress)) {
	progressMu.Lock()
	defer progressMu.Unlock()
	change(&progress)
	for ch := range progressSubscribers {
		// Only the newest progress matters, so replace one that hasn't been read yet
		select {
		case <-ch:
		default:
		}
		ch <- copyProgress(progress)
	}
}

// startLibraryProgress starts a load with each library's size from last time
func startLibraryProgress(titles []string, lastTotals map[string]int) {
	updateLibraryProgress(func(p *LibraryProgress) {
		*p = LibraryProgress{Running: true, StartedAt: time.Now().Unix(), Libraries: []LibraryLoad{}}
		for _, title := range titles {
			p.Libraries = append(p.Libraries, LibraryLoad{Title: title, Total: lastTotals[title]})
		}
	})
}

func updateLibraryLoad(title string, loaded, total int, done bool) {
	updateLibraryProgress(func(p *LibraryProgress) {
		for i := range p.Libraries {
			if p.Libraries[i].Title == title {
				p.Libraries[i] = LibraryLoad{Title: title, Loaded: loaded, Total: total, Done: done}
			}
		}
	})
}

func finishLibraryProgress(success bool) {
	updateLibraryProgress(func(p *LibraryProgress) {
		p.Running = false
		p.Failed = !success
		p.FinishedAt = time.Now().Unix()
	})
}
