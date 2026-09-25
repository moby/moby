//go:build !windows

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package v2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/containerd/containerd/v2/defaults"
	"github.com/containerd/fifo"
	"golang.org/x/sys/unix"
)

func openShimLog(ctx context.Context, bundle *Bundle, _ func(string, time.Duration) (net.Conn, error)) (io.ReadCloser, error) {
	return fifo.OpenFifo(ctx, filepath.Join(bundle.Path, "log"), unix.O_RDWR|unix.O_CREAT|unix.O_NONBLOCK, 0700)
}

func checkCopyShimLogError(ctx context.Context, err error) error {
	select {
	case <-ctx.Done():
		if err == fifo.ErrReadClosed || errors.Is(err, os.ErrClosed) {
			return nil
		}
	default:
	}
	return err
}

// defaultSocketDir returns the directory used for shim unix sockets.
// The path is intentionally kept short and hardcoded rather than derived
// from the configured state directory, because unix socket paths are
// limited to 104-108 characters. If no suitable path can be found,
// an empty string is returned.
//
// The selection order is:
//  1. The default state directory (/run/containerd/s) — try to create it,
//     and if it already exists verify the current user owns it.
//  2. $XDG_RUNTIME_DIR/containerd/s — used if XDG_RUNTIME_DIR is set
//     and owned by the current user.
//  3. /run/<UID>/containerd/s — used if the directory /run/<UID>
//     exists and is owned by the current user.
//  4. /tmp/containerd-s-<UID> — created and ownership-verified as a last resort.
func defaultSocketDir() string {
	defaultDir := filepath.Join(defaults.DefaultStateDir, "s")
	uid := os.Geteuid()
	if uid == 0 {
		return defaultDir
	}

	candidates := []string{defaultDir}
	if xdgDir := os.Getenv("XDG_RUNTIME_DIR"); xdgDir != "" {
		candidates = append(candidates, filepath.Join(xdgDir, "containerd", "s"))
	}
	candidates = append(candidates,
		fmt.Sprintf("/run/%d/containerd/s", uid),
		fmt.Sprintf("/tmp/containerd-s-%d", uid),
	)

	for _, dir := range candidates {
		if len(dir) <= maxSocketDirLen {
			if ensureSocketDir(dir, uid) {
				return dir
			}
		}
	}

	// All candidates failed, return empty string and let caller handle
	return ""
}

// ensureSocketDir attempts to create dir with mode 0700, verifies it is
// owned by uid, and corrects the permissions to 0700 if they differ.
// Returns true if the directory is ready for use.
func ensureSocketDir(dir string, uid int) bool {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return false
	}
	st, err := os.Lstat(dir)
	if err != nil {
		return false
	}
	if !st.IsDir() {
		return false
	}
	sys := st.Sys()
	if sys == nil {
		return false
	}
	stat, ok := sys.(*syscall.Stat_t)
	if !ok {
		return false
	}
	if int(stat.Uid) != uid {
		return false
	}
	if st.Mode().Perm() != 0700 {
		if err := os.Chmod(dir, 0700); err != nil {
			return false
		}
	}
	return true
}
