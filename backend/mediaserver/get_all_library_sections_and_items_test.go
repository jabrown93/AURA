package mediaserver

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetAllLibrarySectionsAndItemsSerializesInitialRefresh(t *testing.T) {
	warmupMu.Lock()
	oldRun := getAllLibrarySectionsAndItemsRun
	oldDone := warmupDone
	warmupDone = false
	warmupMu.Unlock()
	t.Cleanup(func() {
		warmupMu.Lock()
		getAllLibrarySectionsAndItemsRun = oldRun
		warmupDone = oldDone
		warmupMu.Unlock()
	})

	var calls atomic.Int32
	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	getAllLibrarySectionsAndItemsRun = func(context.Context) bool {
		if calls.Add(1) == 1 {
			close(firstEntered)
			<-releaseFirst
		}
		return true
	}

	firstDone := make(chan struct{})
	go func() {
		GetAllLibrarySectionsAndItems(context.Background(), false)
		close(firstDone)
	}()
	<-firstEntered

	secondDone := make(chan struct{})
	go func() {
		GetAllLibrarySectionsAndItems(context.Background(), false)
		close(secondDone)
	}()

	select {
	case <-secondDone:
		t.Fatal("second initial refresh overlapped first")
	case <-time.After(20 * time.Millisecond):
	}
	close(releaseFirst)
	<-firstDone
	<-secondDone

	if got := calls.Load(); got != 1 {
		t.Fatalf("refresh calls = %d, want 1", got)
	}
}
