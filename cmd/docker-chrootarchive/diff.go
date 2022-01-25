//go:build !windows
// +build !windows

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/containerd/containerd/pkg/userns"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/system"
)

// applyLayer is the entry-point for docker-applylayer on re-exec. This is not
// used on Windows as it does not support chroot, hence no point sandboxing
// through chroot and rexec.
func applyLayer() {

	var (
		tmpDir  string
		err     error
		options *archive.TarOptions
	)
	runtime.LockOSThread()
	flag.Parse()

	inUserns := userns.RunningInUserNS()
	if err := chroot(flag.Arg(0)); err != nil {
		fatal(err)
	}

	// We need to be able to set any perms
	oldmask, err := system.Umask(0)
	defer system.Umask(oldmask)
	if err != nil {
		fatal(err)
	}

	if err := json.Unmarshal([]byte(os.Getenv("OPT")), &options); err != nil {
		fatal(err)
	}

	if inUserns {
		options.InUserNS = true
	}

	if tmpDir, err = os.MkdirTemp("/", "temp-docker-extract"); err != nil {
		fatal(err)
	}

	os.Setenv("TMPDIR", tmpDir)
	size, err := archive.UnpackLayer("/", os.Stdin, options)
	os.RemoveAll(tmpDir)
	if err != nil {
		fatal(err)
	}

	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(chrootarchive.InternalApplyLayerResponse{LayerSize: size}); err != nil {
		fatal(fmt.Errorf("unable to encode layerSize JSON: %s", err))
	}

	if _, err := flush(os.Stdin); err != nil {
		fatal(err)
	}

	os.Exit(0)
}
