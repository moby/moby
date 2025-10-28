package utils

import (
	"context"
	"errors"
	"io"
	"syscall"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/internal/progress"
	"github.com/moby/moby/v2/daemon/internal/streamformatter"
)

// WriteDistributionProgress is a helper for writing progress from chan to JSON
// stream with an optional cancel function.
func WriteDistributionProgress(cancelFunc func(), outStream io.Writer, progressChan <-chan progress.Progress) {
	progressOutput := streamformatter.NewJSONProgressOutput(outStream, false)
	operationCancelled := false

	for prog := range progressChan {
		if err := progressOutput.WriteProgress(prog); err != nil && !operationCancelled {
			// don't log broken pipe errors as this is the normal case when a client aborts
			if errors.Is(err, syscall.EPIPE) {
				log.G(context.TODO()).Info("Pull session cancelled")
			} else {
				log.G(context.TODO()).Errorf("error writing progress to client: %v", err)
			}
			cancelFunc()
			operationCancelled = true
			// Don't return, because we need to continue draining
			// progressChan until it's closed to avoid a deadlock.
		}
	}
}
