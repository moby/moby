// +build !windows

package graph

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/archive"
	"github.com/vbatts/tar-split/tar/asm"
	"github.com/vbatts/tar-split/tar/storage"
)

// allowBaseParentImage allows images to define a custom parent that is not
// transported with push/pull but already included with the installation.
// Only used in Windows.
const allowBaseParentImage = false

func (graph *Graph) disassembleAndApplyTarLayer(id, parent string, layerData io.Reader, root string) (int64, error) {
	// this is saving the tar-split metadata
	mf, err := os.OpenFile(filepath.Join(root, tarDataFileName), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(0600))
	if err != nil {
		return 0, err
	}

	mfz := gzip.NewWriter(mf)
	metaPacker := storage.NewJSONPacker(mfz)
	defer mf.Close()
	defer mfz.Close()

	inflatedLayerData, err := archive.DecompressStream(layerData)
	if err != nil {
		return 0, err
	}

	// we're passing nil here for the file putter, because the ApplyDiff will
	// handle the extraction of the archive
	rdr, err := asm.NewInputTarStream(inflatedLayerData, metaPacker, nil)
	if err != nil {
		return 0, err
	}

	return graph.driver.ApplyDiff(id, parent, archive.Reader(rdr))
}
