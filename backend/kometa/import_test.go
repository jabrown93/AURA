package kometa

import (
	"aura/config"
	"testing"
	"time"
)

func TestRunImportWaitsForImportCompletion(t *testing.T) {
	previousConfig, previousExecute := config.Current, executeImport
	statusMu.Lock()
	previousRunning, previousResult := running, lastResult
	running, lastResult = false, nil
	statusMu.Unlock()
	t.Cleanup(func() {
		config.Current, executeImport = previousConfig, previousExecute
		statusMu.Lock()
		running, lastResult = previousRunning, previousResult
		statusMu.Unlock()
	})

	config.Current.Images.Kometa.Enabled = true
	config.Current.Images.Kometa.AssetDirectory = t.TempDir()
	config.Current.MediaServer.Type = "Plex"

	entered := make(chan struct{})
	release := make(chan struct{})
	executeImport = func() {
		close(entered)
		<-release
		finish(&ImportResult{})
	}

	done := make(chan bool, 1)
	go func() { done <- RunImport() }()
	<-entered

	select {
	case <-done:
		t.Fatal("RunImport returned before import completed")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	if !<-done {
		t.Fatal("RunImport did not start import")
	}
}
