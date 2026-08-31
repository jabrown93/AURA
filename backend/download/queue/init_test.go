package downloadqueue

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	code := m.Run()
	// The package init() creates the queue folders relative to the (unset)
	// config path when running under `go test`.
	os.RemoveAll(FolderPath)
	os.Exit(code)
}

// Queue processors keep appending to the error/warning slices after handing
// them to UpdateLatestInfo, so the stored copy must not alias the caller's
// backing array.
func TestUpdateLatestInfoClonesSlices(t *testing.T) {
	errors := make([]string, 1, 4)
	errors[0] = "first"

	UpdateLatestInfo(func(info *LatestInfo) {
		info.Errors = errors
		info.Warnings = nil
	})

	errors = append(errors, "second")
	errors[0] = "mutated"

	stored := GetLatestInfo().Errors
	if len(stored) != 1 || stored[0] != "first" {
		t.Fatalf("stored errors alias the caller slice: got %v", stored)
	}
}

func TestLatestInfoConcurrentAccess(t *testing.T) {
	var wg sync.WaitGroup

	for writer := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 200 {
				errors := make([]string, 1, 4)
				errors[0] = fmt.Sprintf("error-%d-%d", writer, i)

				UpdateLatestInfo(func(info *LatestInfo) {
					info.Time = time.Now()
					info.Status = LAST_STATUS_PROCESSING
					info.Message = fmt.Sprintf("processing %d", i)
					info.Errors = errors
					info.Warnings = []string{}
				})

				errors = append(errors, "appended after publishing")
			}
		}()
	}

	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 200 {
				info := GetLatestInfo()
				for _, message := range info.Errors {
					_ = message
				}
				for _, message := range info.Warnings {
					_ = message
				}
			}
		}()
	}

	wg.Wait()
}
