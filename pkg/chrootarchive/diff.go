package chrootarchive

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/docker/pkg/system"
)

type applyLayerResponse struct {
	LayerSize int64 `json:"layerSize"`
}

func applyLayer() {

	var (
		root   = "/"
		tmpDir = ""
		err    error
	)

	runtime.LockOSThread()
	flag.Parse()

	if runtime.GOOS != "windows" {
		if err := chroot(flag.Arg(0)); err != nil {
			fatal(err)
		}

		// We need to be able to set any perms
		oldmask, err := system.Umask(0)
		defer system.Umask(oldmask)
		if err != nil {
			fatal(err)
		}
	} else {
		// As Windows does not support chroot or umask, we use the directory
		// passed in which will be <pathto>\docker-buildnnnnnnnn instead of
		// the 'chroot-root', "/"
		root = flag.Arg(0)
	}

	if tmpDir, err = ioutil.TempDir(root, "temp-docker-extract"); err != nil {
		fatal(err)
	}

	os.Setenv("TMPDIR", tmpDir)
	size, err := archive.UnpackLayer(root, os.Stdin)
	os.RemoveAll(tmpDir)
	if err != nil {
		fatal(err)
	}

	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(applyLayerResponse{size}); err != nil {
		fatal(fmt.Errorf("unable to encode layerSize JSON: %s", err))
	}

	flush(os.Stdout)
	flush(os.Stdin)
	os.Exit(0)
}

func ApplyLayer(dest string, layer archive.ArchiveReader) (size int64, err error) {
	dest = filepath.Clean(dest)
	decompressed, err := archive.DecompressStream(layer)
	if err != nil {
		return 0, err
	}

	defer decompressed.Close()

	cmd := reexec.Command("docker-applyLayer", dest)
	cmd.Stdin = decompressed

	outBuf, errBuf := new(bytes.Buffer), new(bytes.Buffer)
	cmd.Stdout, cmd.Stderr = outBuf, errBuf

	if err = cmd.Run(); err != nil {
		return 0, fmt.Errorf("ApplyLayer %s stdout: %s stderr: %s", err, outBuf, errBuf)
	}

	// Stdout should be a valid JSON struct representing an applyLayerResponse.
	response := applyLayerResponse{}
	decoder := json.NewDecoder(outBuf)
	if err = decoder.Decode(&response); err != nil {
		return 0, fmt.Errorf("unable to decode ApplyLayer JSON response: %s", err)
	}

	return response.LayerSize, nil
}
