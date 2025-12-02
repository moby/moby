package images

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/distribution/reference"
	librsync "github.com/balena-os/librsync-go"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/internal/image"
	"github.com/moby/moby/v2/daemon/internal/layer"
	"github.com/moby/moby/v2/daemon/internal/progress"
	"github.com/moby/moby/v2/daemon/internal/streamformatter"
	"github.com/moby/moby/v2/daemon/server/imagebackend"
	"github.com/pkg/errors"
)

const (
	// DeltaBaseLabel identifies the base image ID for a delta image
	DeltaBaseLabel = "io.resin.delta.base"
	// DeltaConfigLabel contains additional delta configuration metadata
	DeltaConfigLabel = "io.resin.delta.config"
)

// CreateImageDelta generates a binary delta between two images using librsync.
// The delta is stored as a standard OCI image with special metadata labels.
func (i *ImageService) CreateImageDelta(ctx context.Context, baseImage, targetImage, tag string, outStream io.Writer) error {
	progressOutput := streamformatter.NewJSONProgressOutput(outStream, false)

	// 1. Get base and target images
	progress.Messagef(progressOutput, "", "Loading base image")
	baseImg, err := i.GetImage(ctx, baseImage, imagebackend.GetImageOpts{})
	if err != nil {
		return errors.Wrapf(err, "failed to find base image %s", baseImage)
	}

	progress.Messagef(progressOutput, "", "Loading target image")
	targetImg, err := i.GetImage(ctx, targetImage, imagebackend.GetImageOpts{})
	if err != nil {
		return errors.Wrapf(err, "failed to find target image %s", targetImage)
	}

	// 2. Export images to tar streams
	progress.Messagef(progressOutput, "", "Exporting base image")
	baseTar := new(bytes.Buffer)
	if err := i.ExportImage(ctx, []string{baseImage}, nil, baseTar); err != nil {
		return errors.Wrap(err, "failed to export base image")
	}

	progress.Messagef(progressOutput, "", "Exporting target image")
	targetTar := new(bytes.Buffer)
	if err := i.ExportImage(ctx, []string{targetImage}, nil, targetTar); err != nil {
		return errors.Wrap(err, "failed to export target image")
	}

	// 3. Generate librsync signature from base
	progress.Messagef(progressOutput, "", "Generating signature")
	sigBuf := new(bytes.Buffer)
	// Use standard librsync defaults: 2KB block size, 32-byte BLAKE2 checksum
	const blockSize = 2048
	const strongLen = 32
	sig, err := librsync.Signature(bytes.NewReader(baseTar.Bytes()), sigBuf, blockSize, strongLen, librsync.BLAKE2_SIG_MAGIC)
	if err != nil {
		return errors.Wrap(err, "failed to generate signature")
	}

	// 4. Create delta
	progress.Messagef(progressOutput, "", "Creating delta")
	delta := new(bytes.Buffer)
	if err := librsync.Delta(sig, bytes.NewReader(targetTar.Bytes()), delta); err != nil {
		return errors.Wrap(err, "failed to create delta")
	}

	// 5. Create delta image with metadata
	progress.Messagef(progressOutput, "", "Creating delta image")
	deltaID, err := i.createDeltaImage(ctx, delta.Bytes(), baseImg.ID(), targetImg)
	if err != nil {
		return errors.Wrap(err, "failed to create delta image")
	}

	// 6. Tag if specified
	if tag != "" {
		progress.Messagef(progressOutput, "", "Tagging as %s", tag)
		ref, err := reference.ParseNormalizedNamed(tag)
		if err != nil {
			return errors.Wrap(err, "invalid tag")
		}
		ref = reference.TagNameOnly(ref)
		if err := i.TagImage(ctx, deltaID, ref); err != nil {
			return errors.Wrap(err, "failed to tag delta image")
		}
	}

	// 7. Report statistics
	baseSize := baseTar.Len()
	targetSize := targetTar.Len()
	deltaSize := delta.Len()
	var improvement float64
	if deltaSize > 0 {
		improvement = float64(targetSize) / float64(deltaSize)
	}

	progress.Messagef(progressOutput, "", "Base size: %d bytes", baseSize)
	progress.Messagef(progressOutput, "", "Target size: %d bytes", targetSize)
	progress.Messagef(progressOutput, "", "Delta size: %d bytes (%.2fx smaller)", deltaSize, improvement)
	progress.Messagef(progressOutput, "", "Created delta: %s", deltaID)

	return nil
}

