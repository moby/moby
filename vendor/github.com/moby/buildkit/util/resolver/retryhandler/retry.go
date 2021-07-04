package retryhandler

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd/images"
	remoteserrors "github.com/containerd/containerd/remotes/errors"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func New(f images.HandlerFunc, logger func([]byte)) images.HandlerFunc {
	return func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
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
			if backoff >= 8*time.Second {
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

	if errors.Is(err, io.EOF) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) {
		return true
	}
	// catches TLS timeout or other network-related temporary errors
	if ne, ok := errors.Cause(err).(net.Error); ok && ne.Temporary() {
		return true
	}
	// https://github.com/containerd/containerd/pull/4724
	if errors.Cause(err).Error() == "no response" {
		return true
	}

	// net.ErrClosed exposed in go1.16
	if strings.Contains(err.Error(), "use of closed network connection") {
		return true
	}

	return false
}
