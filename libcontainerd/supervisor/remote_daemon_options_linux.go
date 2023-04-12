package supervisor // import "github.com/docker/docker/libcontainerd/supervisor"

// WithOOMScore defines the oom_score_adj to set for the containerd process.
//
// Deprecated: setting the oom-score-adjust from the daemon itself is deprecated, and should be handled by the process-manager starting the daemon instead.
func WithOOMScore(score int) DaemonOpt {
	return func(r *remote) error {
		r.oomScore = score
		return nil
	}
}
