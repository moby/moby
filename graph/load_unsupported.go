// +build !linux

package graph

import (
	"github.com/docker/docker/engine"
)

func (s *TagStore) CmdLoad(job *engine.Job) engine.Status {
	return job.Errorf("CmdLoad is not supported on this platform")
}
