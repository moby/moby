package filesync

import (
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/tonistiigi/fsutil"
	"google.golang.org/grpc"
)

func sendDiffCopy(stream grpc.Stream, dir string, includes, excludes []string, progress progressCb) error {
	return fsutil.Send(stream.Context(), stream, dir, &fsutil.WalkOpt{
		ExcludePatterns: excludes,
		IncludePaths:    includes, // TODO: rename IncludePatterns
	}, progress)
}

func recvDiffCopy(ds grpc.Stream, dest string, cu CacheUpdater, progress progressCb) error {
	st := time.Now()
	defer func() {
		logrus.Debugf("diffcopy took: %v", time.Since(st))
	}()
	var cf fsutil.ChangeFunc
	var ch fsutil.ContentHasher
	if cu != nil {
		cu.MarkSupported(true)
		cf = cu.HandleChange
		ch = cu.ContentHasher()
	}
	return fsutil.Receive(ds.Context(), ds, dest, fsutil.ReceiveOpt{
		NotifyHashed:  cf,
		ContentHasher: ch,
		ProgressCb:    progress,
	})
}

func syncTargetDiffCopy(ds grpc.Stream, dest string) error {
	if err := os.MkdirAll(dest, 0700); err != nil {
		return err
	}
	return fsutil.Receive(ds.Context(), ds, dest, fsutil.ReceiveOpt{
		Merge: true,
		Filter: func() func(*fsutil.Stat) bool {
			uid := os.Getuid()
			gid := os.Getgid()
			return func(st *fsutil.Stat) bool {
				st.Uid = uint32(uid)
				st.Gid = uint32(gid)
				return true
			}
		}(),
	})
}
