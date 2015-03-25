// +build !linux

package graph

import (
	"fmt"

	"github.com/docker/docker/engine"
)

func (s *TagStore) CmdLoad(job *engine.Job) error {
	return fmt.Errorf("CmdLoad is not supported on this platform")
}
