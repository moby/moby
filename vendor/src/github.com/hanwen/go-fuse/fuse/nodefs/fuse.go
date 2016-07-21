package nodefs

import (
	"github.com/hanwen/go-fuse/fuse"
)

// Mounts a filesystem with the given root node on the given directory
func MountRoot(mountpoint string, root Node, opts *Options) (*fuse.Server, *FileSystemConnector, error) {
	conn := NewFileSystemConnector(root, opts)

	mountOpts := fuse.MountOptions{}
	if opts != nil && opts.Debug {
		mountOpts.Debug = opts.Debug
	}
	s, err := fuse.NewServer(conn.RawFS(), mountpoint, &mountOpts)
	if err != nil {
		return nil, nil, err
	}
	return s, conn, nil
}
