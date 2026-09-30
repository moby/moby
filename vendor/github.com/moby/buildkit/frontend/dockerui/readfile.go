package dockerui

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/v2/defaults"
	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/pkg/errors"
)

// maxFileSize bounds files that the builtin frontend loads into daemon memory.
// A frontend running over the gateway cannot read more than one gRPC message in
// a single request anyway, so the builtin frontend is held to the same size.
const maxFileSize = defaults.DefaultMaxRecvMsgSize

type fileTooLargeError struct {
	filename string
}

func (e *fileTooLargeError) Error() string {
	return fmt.Sprintf("%s exceeds maximum allowed size of %d bytes", e.filename, maxFileSize)
}

func isFileTooLarge(err error) bool {
	var e *fileTooLargeError
	return errors.As(err, &e)
}

// ReadFile reads filename from ref, refusing files larger than maxFileSize.
// Oversized files are rejected on their reported size so that nothing is read
// from them, and the read itself carries a range so that no more than
// maxFileSize+1 bytes are ever loaded even when the size could not be checked.
func ReadFile(ctx context.Context, ref client.Reference, filename string) ([]byte, error) {
	// stat failures are left to the read, which reports them properly
	if st, err := ref.StatFile(ctx, client.StatRequest{Path: filename}); err == nil && st.Size > maxFileSize {
		return nil, errors.WithStack(&fileTooLargeError{filename: filename})
	}
	dt, err := ref.ReadFile(ctx, client.ReadRequest{
		Filename: filename,
		Range: &client.FileRange{
			Length: maxFileSize + 1,
		},
	})
	if err != nil {
		return nil, err
	}
	if len(dt) > maxFileSize {
		return nil, errors.WithStack(&fileTooLargeError{filename: filename})
	}
	return dt, nil
}
