package remotecontext

import (
	"context"
	"os"

	"github.com/containerd/log"
	"github.com/moby/go-archive"
	"github.com/moby/moby/v2/daemon/builder"
	"github.com/moby/moby/v2/daemon/builder/remotecontext/git"
)

// MakeGitContext returns a Context from gitURL that is cloned in a temporary directory.
func MakeGitContext(gitURL string) (builder.Source, error) {
	root, err := git.Clone(gitURL, git.WithIsolatedConfig(true))
	if err != nil {
		return nil, err
	}

	c, err := archive.Tar(root, archive.Uncompressed)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := c.Close(); err != nil {
			log.G(context.TODO()).WithFields(log.Fields{
				"error":  err,
				"action": "MakeGitContext",
				"module": "builder",
				"url":    gitURL,
			}).Error("error while closing git context")
		}
		if err := os.RemoveAll(root); err != nil {
			log.G(context.TODO()).WithFields(log.Fields{
				"error":  err,
				"action": "MakeGitContext",
				"module": "builder",
				"url":    gitURL,
			}).Error("error while removing path and children of root")
		}
	}()
	return FromArchive(c)
}
