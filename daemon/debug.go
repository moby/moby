package daemon

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/pkg/errors"
)

// SupportDump returns a tar with details often required to debug user issues.
// The format of this tar is not stable and is intended for human consumption.
func (daemon *Daemon) SupportDump(ctx context.Context) (io.Reader, error) {
	buf := bytes.NewBuffer(nil)
	w := tar.NewWriter(buf)
	defer w.Close()
	defer w.Flush()

	stacksBuf := make([]byte, 16384)
	n := runtime.Stack(stacksBuf, true)

	err := w.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     filepath.Base(os.Args[0]) + "-goroutines.txt",
		ModTime:  time.Now(),
		Size:     int64(n),
		Mode:     0644,
	})
	if err != nil {
		return nil, errors.Wrap(err, "error writing tar header for daemon goroutines")
	}

	if _, err := w.Write(stacksBuf[:n]); err != nil {
		return nil, errors.Wrap(err, "error writing daemon goroutine dump to tar")
	}

	version, err := json.MarshalIndent(daemon.SystemVersion(), "", "\t")
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling daemon version")
	}

	err = w.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "version.txt",
		ModTime:  time.Now(),
		Size:     int64(len(version)),
		Mode:     0644,
	})
	if err != nil {
		return nil, errors.Wrap(err, "error writing tar header for version.txt")
	}

	if _, err := w.Write(version); err != nil {
		return nil, errors.Wrap(err, "error writing version file to tar")
	}

	info, err := daemon.SystemInfo()
	if err != nil {
		return nil, errors.Wrap(err, "error getting system info")
	}
	infoB, err := json.MarshalIndent(info, "", "\t")
	if err != nil {
		return nil, errors.Wrap(err, "error marshaling daemon info")
	}

	err = w.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "daemoninfo.txt",
		ModTime:  time.Now(),
		Size:     int64(len(infoB)),
		Mode:     0644,
	})
	if err != nil {
		return nil, errors.Wrap(err, "error writing info tar header")
	}
	if _, err := w.Write(infoB); err != nil {
		return nil, errors.Wrap(err, "error writing daemon info to tar")
	}

	// TODO(@cpuguy83): Add containerd stack dump

	return buf, nil
}
