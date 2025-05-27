package containerd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/containerd/containerd/v2/core/remotes/docker"
	remoteerrors "github.com/containerd/containerd/v2/core/remotes/errors"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
)

func translateRegistryError(ctx context.Context, err error) error {
	// Check for registry specific error
	var derrs docker.Errors
	if !errors.As(err, &derrs) {
		var remoteErr remoteerrors.ErrUnexpectedStatus
		if errors.As(err, &remoteErr) {
			if jerr := json.Unmarshal(remoteErr.Body, &derrs); jerr != nil {
				log.G(ctx).WithError(derrs).Debug("unable to unmarshal registry error")
				return fmt.Errorf("%w: %w", cerrdefs.ErrUnknown, err)
			}
		} else {
			var derr docker.Error
			if errors.As(err, &derr) {
				derrs = append(derrs, derr)
			} else {
				return err
			}
		}
	}
	var errs []error
	for _, err := range derrs {
		var derr docker.Error
		if errors.As(err, &derr) {
			var message string

			if derr.Message != "" {
				message = derr.Message
			} else {
				message = derr.Code.Message()
			}

			if detail, ok := derr.Detail.(string); ok {
				message = fmt.Sprintf("%s - %s", message, detail)
			}

			switch derr.Code {
			case docker.ErrorCodeUnsupported:
				err = cerrdefs.ErrNotImplemented.WithMessage(message)
			case docker.ErrorCodeUnauthorized:
				err = cerrdefs.ErrUnauthenticated.WithMessage(message)
			case docker.ErrorCodeDenied:
				err = cerrdefs.ErrPermissionDenied.WithMessage(message)
			case docker.ErrorCodeUnavailable:
				err = cerrdefs.ErrUnavailable.WithMessage(message)
			case docker.ErrorCodeTooManyRequests:
				err = cerrdefs.ErrResourceExhausted.WithMessage(message)
			default:
				err = cerrdefs.ErrUnknown.WithMessage(message)
			}
		} else {
			errs = append(errs, cerrdefs.ErrUnknown.WithMessage(err.Error()))
		}
		errs = append(errs, err)
	}
	switch len(errs) {
	case 0:
		err = cerrdefs.ErrUnknown.WithMessage(err.Error())
	case 1:
		err = errs[0]
	default:
		err = errors.Join(errs...)
	}
	return fmt.Errorf("error from registry: %w", err)
}
