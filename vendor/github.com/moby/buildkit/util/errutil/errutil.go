package errutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/containerd/containerd/v2/core/remotes/docker"
	remoteserrors "github.com/containerd/containerd/v2/core/remotes/errors"
)

const (
	maxPrintedBodySize = 256
)

func WithDetails(err error) error {
	if err == nil {
		return nil
	}
	var errStatus remoteserrors.ErrUnexpectedStatus
	if errors.As(err, &errStatus) {
		var dErr docker.Errors
		if err1 := json.Unmarshal(errStatus.Body, &dErr); err1 == nil && len(dErr) > 0 {
			return &formattedDockerError{dErr: dErr}
		}

		return verboseUnexpectedStatusError{ErrUnexpectedStatus: errStatus}
	}
	return err
}

type verboseUnexpectedStatusError struct {
	remoteserrors.ErrUnexpectedStatus
}

func (e verboseUnexpectedStatusError) Unwrap() error {
	return e.ErrUnexpectedStatus
}

func (e verboseUnexpectedStatusError) Error() string {
	if len(e.Body) == 0 {
		return e.ErrUnexpectedStatus.Error()
	}
	var details string

	var errDetails struct {
		Details string `json:"details"`
	}

	if err := json.Unmarshal(e.Body, &errDetails); err == nil && errDetails.Details != "" {
		details = errDetails.Details
	} else {
		if len(e.Body) > maxPrintedBodySize {
			details = string(e.Body[:maxPrintedBodySize]) + fmt.Sprintf("... (%d bytes truncated)", len(e.Body)-maxPrintedBodySize)
		} else {
			details = string(e.Body)
		}
	}

	return fmt.Sprintf("%s: %s", e.ErrUnexpectedStatus.Error(), details)
}

type formattedDockerError struct {
	dErr docker.Errors
}

func (e *formattedDockerError) Error() string {
	format := func(err error) string {
		out := err.Error()
		var dErr docker.Error
		if errors.As(err, &dErr) {
			if v, ok := dErr.Detail.(string); ok && v != "" {
				out += " - " + v
			}
		}
		return out
	}
	switch len(e.dErr) {
	case 0:
		return "<nil>"
	case 1:
		return format(e.dErr[0])
	default:
		var msg strings.Builder
		msg.WriteString("errors:\n")
		for _, err := range e.dErr {
			msg.WriteString(format(err))
			msg.WriteByte('\n')
		}
		return msg.String()
	}
}

func (e *formattedDockerError) Unwrap() error {
	return e.dErr
}
