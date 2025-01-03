package retryhandler

import (
	"context"
	"fmt"
	"io"
	"net"
	"syscall"
	"time"

	"github.com/containerd/containerd/v2/core/images"
	remoteserrors "github.com/containerd/containerd/v2/core/remotes/errors"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// MaxRetryBackoff is the maximum backoff time before giving up. This is a
// variable so that code which embeds BuildKit can override the default value.
var MaxRetryBackoff = 8 * time.Second

func New(f images.HandlerFunc, logger func([]byte)) images.HandlerFunc {
	return func(ctx context.Context, desc ocispecs.Descriptor) ([]ocispecs.Descriptor, error) {
		backoff := time.Second
		for {
			descs, err := f(ctx, desc)
			if err != nil {
				select {
				case <-ctx.Done():
					return nil, err
				default:
					if !retryError(err) {
						return nil, err
					}
				}
				if logger != nil {
					logger([]byte(fmt.Sprintf("error: %v\n", err.Error())))
				}
			} else {
				return descs, nil
			}
			// backoff logic
			if backoff >= MaxRetryBackoff {
				return nil, err
			}
			if logger != nil {
				logger([]byte(fmt.Sprintf("retrying in %v\n", backoff)))
			}
			time.Sleep(backoff)
			backoff *= 2
		}
	}
}

func retryError(err error) bool {
	// Retry on 5xx errors
	var errUnexpectedStatus remoteserrors.ErrUnexpectedStatus
	if errors.As(err, &errUnexpectedStatus) &&
		errUnexpectedStatus.StatusCode >= 500 &&
		errUnexpectedStatus.StatusCode <= 599 {
		return true
	}

	if errors.Is(err, io.EOF) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) || errors.Is(err, net.ErrClosed) {
		return true
	}
	// catches TLS timeout or other network-related temporary errors
	if ne := net.Error(nil); errors.As(err, &ne) && ne.Temporary() { //nolint:staticcheck // ignoring "SA1019: Temporary is deprecated", continue to propagate net.Error through the "temporary" status
		return true
	}

	return false
}
