// +build !windows

package idtools

import (
	"github.com/cpuguy83/idmapfs"
	"github.com/cpuguy83/idmapfs/idtools"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	"github.com/sirupsen/logrus"
)

// MapFS creates a bind mount from the source to the target location with the
// ownership of files and folders mapped to the container namespaced ID's.
func MapFS(m *IdentityMapping, source, target string, options []string) (func() error, error) {
	var uids []idtools.IDMap
	for _, id := range m.UIDs() {
		uids = append(uids, idtools.IDMap(id))
	}

	var gids []idtools.IDMap
	for _, id := range m.GIDs() {
		gids = append(gids, idtools.IDMap(id))
	}

	idmap := idtools.NewIDMappingsFromMaps(uids, gids)

	logger := logrus.StandardLogger() // TODO: Make configurable
	fs := idmapfs.New(pathfs.NewLoopbackFileSystem(source),
		idmap,
		"idmapfs",
		logger.WriterLevel(logrus.DebugLevel),
	)

	enableDebug := logrus.GetLevel() >= logrus.DebugLevel
	fs.SetDebug(enableDebug)

	opts := nodefs.NewOptions()
	opts.Owner = nil
	opts.Debug = enableDebug

	conn := nodefs.NewFileSystemConnector(pathfs.NewPathNodeFs(fs, &pathfs.PathNodeFsOptions{Debug: enableDebug}).Root(), opts)
	srv, err := fuse.NewServer(conn.RawFS(), target, &fuse.MountOptions{
		AllowOther: true,
		Name:       "idmapfs",
		FsName:     "idmapfs",
		Options:    options,
	})
	if err != nil {
		return nil, err
	}

	go srv.Serve()

	return srv.Unmount, nil
}
