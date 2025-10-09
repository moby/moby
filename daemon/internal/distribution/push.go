package distribution

import (
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/containerd/log"
	"github.com/distribution/reference"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/v2/daemon/internal/progress"
)

const compressionBufSize = 32768

// Push initiates a push operation on ref. ref is the specific variant of the
// image to push. If no tag is provided, all tags are pushed.
func Push(ctx context.Context, ref reference.Named, config *ImagePushConfig) error {
	// FIXME: Allow to interrupt current push when new push of same image is done.

	repoName := reference.TrimNamed(ref)

	endpoints, err := config.RegistryService.LookupPushEndpoints(reference.Domain(repoName))
	if err != nil {
		return err
	}

	progress.Messagef(config.ProgressOutput, "", "The push refers to repository [%s]", repoName.Name())

	associations := config.ReferenceStore.ReferencesByName(repoName)
	if len(associations) == 0 {
		return fmt.Errorf("An image does not exist locally with the tag: %s", reference.FamiliarName(repoName))
	}

	var (
		lastErr error

		// confirmedTLSRegistries is a map indicating which registries
		// are known to be using TLS. There should never be a plaintext
		// retry for any of these.
		confirmedTLSRegistries = make(map[string]struct{})
	)

	for _, endpoint := range endpoints {
		if endpoint.URL.Scheme != "https" {
			if _, confirmedTLS := confirmedTLSRegistries[endpoint.URL.Host]; confirmedTLS {
				log.G(ctx).Debugf("Skipping non-TLS endpoint %s for host/port that appears to use TLS", endpoint.URL)
				continue
			}
		}

		log.G(ctx).Debugf("Trying to push %s to %s", repoName.Name(), endpoint.URL)

		if err := newPusher(ref, endpoint, repoName, config).push(ctx); err != nil {
			// Was this push cancelled? If so, don't try to fall
			// back.
			select {
			case <-ctx.Done():
			default:
				if fallbackErr, ok := err.(fallbackError); ok {
					if fallbackErr.transportOK && endpoint.URL.Scheme == "https" {
						confirmedTLSRegistries[endpoint.URL.Host] = struct{}{}
					}
					err = fallbackErr.err
					lastErr = err
					log.G(ctx).Infof("Attempting next endpoint for push after error: %v", err)
					continue
				}
			}

			// FIXME(thaJeztah): cleanup error and context handling in this package, as it's really messy.
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				log.G(ctx).WithError(err).Info("Not continuing with push after error")
			} else {
				log.G(ctx).WithError(err).Error("Not continuing with push after error")
			}
			return err
		}

		config.ImageEventLogger(ctx, reference.FamiliarString(ref), reference.FamiliarName(repoName), events.ActionPush)
		return nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no endpoints found for %s", repoName.Name())
	}
	return lastErr
}

// compress returns an io.ReadCloser which will supply a compressed version of
// the provided Reader. The caller must close the ReadCloser after reading the
// compressed data.
//
// Note that this function returns a reader instead of taking a writer as an
// argument so that it can be used with httpBlobWriter's ReadFrom method.
// Using httpBlobWriter's Write method would send a PATCH request for every
// Write call.
//
// The second return value is a channel that gets closed when the goroutine
// is finished. This allows the caller to make sure the goroutine finishes
// before it releases any resources connected with the reader that was
// passed in.
func compress(in io.Reader) (io.ReadCloser, chan struct{}) {
	compressionDone := make(chan struct{})

	pipeReader, pipeWriter := io.Pipe()
	// Use a bufio.Writer to avoid excessive chunking in HTTP request.
	bufWriter := bufio.NewWriterSize(pipeWriter, compressionBufSize)
	compressor := gzip.NewWriter(bufWriter)

	go func() {
		_, err := io.Copy(compressor, in)
		if err == nil {
			err = compressor.Close()
		}
		if err == nil {
			err = bufWriter.Flush()
		}
		if err != nil {
			pipeWriter.CloseWithError(err)
		} else {
			pipeWriter.Close()
		}
		close(compressionDone)
	}()

	return pipeReader, compressionDone
}
