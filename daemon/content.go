package daemon

import (
	"os"
	"path/filepath"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/content/local"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/metadata"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"
)

func (daemon *Daemon) configureLocalContentStore() (content.Store, leases.Manager, error) {
	if err := os.MkdirAll(filepath.Join(daemon.root, "content"), 0700); err != nil {
		return nil, nil, errors.Wrap(err, "error creating dir for content store")
	}
	db, err := bbolt.Open(filepath.Join(daemon.root, "content", "metadata.db"), 0600, nil)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error opening bolt db for content metadata store")
	}
	cs, err := local.NewStore(filepath.Join(daemon.root, "content", "data"))
	if err != nil {
		return nil, nil, errors.Wrap(err, "error setting up content store")
	}
	md := metadata.NewDB(db, cs, nil)
	daemon.mdDB = db
	return md.ContentStore(), metadata.NewLeaseManager(md), nil
}
