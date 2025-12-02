package tarexport

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"

	librsync "github.com/balena-os/librsync-go"
	"github.com/moby/moby/v2/daemon/internal/image"
	"github.com/moby/moby/v2/daemon/internal/layer"
	"github.com/moby/moby/v2/daemon/internal/progress"
	"github.com/pkg/errors"
)

const (
	// DeltaBaseLabel identifies the base image ID for a delta image
	DeltaBaseLabel = "io.resin.delta.base"
	// DeltaConfigLabel contains additional delta configuration metadata
	DeltaConfigLabel = "io.resin.delta.config"
)

// isDeltaImage checks if an image is a delta image by looking for the delta base label
func isDeltaImage(img *image.Image) bool {
	if img.Config == nil {
		return false
	}
	_, hasBase := img.Config.Labels[DeltaBaseLabel]
	return hasBase
}

// getBaseImageID extracts the base image ID from a delta image's labels
func getBaseImageID(img *image.Image) (image.ID, error) {
	if img.Config == nil {
		return "", errors.New("image config is nil")
	}
	baseIDStr, ok := img.Config.Labels[DeltaBaseLabel]
	if !ok {
		return "", errors.New("delta image missing base label")
	}
	return image.ID(baseIDStr), nil
}

// deltaConfig represents the metadata stored in the delta config label
type deltaConfig struct {
	DeltaSize  int    `json:"deltaSize"`
	TargetID   string `json:"targetID"`
	Compressed bool   `json:"compressed"`
}

// getDeltaConfig extracts and parses the delta configuration from labels
func getDeltaConfig(img *image.Image) (*deltaConfig, error) {
	if img.Config == nil {
		return nil, errors.New("image config is nil")
	}
	configStr, ok := img.Config.Labels[DeltaConfigLabel]
	if !ok {
		return nil, errors.New("delta image missing config label")
	}

	var cfg deltaConfig
	if err := json.Unmarshal([]byte(configStr), &cfg); err != nil {
		return nil, errors.Wrap(err, "failed to parse delta config")
	}
	return &cfg, nil
}

// applyDelta applies a delta image to its base image to reconstruct the target image.
// This is called during docker load when a delta image is detected.
func (l *tarexporter) applyDelta(ctx context.Context, deltaImg *image.Image, tmpDir string, progressOutput progress.Output) (*image.Image, string, error) {
	// 1. Get base image ID from delta labels
	baseID, err := getBaseImageID(deltaImg)
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to get base image ID")
	}

	progress.Messagef(progressOutput, "", "Delta image detected, base: %s", baseID)

	// 2. Check if base image exists locally
	_, err = l.is.Get(baseID)
	if err != nil {
		return nil, "", errors.Wrapf(err, "delta requires base image %s which is not present in local store", baseID)
	}

	progress.Messagef(progressOutput, "", "Base image found, applying delta")

	// 3. Export base image to tar
	baseTar := new(bytes.Buffer)
	baseExporter := NewTarExporter(l.is, l.lss, l.rs, l.loggerImgEvent, l.platform)
	if err := baseExporter.Save(ctx, []string{baseID.String()}, baseTar); err != nil {
		return nil, "", errors.Wrap(err, "failed to export base image")
	}

	// 4. Extract delta data from the delta image's layer
	// The delta data should be in the first (and only) layer of the delta image
	deltaData, err := l.extractDeltaData(tmpDir, deltaImg)
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to extract delta data")
	}

	progress.Messagef(progressOutput, "", "Applying delta with librsync")

	// 5. Apply delta using librsync to reconstruct target
	targetTar := new(bytes.Buffer)
	if err := librsync.Patch(bytes.NewReader(baseTar.Bytes()), bytes.NewReader(deltaData), targetTar); err != nil {
		return nil, "", errors.Wrap(err, "failed to apply delta patch")
	}

	progress.Messagef(progressOutput, "", "Delta applied successfully, reconstructing image")

	// 6. The targetTar now contains the complete tar export of the target image
	// We need to extract it back to a temp directory and let the normal load process handle it
	targetTmpDir, err := os.MkdirTemp("", "docker-delta-target-")
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to create temp dir for target")
	}

	if err := untar(ctx, io.NopCloser(targetTar), targetTmpDir); err != nil {
		os.RemoveAll(targetTmpDir)
		return nil, "", errors.Wrap(err, "failed to untar reconstructed image")
	}

	progress.Messagef(progressOutput, "", "Reconstructed target image from delta")

	return nil, targetTmpDir, nil
}

// extractDeltaData extracts the raw delta bytes from the delta image's layer files
func (l *tarexporter) extractDeltaData(tmpDir string, deltaImg *image.Image) ([]byte, error) {
	// The delta data is stored in the first (and only) layer of the delta image
	// The layer tar contains a single file: delta.bin

	if len(deltaImg.RootFS.DiffIDs) == 0 {
		return nil, errors.New("delta image has no layers")
	}

	// Get the delta layer from the layer store
	diffID := deltaImg.RootFS.DiffIDs[0]
	deltaLayer, err := l.lss.Get(deltaImg.RootFS.ChainID())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get delta layer %s", diffID)
	}
	defer layer.ReleaseAndLog(l.lss, deltaLayer)

	// Get a tar stream of the layer
	tarStream, err := deltaLayer.TarStream()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tar stream from delta layer")
	}
	defer tarStream.Close()

	// Read through the tar to find delta.bin
	tr := tar.NewReader(tarStream)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.Wrap(err, "failed to read tar header")
		}

		if header.Name == "delta.bin" {
			// Found the delta file, read it
			deltaData, err := io.ReadAll(tr)
			if err != nil {
				return nil, errors.Wrap(err, "failed to read delta.bin from tar")
			}
			return deltaData, nil
		}
	}

	return nil, errors.New("delta.bin not found in layer")
}
