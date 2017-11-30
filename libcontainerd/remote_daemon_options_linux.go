package libcontainerd

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

// WithSubreaper sets whether containerd should register itself as a
// subreaper
func WithSubreaper(reap bool) RemoteOption {
	return subreaper(reap)
}

type subreaper bool

func (s subreaper) Apply(r Remote) error {
	if remote, ok := r.(*remote); ok {
		remote.NoSubreaper = !bool(s)
		return nil
	}
	return fmt.Errorf("WithSubreaper option not supported for this remote")
}
