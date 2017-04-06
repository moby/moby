package swarm

import (
	"fmt"
	"net/http"

	"github.com/docker/docker/api/server/httputils"
	basictypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/pkg/stdcopy"
	"golang.org/x/net/context"
)

// swarmLogs takes an http response, request, and selector, and writes the logs
// specified by the selector to the response
func (sr *swarmRouter) swarmLogs(ctx context.Context, w http.ResponseWriter, r *http.Request, selector *backend.LogSelector) error {
	// Args are validated before the stream starts because when it starts we're
	// sending HTTP 200 by writing an empty chunk of data to tell the client that
	// daemon is going to stream. By sending this initial HTTP 200 we can't report
	// any error after the stream starts (i.e. container not found, wrong parameters)
	// with the appropriate status code.
	stdout, stderr := httputils.BoolValue(r, "stdout"), httputils.BoolValue(r, "stderr")
	if !(stdout || stderr) {
		return fmt.Errorf("Bad parameters: you must choose at least one stream")
	}

	logsConfig := &backend.ContainerLogsConfig{
		ContainerLogsOptions: basictypes.ContainerLogsOptions{
			Follow:     httputils.BoolValue(r, "follow"),
			Timestamps: httputils.BoolValue(r, "timestamps"),
			Since:      r.Form.Get("since"),
			Tail:       r.Form.Get("tail"),
			ShowStdout: stdout,
			ShowStderr: stderr,
			Details:    httputils.BoolValue(r, "details"),
		},
		OutStream: w,
	}

	chStarted := make(chan struct{})
	if err := sr.backend.ServiceLogs(ctx, selector, logsConfig, chStarted); err != nil {
		select {
		case <-chStarted:
			// The client may be expecting all of the data we're sending to
			// be multiplexed, so send it through OutStream, which will
			// have been set up to handle that if needed.
			stdwriter := stdcopy.NewStdWriter(w, stdcopy.Systemerr)
			fmt.Fprintf(stdwriter, "Error grabbing service logs: %v\n", err)
		default:
			return err
		}
	}

	return nil
}
