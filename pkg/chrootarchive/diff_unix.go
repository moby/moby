//go:build !windows
// +build !windows

package chrootarchive // import "github.com/docker/docker/pkg/chrootarchive"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/containerd/containerd/pkg/userns"
	"github.com/docker/docker/pkg/archive"
)

// InternalApplyLayerResponse is an internal type but exported because it is
// cross-package; it is part of the implementation of the "docker-chrootarchive"
// command.
type InternalApplyLayerResponse struct {
	LayerSize int64 `json:"layerSize"`
}

// applyLayerHandler parses a diff in the standard layer format from `layer`, and
// applies it to the directory `dest`. Returns the size in bytes of the
// contents of the layer.
func applyLayerHandler(dest string, layer io.Reader, options *archive.TarOptions, decompress bool) (size int64, err error) {
	dest = filepath.Clean(dest)
	if decompress {
		decompressed, err := archive.DecompressStream(layer)
		if err != nil {
			return 0, err
		}
		defer decompressed.Close()

		layer = decompressed
	}
	if options == nil {
		options = &archive.TarOptions{}
		if userns.RunningInUserNS() {
			options.InUserNS = true
		}
	}
	if options.ExcludePatterns == nil {
		options.ExcludePatterns = []string{}
	}

	data, err := json.Marshal(options)
	if err != nil {
		return 0, fmt.Errorf("ApplyLayer json encode: %v", err)
	}

	cmd := command("docker-applyLayer", dest)
	cmd.Stdin = layer
	cmd.Env = append(cmd.Env, fmt.Sprintf("OPT=%s", data))

	outBuf, errBuf := new(bytes.Buffer), new(bytes.Buffer)
	cmd.Stdout, cmd.Stderr = outBuf, errBuf

	if err = cmd.Run(); err != nil {
		return 0, fmt.Errorf("ApplyLayer %s stdout: %s stderr: %s", err, outBuf, errBuf)
	}

	// Stdout should be a valid JSON struct representing an InternalApplyLayerResponse.
	response := InternalApplyLayerResponse{}
	decoder := json.NewDecoder(outBuf)
	if err = decoder.Decode(&response); err != nil {
		return 0, fmt.Errorf("unable to decode ApplyLayer JSON response: %s", err)
	}

	return response.LayerSize, nil
}
