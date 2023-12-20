package filelease

import (
	"os"
	"time"

	"github.com/docker/docker/libnetwork/types"

	"golang.org/x/sys/unix"
)

func getWriteLease(f *os.File, opts *Opts) (retErr error) {
	for retry, attempt := true, 0; retry && attempt < opts.attempts; attempt += 1 {
		if attempt > 0 {
			time.Sleep(opts.interval)
		}
		_, retErr = unix.FcntlInt(f.Fd(), unix.F_SETLEASE, unix.F_WRLCK)
		retry = retErr == unix.EAGAIN
	}
	if retErr == unix.EINVAL {
		return types.NotImplementedErrorf("not implemented")
	}
	return retErr
}
