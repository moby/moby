package distribution // import "github.com/docker/docker/distribution"

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/pkg/progress"
	"github.com/sirupsen/logrus"
)

const compressionBufSize = 32768

// Push initiates a push operation on ref. ref is the specific variant of the
// image to push. If no tag is provided, all tags are pushed.
func Push(ctx context.Context, ref reference.Named, config *ImagePushConfig) error {
	// FIXME: Allow to interrupt current push when new push of same image is done.

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := config.RegistryService.ResolveRepository(ref)
	if err != nil {
		return err
	}

	endpoints, err := config.RegistryService.LookupPushEndpoints(reference.Domain(repoInfo.Name))
	if err != nil {
		return err
	}

	progress.Messagef(config.ProgressOutput, "", "The push refers to repository [%s]", repoInfo.Name.Name())

	associations := config.ReferenceStore.ReferencesByName(repoInfo.Name)
	if len(associations) == 0 {
		return fmt.Errorf("An image does not exist locally with the tag: %s", reference.FamiliarName(repoInfo.Name))
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
				logrus.Debugf("Skipping non-TLS endpoint %s for host/port that appears to use TLS", endpoint.URL)
				continue
			}
		}

		logrus.Debugf("Trying to push %s to %s", repoInfo.Name.Name(), endpoint.URL)

		if err := newPusher(ref, endpoint, repoInfo, config).push(ctx); err != nil {
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
					logrus.Infof("Attempting next endpoint for push after error: %v", err)
					continue
				}
			}

			logrus.Errorf("Not continuing with push after error: %v", err)
			return err
		}

		config.ImageEventLogger(reference.FamiliarString(ref), reference.FamiliarName(repoInfo.Name), "push")
		return nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no endpoints found for %s", repoInfo.Name.Name())
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
	var usePigz bool
	noPigzEnv := os.Getenv("MOBY_DISABLE_PIGZ")
	if noPigzEnv == "" {
		usePigz = true
	} else {
		noPigz, err := strconv.ParseBool(noPigzEnv)
		if err != nil {
			logrus.WithError(err).Warn("invalid value in MOBY_DISABLE_PIGZ env var")
		}
		usePigz = !noPigz
	}

	if usePigz {
		pigzPath, err := exec.LookPath("pigz")
		if err != nil {
			logrus.Debugf("pigz binary not found, falling back to go gzip library")
		} else {
			return pigzCompress(ctx, in, pigzPath)
		}
	} else {
		logrus.Debugf("Use of pigz is disabled due to MOBY_DISABLE_PIGZ=%s", noPigzEnv)
	}

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

func pigzCompress(ctx context.Context, in io.Reader, pigzPath string) (io.ReadCloser, chan struct{}) {
	cmd := exec.CommandContext(ctx, pigzPath, "-c")

	cmd.Stdin = in
	pipeR, pipeW := io.Pipe()
	cmd.Stdout = pipeW
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	done := make(chan struct{})

	if err := cmd.Start(); err != nil {
		fmt.Errorf("could not get compression stream: %v", err)
		close(done)
		return nil, done
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			pipeW.CloseWithError(fmt.Errorf("%s: %s", err, errBuf.String()))
		} else {
			pipeW.Close()
		}
		close(done)
	}()

	return pipeR, done
}