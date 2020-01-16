package supervisor // import "github.com/moby/moby/libcontainerd/supervisor"

// WithOOMScore defines the oom_score_adj to set for the containerd process.
func WithOOMScore(score int) DaemonOpt {
	return func(r *remote) error {
		r.OOMScore = score
		return nil
	}
}
