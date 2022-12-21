//go:build !windows
// +build !windows

/*
 * Copyright (c) 2022. Nydus Developers. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package converter

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/containerd/containerd/archive"
	"github.com/containerd/containerd/archive/compression"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/images/converter"
	"github.com/containerd/containerd/labels"
	"github.com/containerd/fifo"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/containerd/nydus-snapshotter/pkg/converter/tool"
	"github.com/containerd/nydus-snapshotter/pkg/errdefs"
)

const bootstrapNameInTar = "image.boot"
const blobNameInTar = "image.blob"

const envNydusBuilder = "NYDUS_BUILDER"
const envNydusWorkDir = "NYDUS_WORKDIR"

const configGCLabelKey = "containerd.io/gc.ref.content.config"

var bufPool = sync.Pool{
	New: func() interface{} {
		buffer := make([]byte, 1<<20)
		return &buffer
	},
}

func getBuilder(specifiedPath string) string {
	if specifiedPath != "" {
		return specifiedPath
	}

	builderPath := os.Getenv(envNydusBuilder)
	if builderPath != "" {
		return builderPath
	}

	return "nydus-image"
}

func ensureWorkDir(specifiedBasePath string) (string, error) {
	var baseWorkDir string

	if specifiedBasePath != "" {
		baseWorkDir = specifiedBasePath
	} else {
		baseWorkDir = os.Getenv(envNydusWorkDir)
	}
	if baseWorkDir == "" {
		baseWorkDir = os.TempDir()
	}

	if err := os.MkdirAll(baseWorkDir, 0750); err != nil {
		return "", errors.Wrapf(err, "create base directory %s", baseWorkDir)
	}

	workDirPath, err := os.MkdirTemp(baseWorkDir, "nydus-converter-")
	if err != nil {
		return "", errors.Wrap(err, "create work directory")
	}

	return workDirPath, nil
}

// Unpack a OCI formatted tar stream into a directory.
func unpackOciTar(ctx context.Context, dst string, reader io.Reader) error {
	ds, err := compression.DecompressStream(reader)
	if err != nil {
		return errors.Wrap(err, "unpack stream")
	}
	defer ds.Close()

	if _, err := archive.Apply(
		ctx,
		dst,
		ds,
		archive.WithConvertWhiteout(func(hdr *tar.Header, file string) (bool, error) {
			// Keep to extract all whiteout files.
			return true, nil
		}),
	); err != nil {
		return errors.Wrap(err, "apply with convert whiteout")
	}

	return nil
}

// Unpack a Nydus formatted tar stream into a directory.
func unpackNydusTar(ctx context.Context, bootDst, blobDst string, ra content.ReaderAt) error {
	boot, err := os.OpenFile(bootDst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return errors.Wrapf(err, "write to bootstrap %s", bootDst)
	}
	defer boot.Close()

	if err = unpackBootstrapFromNydusTar(ctx, ra, boot); err != nil {
		return errors.Wrap(err, "unpack bootstrap from nydus")
	}

	blob, err := os.OpenFile(blobDst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return errors.Wrapf(err, "write to blob %s", blobDst)
	}
	defer blob.Close()

	if err = unpackBlobFromNydusTar(ctx, ra, blob); err != nil {
		return errors.Wrap(err, "unpack blob from nydus")
	}

	return nil
}

// Unpack the bootstrap from nydus formatted tar stream (blob + bootstrap).
// The nydus formatted tar stream is a tar-like structure that arranges the
// data as follows:
//
// `blob_data | blob_tar_header | bootstrap_data | bootstrap_tar_header`
func unpackBootstrapFromNydusTar(ctx context.Context, ra content.ReaderAt, target io.Writer) error {
	cur := ra.Size()
	reader := newSeekReader(ra)

	const headerSize = 512

	// Seek from tail to head of nydus formatted tar stream to find nydus
	// bootstrap data.
	for {
		if headerSize > cur {
			return fmt.Errorf("invalid tar format at pos %d", cur)
		}

		// Try to seek to the part of tar header.
		var err error
		cur, err = reader.Seek(cur-headerSize, io.SeekCurrent)
		if err != nil {
			return errors.Wrapf(err, "seek to %d for tar header", cur-headerSize)
		}

		tr := tar.NewReader(reader)
		// Parse tar header.
		hdr, err := tr.Next()
		if err != nil {
			return errors.Wrap(err, "parse tar header")
		}

		if hdr.Name == bootstrapNameInTar {
			// Try to seek to the part of tar data (bootstrap_data).
			if hdr.Size > cur {
				return fmt.Errorf("invalid tar format at pos %d", cur)
			}
			bootstrapOffset := cur - hdr.Size
			_, err = reader.Seek(bootstrapOffset, io.SeekStart)
			if err != nil {
				return errors.Wrap(err, "seek to bootstrap data offset")
			}

			// Copy tar data (bootstrap_data) to provided target writer.
			if _, err := io.CopyN(target, reader, hdr.Size); err != nil {
				return errors.Wrap(err, "copy bootstrap data to reader")
			}

			return nil
		}

		if cur == hdr.Size {
			break
		}
	}

	return fmt.Errorf("can't find bootstrap in nydus tar")
}

// Unpack the blob from nydus formatted tar stream (blob + bootstrap).
// The nydus formatted tar stream is a tar-like structure that arranges the
// data as follows:
//
// `blob_data | blob_tar_header | bootstrap_data | bootstrap_tar_header`
func unpackBlobFromNydusTar(ctx context.Context, ra content.ReaderAt, target io.Writer) error {
	cur := ra.Size()
	reader := newSeekReader(ra)

	const headerSize = 512

	// Seek from tail to head of nydus formatted tar stream to find nydus
	// bootstrap data.
	for {
		if headerSize > cur {
			break
		}

		// Try to seek to the part of tar header.
		var err error
		cur, err = reader.Seek(cur-headerSize, io.SeekStart)
		if err != nil {
			return errors.Wrapf(err, "seek to %d for tar header", cur-headerSize)
		}

		tr := tar.NewReader(reader)
		// Parse tar header.
		hdr, err := tr.Next()
		if err != nil {
			return errors.Wrap(err, "parse tar header")
		}

		if hdr.Name == bootstrapNameInTar {
			if hdr.Size > cur {
				return fmt.Errorf("invalid tar format at pos %d", cur)
			}
			cur, err = reader.Seek(cur-hdr.Size, io.SeekStart)
			if err != nil {
				return errors.Wrap(err, "seek to bootstrap data offset")
			}
		} else if hdr.Name == blobNameInTar {
			if hdr.Size > cur {
				return fmt.Errorf("invalid tar format at pos %d", cur)
			}
			_, err = reader.Seek(cur-hdr.Size, io.SeekStart)
			if err != nil {
				return errors.Wrap(err, "seek to blob data offset")
			}
			if _, err := io.CopyN(target, reader, hdr.Size); err != nil {
				return errors.Wrap(err, "copy blob data to reader")
			}
			return nil
		}
	}

	return nil
}

// Pack converts an OCI tar stream to nydus formatted stream with a tar-like
// structure that arranges the data as follows:
//
// `blob_data | blob_tar_header | bootstrap_data | bootstrap_tar_header`
//
// The caller should write OCI tar stream into the returned `io.WriteCloser`,
// then the Pack method will write the nydus formatted stream to `dest`
// provided by the caller.
//
// Important: the caller must check `io.WriteCloser.Close() == nil` to ensure
// the conversion workflow is finished.
func Pack(ctx context.Context, dest io.Writer, opt PackOption) (io.WriteCloser, error) {
	workDir, err := ensureWorkDir(opt.WorkDir)
	if err != nil {
		return nil, errors.Wrap(err, "ensure work directory")
	}
	defer func() {
		if err != nil {
			os.RemoveAll(workDir)
		}
	}()

	sourceDir := filepath.Join(workDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		return nil, errors.Wrap(err, "create source directory")
	}

	pr, pw := io.Pipe()

	unpackDone := make(chan bool, 1)
	go func() {
		if err := unpackOciTar(ctx, sourceDir, pr); err != nil {
			pr.CloseWithError(errors.Wrapf(err, "unpack to %s", sourceDir))
			close(unpackDone)
			return
		}
		unpackDone <- true
	}()

	wc := newWriteCloser(pw, func() error {
		defer func() {
			os.RemoveAll(workDir)
		}()

		// Because PipeWriter#Close is called does not mean that the PipeReader
		// has finished reading all the data, and unpack may not be complete yet,
		// so we need to wait for that here.
		<-unpackDone

		blobPath := filepath.Join(workDir, "blob")
		blobFifo, err := fifo.OpenFifo(ctx, blobPath, syscall.O_CREAT|syscall.O_RDONLY|syscall.O_NONBLOCK, 0644)
		if err != nil {
			return errors.Wrapf(err, "create fifo file")
		}
		defer blobFifo.Close()

		go func() {
			err := tool.Pack(tool.PackOption{
				BuilderPath: getBuilder(opt.BuilderPath),

				BlobPath:         blobPath,
				FsVersion:        opt.FsVersion,
				SourcePath:       sourceDir,
				ChunkDictPath:    opt.ChunkDictPath,
				PrefetchPatterns: opt.PrefetchPatterns,
				Compressor:       opt.Compressor,
				Timeout:          opt.Timeout,
			})
			if err != nil {
				pw.CloseWithError(errors.Wrapf(err, "convert blob for %s", sourceDir))
				blobFifo.Close()
			}
		}()

		buffer := bufPool.Get().(*[]byte)
		defer bufPool.Put(buffer)
		if _, err := io.CopyBuffer(dest, blobFifo, *buffer); err != nil {
			return errors.Wrap(err, "pack nydus tar")
		}

		return nil
	})

	return wc, nil
}

// Merge multiple nydus bootstraps (from each layer of image) to a final
// bootstrap. And due to the possibility of enabling the `ChunkDictPath`
// option causes the data deduplication, it will return the actual blob
// digests referenced by the bootstrap.
func Merge(ctx context.Context, layers []Layer, dest io.Writer, opt MergeOption) ([]digest.Digest, error) {
	workDir, err := ensureWorkDir(opt.WorkDir)
	if err != nil {
		return nil, errors.Wrap(err, "ensure work directory")
	}
	defer os.RemoveAll(workDir)

	eg, ctx := errgroup.WithContext(ctx)
	sourceBootstrapPaths := []string{}
	for idx := range layers {
		sourceBootstrapPaths = append(sourceBootstrapPaths, filepath.Join(workDir, layers[idx].Digest.Hex()))
		eg.Go(func(idx int) func() error {
			return func() error {
				layer := layers[idx]

				// Use the hex hash string of whole tar blob as the bootstrap name.
				bootstrap, err := os.Create(filepath.Join(workDir, layer.Digest.Hex()))
				if err != nil {
					return errors.Wrap(err, "create source bootstrap")
				}
				defer bootstrap.Close()

				if err := unpackBootstrapFromNydusTar(ctx, layer.ReaderAt, bootstrap); err != nil {
					return errors.Wrap(err, "unpack nydus tar")
				}

				return nil
			}
		}(idx))
	}

	if err := eg.Wait(); err != nil {
		return nil, errors.Wrap(err, "unpack all bootstraps")
	}

	targetBootstrapPath := filepath.Join(workDir, "bootstrap")

	blobDigests, err := tool.Merge(tool.MergeOption{
		BuilderPath: getBuilder(opt.BuilderPath),

		SourceBootstrapPaths: sourceBootstrapPaths,
		TargetBootstrapPath:  targetBootstrapPath,
		ChunkDictPath:        opt.ChunkDictPath,
		PrefetchPatterns:     opt.PrefetchPatterns,
		OutputJSONPath:       filepath.Join(workDir, "merge-output.json"),
		Timeout:              opt.Timeout,
	})
	if err != nil {
		return nil, errors.Wrap(err, "merge bootstrap")
	}

	var rc io.ReadCloser

	if opt.WithTar {
		rc, err = packToTar(targetBootstrapPath, fmt.Sprintf("image/%s", bootstrapNameInTar), false)
		if err != nil {
			return nil, errors.Wrap(err, "pack bootstrap to tar")
		}
	} else {
		rc, err = os.Open(targetBootstrapPath)
		if err != nil {
			return nil, errors.Wrap(err, "open targe bootstrap")
		}
	}
	defer rc.Close()

	buffer := bufPool.Get().(*[]byte)
	defer bufPool.Put(buffer)
	if _, err = io.CopyBuffer(dest, rc, *buffer); err != nil {
		return nil, errors.Wrap(err, "copy merged bootstrap")
	}

	return blobDigests, nil
}

// Unpack converts a nydus blob layer to OCI formatted tar stream.
func Unpack(ctx context.Context, ra content.ReaderAt, dest io.Writer, opt UnpackOption) error {
	workDir, err := ensureWorkDir(opt.WorkDir)
	if err != nil {
		return errors.Wrap(err, "ensure work directory")
	}
	defer os.RemoveAll(workDir)

	bootPath, blobPath := filepath.Join(workDir, bootstrapNameInTar), filepath.Join(workDir, blobNameInTar)
	if err = unpackNydusTar(ctx, bootPath, blobPath, ra); err != nil {
		return errors.Wrap(err, "unpack nydus tar")
	}

	tarPath := filepath.Join(workDir, "oci.tar")
	blobFifo, err := fifo.OpenFifo(ctx, tarPath, syscall.O_CREAT|syscall.O_RDONLY|syscall.O_NONBLOCK, 0644)
	if err != nil {
		return errors.Wrapf(err, "create fifo file")
	}
	defer blobFifo.Close()

	unpackErrChan := make(chan error)
	go func() {
		defer close(unpackErrChan)
		err := tool.Unpack(tool.UnpackOption{
			BuilderPath:   getBuilder(opt.BuilderPath),
			BootstrapPath: bootPath,
			BlobPath:      blobPath,
			TarPath:       tarPath,
			Timeout:       opt.Timeout,
		})
		if err != nil {
			blobFifo.Close()
			unpackErrChan <- err
		}
	}()

	buffer := bufPool.Get().(*[]byte)
	defer bufPool.Put(buffer)
	if _, err := io.CopyBuffer(dest, blobFifo, *buffer); err != nil {
		if unpackErr := <-unpackErrChan; unpackErr != nil {
			return errors.Wrap(unpackErr, "unpack")
		}
		return errors.Wrap(err, "copy oci tar")
	}

	return nil
}

// IsNydusBlobAndExists returns true when the specified digest of content exists in
// the content store and it's nydus blob format.
func IsNydusBlobAndExists(ctx context.Context, cs content.Store, desc ocispec.Descriptor) bool {
	_, err := cs.Info(ctx, desc.Digest)
	if err != nil {
		return false
	}

	return IsNydusBlob(ctx, desc)
}

// IsNydusBlob returns true when the specified descriptor is nydus blob format.
func IsNydusBlob(ctx context.Context, desc ocispec.Descriptor) bool {
	if desc.Annotations == nil {
		return false
	}

	_, hasAnno := desc.Annotations[LayerAnnotationNydusBlob]
	return hasAnno
}

// LayerConvertFunc returns a function which converts an OCI image layer to
// a nydus blob layer, and set the media type to "application/vnd.oci.image.layer.nydus.blob.v1".
func LayerConvertFunc(opt PackOption) converter.ConvertFunc {
	return func(ctx context.Context, cs content.Store, desc ocispec.Descriptor) (*ocispec.Descriptor, error) {
		if !images.IsLayerType(desc.MediaType) {
			return nil, nil
		}

		ra, err := cs.ReaderAt(ctx, desc)
		if err != nil {
			return nil, errors.Wrap(err, "get source blob reader")
		}
		defer ra.Close()
		rdr := io.NewSectionReader(ra, 0, ra.Size())

		ref := fmt.Sprintf("convert-nydus-from-%s", desc.Digest)
		dst, err := content.OpenWriter(ctx, cs, content.WithRef(ref))
		if err != nil {
			return nil, errors.Wrap(err, "open blob writer")
		}
		defer dst.Close()

		tr, err := compression.DecompressStream(rdr)
		if err != nil {
			return nil, errors.Wrap(err, "decompress blob stream")
		}

		digester := digest.SHA256.Digester()
		pr, pw := io.Pipe()
		tw, err := Pack(ctx, io.MultiWriter(pw, digester.Hash()), opt)
		if err != nil {
			return nil, errors.Wrap(err, "pack tar to nydus")
		}

		go func() {
			defer pw.Close()
			buffer := bufPool.Get().(*[]byte)
			defer bufPool.Put(buffer)
			if _, err := io.CopyBuffer(tw, tr, *buffer); err != nil {
				pw.CloseWithError(err)
				return
			}
			if err := tr.Close(); err != nil {
				pw.CloseWithError(err)
				return
			}
			if err := tw.Close(); err != nil {
				pw.CloseWithError(err)
				return
			}
		}()

		if err := content.Copy(ctx, dst, pr, 0, ""); err != nil {
			return nil, errors.Wrap(err, "copy nydus blob to content store")
		}

		blobDigest := digester.Digest()
		info, err := cs.Info(ctx, blobDigest)
		if err != nil {
			return nil, errors.Wrapf(err, "get blob info %s", blobDigest)
		}
		if info.Labels == nil {
			info.Labels = map[string]string{}
		}
		// Write a diff id label of layer in content store for simplifying
		// diff id calculation to speed up the conversion.
		// See: https://github.com/containerd/containerd/blob/e4fefea5544d259177abb85b64e428702ac49c97/images/diffid.go#L49
		info.Labels[labels.LabelUncompressed] = blobDigest.String()
		_, err = cs.Update(ctx, info)
		if err != nil {
			return nil, errors.Wrap(err, "update layer label")
		}

		newDesc := ocispec.Descriptor{
			Digest:    blobDigest,
			Size:      info.Size,
			MediaType: MediaTypeNydusBlob,
			Annotations: map[string]string{
				// Use `containerd.io/uncompressed` to generate DiffID of
				// layer defined in OCI spec.
				LayerAnnotationUncompressed: blobDigest.String(),
				LayerAnnotationNydusBlob:    "true",
			},
		}

		if opt.Backend != nil {
			blobRa, err := cs.ReaderAt(ctx, newDesc)
			if err != nil {
				return nil, errors.Wrap(err, "get nydus blob reader")
			}
			defer blobRa.Close()

			if err := opt.Backend.Push(ctx, blobRa, blobDigest); err != nil {
				return nil, errors.Wrap(err, "push to storage backend")
			}
		}

		return &newDesc, nil
	}
}

// ConvertHookFunc returns a function which will be used as a callback
// called for each blob after conversion is done. The function only hooks
// the index conversion and the manifest conversion.
func ConvertHookFunc(opt MergeOption) converter.ConvertHookFunc {
	return func(ctx context.Context, cs content.Store, orgDesc ocispec.Descriptor, newDesc *ocispec.Descriptor) (*ocispec.Descriptor, error) {
		switch {
		case images.IsIndexType(newDesc.MediaType):
			return convertIndex(ctx, cs, orgDesc, newDesc)
		case images.IsManifestType(newDesc.MediaType):
			return convertManifest(ctx, cs, newDesc, opt)
		default:
			return newDesc, nil
		}
	}
}

// convertIndex modifies the original index by appending "nydus.remoteimage.v1"
// to the Platform.OSFeatures of each modified manifest descriptors.
func convertIndex(ctx context.Context, cs content.Store, orgDesc ocispec.Descriptor, newDesc *ocispec.Descriptor) (*ocispec.Descriptor, error) {
	var orgIndex ocispec.Index
	if _, err := readJSON(ctx, cs, &orgIndex, orgDesc); err != nil {
		return nil, errors.Wrap(err, "read target image index json")
	}
	// isManifestModified is a function to check whether the manifest is modified.
	isManifestModified := func(manifest ocispec.Descriptor) bool {
		for _, oldManifest := range orgIndex.Manifests {
			if manifest.Digest == oldManifest.Digest {
				return false
			}
		}
		return true
	}

	var index ocispec.Index
	indexLabels, err := readJSON(ctx, cs, &index, *newDesc)
	if err != nil {
		return nil, errors.Wrap(err, "read index json")
	}
	for i, manifest := range index.Manifests {
		if !isManifestModified(manifest) {
			// Skip the manifest which is not modified.
			continue
		}
		manifest.Platform.OSFeatures = append(manifest.Platform.OSFeatures, ManifestOSFeatureNydus)
		index.Manifests[i] = manifest
	}
	// Update image index in content store.
	newIndexDesc, err := writeJSON(ctx, cs, index, *newDesc, indexLabels)
	if err != nil {
		return nil, errors.Wrap(err, "write index json")
	}
	return newIndexDesc, nil
}

// convertManifest merges all the nydus blob layers into a
// nydus bootstrap layer, update the image config,
// and modify the image manifest.
func convertManifest(ctx context.Context, cs content.Store, newDesc *ocispec.Descriptor, opt MergeOption) (*ocispec.Descriptor, error) {
	var manifest ocispec.Manifest
	manifestDesc := *newDesc
	manifestLabels, err := readJSON(ctx, cs, &manifest, manifestDesc)
	if err != nil {
		return nil, errors.Wrap(err, "read manifest json")
	}

	// Append bootstrap layer to manifest.
	bootstrapDesc, blobDescs, err := MergeLayers(ctx, cs, manifest.Layers, MergeOption{
		BuilderPath:   opt.BuilderPath,
		WorkDir:       opt.WorkDir,
		ChunkDictPath: opt.ChunkDictPath,
		FsVersion:     opt.FsVersion,
		WithTar:       true,
	})
	if err != nil {
		return nil, errors.Wrap(err, "merge nydus layers")
	}
	if opt.Backend != nil {
		// Only append nydus bootstrap layer into manifest, and do not put nydus
		// blob layer into manifest if blob storage backend is specified.
		manifest.Layers = []ocispec.Descriptor{*bootstrapDesc}
	} else {
		for idx, blobDesc := range blobDescs {
			blobGCLabelKey := fmt.Sprintf("containerd.io/gc.ref.content.l.%d", idx)
			manifestLabels[blobGCLabelKey] = blobDesc.Digest.String()
		}
		// Affected by chunk dict, the blob list referenced by final bootstrap
		// are from different layers, part of them are from original layers, part
		// from chunk dict bootstrap, so we need to rewrite manifest's layers here.
		manifest.Layers = append(blobDescs, *bootstrapDesc)
	}

	// Update the gc label of bootstrap layer
	bootstrapGCLabelKey := fmt.Sprintf("containerd.io/gc.ref.content.l.%d", len(manifest.Layers)-1)
	manifestLabels[bootstrapGCLabelKey] = bootstrapDesc.Digest.String()

	// Rewrite diff ids and remove useless annotation.
	var config ocispec.Image
	configLabels, err := readJSON(ctx, cs, &config, manifest.Config)
	if err != nil {
		return nil, errors.Wrap(err, "read image config")
	}
	if opt.Backend != nil {
		config.RootFS.DiffIDs = []digest.Digest{digest.Digest(bootstrapDesc.Annotations[LayerAnnotationUncompressed])}
	} else {
		config.RootFS.DiffIDs = make([]digest.Digest, 0, len(manifest.Layers))
		for i, layer := range manifest.Layers {
			config.RootFS.DiffIDs = append(config.RootFS.DiffIDs, digest.Digest(layer.Annotations[LayerAnnotationUncompressed]))
			// Remove useless annotation.
			delete(manifest.Layers[i].Annotations, LayerAnnotationUncompressed)
		}
	}
	// Update image config in content store.
	newConfigDesc, err := writeJSON(ctx, cs, config, manifest.Config, configLabels)
	if err != nil {
		return nil, errors.Wrap(err, "write image config")
	}
	manifest.Config = *newConfigDesc
	// Update the config gc label
	manifestLabels[configGCLabelKey] = newConfigDesc.Digest.String()

	// Update image manifest in content store.
	newManifestDesc, err := writeJSON(ctx, cs, manifest, manifestDesc, manifestLabels)
	if err != nil {
		return nil, errors.Wrap(err, "write manifest")
	}

	return newManifestDesc, nil
}

// MergeLayers merges a list of nydus blob layer into a nydus bootstrap layer.
// The media type of the nydus bootstrap layer is "application/vnd.oci.image.layer.v1.tar+gzip".
func MergeLayers(ctx context.Context, cs content.Store, descs []ocispec.Descriptor, opt MergeOption) (*ocispec.Descriptor, []ocispec.Descriptor, error) {
	// Extracts nydus bootstrap from nydus format for each layer.
	layers := []Layer{}

	var chainID digest.Digest
	for _, blobDesc := range descs {
		ra, err := cs.ReaderAt(ctx, blobDesc)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "get reader for blob %q", blobDesc.Digest)
		}
		defer ra.Close()
		layers = append(layers, Layer{
			Digest:   blobDesc.Digest,
			ReaderAt: ra,
		})
		if chainID == "" {
			chainID = identity.ChainID([]digest.Digest{blobDesc.Digest})
		} else {
			chainID = identity.ChainID([]digest.Digest{chainID, blobDesc.Digest})
		}
	}

	// Merge all nydus bootstraps into a final nydus bootstrap.
	pr, pw := io.Pipe()
	blobDigestChan := make(chan []digest.Digest, 1)
	go func() {
		defer pw.Close()
		blobDigests, err := Merge(ctx, layers, pw, opt)
		if err != nil {
			pw.CloseWithError(errors.Wrapf(err, "merge nydus bootstrap"))
		}
		blobDigestChan <- blobDigests
	}()

	// Compress final nydus bootstrap to tar.gz and write into content store.
	cw, err := content.OpenWriter(ctx, cs, content.WithRef("nydus-merge-"+chainID.String()))
	if err != nil {
		return nil, nil, errors.Wrap(err, "open content store writer")
	}
	defer cw.Close()

	gw := gzip.NewWriter(cw)
	uncompressedDgst := digest.SHA256.Digester()
	compressed := io.MultiWriter(gw, uncompressedDgst.Hash())
	buffer := bufPool.Get().(*[]byte)
	defer bufPool.Put(buffer)
	if _, err := io.CopyBuffer(compressed, pr, *buffer); err != nil {
		return nil, nil, errors.Wrapf(err, "copy bootstrap targz into content store")
	}
	if err := gw.Close(); err != nil {
		return nil, nil, errors.Wrap(err, "close gzip writer")
	}

	compressedDgst := cw.Digest()
	if err := cw.Commit(ctx, 0, compressedDgst, content.WithLabels(map[string]string{
		LayerAnnotationUncompressed: uncompressedDgst.Digest().String(),
	})); err != nil {
		if !errdefs.IsAlreadyExists(err) {
			return nil, nil, errors.Wrap(err, "commit to content store")
		}
	}
	if err := cw.Close(); err != nil {
		return nil, nil, errors.Wrap(err, "close content store writer")
	}

	bootstrapInfo, err := cs.Info(ctx, compressedDgst)
	if err != nil {
		return nil, nil, errors.Wrap(err, "get info from content store")
	}

	blobDigests := <-blobDigestChan
	blobDescs := []ocispec.Descriptor{}
	blobIDs := []string{}
	for _, blobDigest := range blobDigests {
		blobInfo, err := cs.Info(ctx, blobDigest)
		if err != nil {
			return nil, nil, errors.Wrap(err, "get info from content store")
		}
		blobDesc := ocispec.Descriptor{
			Digest:    blobDigest,
			Size:      blobInfo.Size,
			MediaType: MediaTypeNydusBlob,
			Annotations: map[string]string{
				LayerAnnotationUncompressed: blobDigest.String(),
				LayerAnnotationNydusBlob:    "true",
			},
		}
		blobDescs = append(blobDescs, blobDesc)
		blobIDs = append(blobIDs, blobDigest.Hex())
	}

	blobIDsBytes, err := json.Marshal(blobIDs)
	if err != nil {
		return nil, nil, errors.Wrap(err, "marshal blob ids")
	}

	if opt.FsVersion == "" {
		opt.FsVersion = "5"
	}

	bootstrapDesc := ocispec.Descriptor{
		Digest:    compressedDgst,
		Size:      bootstrapInfo.Size,
		MediaType: ocispec.MediaTypeImageLayerGzip,
		Annotations: map[string]string{
			LayerAnnotationUncompressed: uncompressedDgst.Digest().String(),
			LayerAnnotationFSVersion:    opt.FsVersion,
			// Use this annotation to identify nydus bootstrap layer.
			LayerAnnotationNydusBootstrap: "true",
			// Track all blob digests for nydus snapshotter.
			LayerAnnotationNydusBlobIDs: string(blobIDsBytes),
		},
	}

	return &bootstrapDesc, blobDescs, nil
}
