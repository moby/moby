package errdefs

import (
	"fmt"

	"google.golang.org/grpc/codes"
)

type UnknownJobError struct {
	id string
}

func (e *UnknownJobError) Code() codes.Code {
	return codes.NotFound
}

func (e *UnknownJobError) Error() string {
	return fmt.Sprintf("no such job %s", e.id)
}

func NewUnknownJobError(id string) error {
	return &UnknownJobError{id: id}
}
