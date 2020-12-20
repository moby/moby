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

func (d *Daemon) configureLocalContentStore() (content.Store, leases.Manager, error) {
	if err := os.MkdirAll(filepath.Join(d.root, "content"), 0700); err != nil {
		return nil, nil, errors.Wrap(err, "error creating dir for content store")
	}
	db, err := bbolt.Open(filepath.Join(d.root, "content", "metadata.db"), 0600, nil)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error opening bolt db for content metadata store")
	}
	cs, err := local.NewStore(filepath.Join(d.root, "content", "data"))
	if err != nil {
		return nil, nil, errors.Wrap(err, "error setting up content store")
	}
	md := metadata.NewDB(db, cs, nil)
	d.mdDB = db
	return md.ContentStore(), metadata.NewLeaseManager(md), nil
}