// createDeltaImage creates a new OCI image containing the delta data
// with special labels identifying it as a delta image.
func (i *ImageService) createDeltaImage(ctx context.Context, deltaData []byte, baseID image.ID, targetImg *image.Image) (image.ID, error) {
	// 1. Create a tar stream containing the delta data as a single file
	// This will be registered as a layer
	deltaTar := new(bytes.Buffer)
	if err := createDeltaTar(deltaTar, deltaData); err != nil {
		return "", errors.Wrap(err, "failed to create delta tar")
	}

	// 2. Register the delta as a layer (no parent since it's a standalone delta)
	deltaLayer, err := i.layerStore.Register(bytes.NewReader(deltaTar.Bytes()), "")
	if err != nil {
		return "", errors.Wrap(err, "failed to register delta layer")
	}
	defer layer.ReleaseAndLog(i.layerStore, deltaLayer)

	// 3. Create a new image config with delta metadata
	// Start with a minimal config
	img := &image.Image{
		V1Image: image.V1Image{
			DockerVersion: "delta-image",
			Architecture:  targetImg.Architecture,
			OS:            targetImg.OS,
			Config:        targetImg.Config,
			Created:       targetImg.Created,
		},
	}

	// Initialize config if needed
	if img.Config == nil {
		img.Config = &container.Config{}
	}
	if img.Config.Labels == nil {
		img.Config.Labels = make(map[string]string)
	}

	// Add delta-specific labels
	img.Config.Labels[DeltaBaseLabel] = baseID.String()

	// Store delta metadata
	deltaConfig := map[string]interface{}{
		"deltaSize":  len(deltaData),
		"targetID":   targetImg.ID().String(),
		"compressed": false,
	}
	deltaConfigJSON, err := json.Marshal(deltaConfig)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal delta config")
	}
	img.Config.Labels[DeltaConfigLabel] = string(deltaConfigJSON)

	// 4. Set up RootFS with the delta layer
	img.RootFS = &image.RootFS{
		Type:    "layers",
		DiffIDs: []layer.DiffID{deltaLayer.DiffID()},
	}

	// 5. Add history entry
	img.History = []image.History{
		{
			Created:   img.Created,
			Comment:   "Delta image created by Docker",
			CreatedBy: "docker image delta",
		},
	}

	// 6. Marshal image config to JSON
	imgJSON, err := json.Marshal(img)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal image config")
	}

	// 7. Create the image in the image store
	imageID, err := i.imageStore.Create(imgJSON)
	if err != nil {
		return "", errors.Wrap(err, "failed to create image")
	}

	return imageID, nil
}

// createDeltaTar creates a tar archive containing the delta data as a single file
func createDeltaTar(w io.Writer, deltaData []byte) error {
	tw := tar.NewWriter(w)
	defer tw.Close()

	// Create a tar header for the delta file
	header := &tar.Header{
		Name:    "delta.bin",
		Mode:    0600,
		Size:    int64(len(deltaData)),
		ModTime: time.Now(),
	}

	// Write the header
	if err := tw.WriteHeader(header); err != nil {
		return errors.Wrap(err, "failed to write tar header")
	}

	// Write the delta data
	if _, err := tw.Write(deltaData); err != nil {
		return errors.Wrap(err, "failed to write delta data")
	}

	return nil
}
