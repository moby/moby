package filesync

import (
	"time"

	"google.golang.org/grpc"

	"github.com/Sirupsen/logrus"
	"github.com/tonistiigi/fsutil"
)

func sendDiffCopy(stream grpc.Stream, dir string, excludes []string, progress progressCb) error {
	return fsutil.Send(stream.Context(), stream, dir, &fsutil.WalkOpt{
		ExcludePatterns: excludes,
	}, progress)
}

func recvDiffCopy(ds grpc.Stream, dest string, cu CacheUpdater) error {
	st := time.Now()
	defer func() {
		logrus.Debugf("diffcopy took: %v", time.Since(st))
	}()
	var cf fsutil.ChangeFunc
	if cu != nil {
		cu.MarkSupported(true)
		cf = cu.HandleChange
	}

	return fsutil.Receive(ds.Context(), ds, dest, cf)
}
