package libcontainerd // import "github.com/docker/docker/libcontainerd"

import "fmt"

// WithOOMScore defines the oom_score_adj to set for the containerd process.
func WithOOMScore(score int) RemoteOption {
	return oomScore(score)
}

type oomScore int

func (o oomScore) Apply(r Remote) error {
	if remote, ok := r.(*remote); ok {
		remote.OOMScore = int(o)
		return nil
	}
	return fmt.Errorf("WithOOMScore option not supported for this remote")
}
