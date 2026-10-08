package httputils

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/moby/v2/daemon/internal/pruneprogress"
	"github.com/moby/moby/v2/daemon/internal/versions"
	"github.com/moby/moby/v2/pkg/authorization"
)

// WritePruneResponse runs a prune operation and writes its report. API v1.56
// clients may opt into a JSON Lines stream with stream=true. Progress is scoped
// to this request; the last message contains the complete report or an error.
// Before the first progress message, errors retain their normal HTTP status.
func WritePruneResponse(ctx context.Context, w http.ResponseWriter, r *http.Request, run func(context.Context) (any, error)) error {
	// Response authorization runs after the handler returns. Keep its single
	// JSON response buffered so plugins can inspect and deny it before any
	// content reaches the client. Flushing progress would bypass that decision.
	_, authorizedResponse := w.(authorization.ResponseModifier)
	if authorizedResponse || versions.LessThan(VersionFromContext(ctx), "1.56") || !BoolValue(r, "stream") {
		report, err := run(ctx)
		if err != nil {
			return err
		}
		return WriteJSON(w, http.StatusOK, report)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	controller := http.NewResponseController(w)
	// Bound each network write, rather than the entire operation: removing an
	// object may legitimately take a long time, but a stalled reader must not
	// hold prune locks indefinitely. Clear the deadline between notifications.
	const writeTimeout = 30 * time.Second
	var mu sync.Mutex
	var started bool
	var writeErr error
	write := func(message jsonstream.Message) {
		if writeErr != nil {
			return
		}
		if !started {
			w.Header().Set("Content-Type", "application/jsonl")
			started = true
		}
		writeErr = controller.SetWriteDeadline(time.Now().Add(writeTimeout))
		if errors.Is(writeErr, http.ErrNotSupported) {
			writeErr = nil
		}
		if writeErr == nil {
			writeErr = enc.Encode(message)
		}
		if writeErr == nil {
			writeErr = controller.Flush()
		}
		_ = controller.SetWriteDeadline(time.Time{})
		if writeErr != nil {
			cancel()
		}
	}
	ctx = pruneprogress.WithReporter(ctx, func(progress jsonstream.Message) {
		mu.Lock()
		defer mu.Unlock()
		write(progress)
	})
	report, err := run(ctx)
	mu.Lock()
	defer mu.Unlock()
	if writeErr != nil {
		// The response has already started; returning an error would append a
		// second, incompatible HTTP error body to the stream.
		return nil
	}
	if err != nil {
		if !started {
			return err
		}
		write(jsonstream.Message{Error: &jsonstream.Error{Message: err.Error()}})
		return nil
	}
	data, err := json.Marshal(report)
	if err != nil {
		if !started {
			return err
		}
		write(jsonstream.Message{Error: &jsonstream.Error{Message: err.Error()}})
		return nil
	}
	raw := json.RawMessage(data)
	write(jsonstream.Message{Aux: &raw})
	return nil
}
