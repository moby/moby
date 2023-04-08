package supervisor // import "github.com/docker/docker/libcontainerd/supervisor"
import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// WithLogLevel defines which log level to start containerd with.
func WithLogLevel(lvl string) DaemonOpt {
	return func(r *remote) error {
		if lvl == "info" {
			// both dockerd and containerd default log-level is "info",
			// so don't pass the default.
			lvl = ""
		}
		r.logLevel = lvl
		return nil
	}
}

// WithCRIDisabled disables the CRI plugin.
func WithCRIDisabled() DaemonOpt {
	return func(r *remote) error {
		r.DisabledPlugins = append(r.DisabledPlugins, "io.containerd.grpc.v1.cri")
		return nil
	}
}

// WithDetectLocalBinary checks if a containerd binary is present in the same
// directory as the dockerd binary, and overrides the path of the containerd
// binary to start if found. If no binary is found, no changes are made.
func WithDetectLocalBinary() DaemonOpt {
	return func(r *remote) error {
		dockerdPath, err := os.Executable()
		if err != nil {
			return errors.Wrap(err, "looking up binary path")
		}

		localBinary := filepath.Join(filepath.Dir(dockerdPath), binaryName)
		fi, err := os.Stat(localBinary)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			return nil
		}
		if fi.IsDir() {
			return errors.Errorf("local containerd path found (%s), but is a directory", localBinary)
		}
		r.daemonPath = localBinary
		r.logger.WithError(err).WithField("binary", localBinary).Debug("failed to look up local containerd binary; using default binary")
		return nil
	}
}
