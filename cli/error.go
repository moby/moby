package cli // import "github.com/docker/docker/cli"

import (
	"fmt"
)

// StatusError reports an unsuccessful exit by a command.
type StatusError struct {
	Status     string
	StatusCode int
}

func (e StatusError) Error() string {
	return fmt.Sprintf("Status: %s, Code: %d", e.Status, e.StatusCode)
}
