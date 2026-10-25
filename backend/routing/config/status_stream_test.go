package routes_config

import (
	"aura/config"
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStreamConfigStatusSendsEachStep(t *testing.T) {
	previousStep := config.AppLoadingStep
	t.Cleanup(func() { config.AppLoadingStep = previousStep })
	config.SetAppLoadingStep("Loading Configuration")

	server := httptest.NewServer(http.HandlerFunc(StreamAppConfigStatus))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q", ct)
	}

	events := make(chan string)
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			if line := scanner.Text(); strings.HasPrefix(line, "data: ") {
				var body configStatusResponse
				if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &body) == nil {
					events <- body.Status.AppLoadingStep
				}
			}
		}
	}()
	next := func() string {
		select {
		case step := <-events:
			return step
		case <-time.After(3 * time.Second):
			t.Fatal("no event received")
			return ""
		}
	}

	if step := next(); step != "Loading Configuration" {
		t.Errorf("first event step = %q, want the current step", step)
	}
	config.SetAppLoadingStep("Optimizing Database")
	if step := next(); step != "Optimizing Database" {
		t.Errorf("event after the step changed = %q, want Optimizing Database", step)
	}
}
