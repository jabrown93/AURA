package logging

import (
	"sync"
	"testing"
)

func TestSetLogKeepWhileLogging(t *testing.T) {
	var wg sync.WaitGroup
	wg.Go(func() {
		for range 200 {
			LOGGER.Info().Msg("test")
		}
	})
	for i := range 50 {
		SetLogKeepDays(i%5 + 1)
		SetLogKeepFiles(i%3 + 1)
	}
	wg.Wait()

	SetLogKeepDays(10)
	SetLogKeepFiles(20)
	if logFile.file.MaxAge != 10 || logFile.file.MaxBackups != 20 {
		t.Errorf("MaxAge %d, MaxBackups %d, want 10 and 20", logFile.file.MaxAge, logFile.file.MaxBackups)
	}

	// 0 keeps the current value
	SetLogKeepDays(0)
	if logFile.file.MaxAge != 10 {
		t.Errorf("MaxAge %d, want 10", logFile.file.MaxAge)
	}
}
