package httpx

import (
	"aura/logging"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

var (
	streamsClosed    = make(chan struct{})
	closeStreamsOnce sync.Once
)

// CloseStreams ends the open event streams, so shutting down doesn't wait for them
func CloseStreams() {
	closeStreamsOnce.Do(func() { close(streamsClosed) })
}

// sseKeepAliveInterval is how often an idle stream sends a keep-alive, so it isn't closed
const sseKeepAliveInterval = 25 * time.Second

// SSEStream is an open Server-Sent Events stream
type SSEStream struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// StartSSE sends the headers of a Server-Sent Events stream, false when streaming isn't supported
func StartSSE(w http.ResponseWriter) (*SSEStream, bool) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return nil, false
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	return &SSEStream{w: w, flusher: flusher}, true
}

// Send sends a value as JSON
func (s *SSEStream) Send(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to marshal a Server-Sent Event")
		return
	}
	fmt.Fprintf(s.w, "data: %s\n\n", data)
	s.flusher.Flush()
}

// StreamEvents calls onEvent for each event until the request ends, the channel closes or aura shuts down
func StreamEvents[T any](ctx context.Context, s *SSEStream, events <-chan T, onEvent func(T)) {
	keepAlive := time.NewTicker(sseKeepAliveInterval)
	defer keepAlive.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-streamsClosed:
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			onEvent(event)
		case <-keepAlive.C:
			fmt.Fprint(s.w, ": keep-alive\n\n")
			s.flusher.Flush()
		}
	}
}
