//go:build !windows
// +build !windows

/*
 * Copyright (c) 2022. Nydus Developers. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package converter

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/images/converter"

	"github.com/containerd/errdefs"
	"github.com/containerd/platforms"
	"github.com/klauspost/compress/zstd"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// DefaultIndexConvertFunc wraps containerd's converter to handle Nydus blob reconversion.
//
// Problem: Containerd's convertManifest calls images.GetDiffID() which attempts to
// decompress layers. Nydus blobs use a custom format and fail with "magic number mismatch".
//
// Solution: Wrap content store with a proxy that adds containerd.io/uncompressed label
// for Nydus blobs. This makes GetDiffID take the fast path and skip decompression.
func DefaultIndexConvertFunc(layerConvertFunc converter.ConvertFunc, docker2oci bool, platformMC platforms.MatchComparer) converter.ConvertFunc {
	hooks := converter.ConvertHooks{
		PostConvertHook: ReconvertHookFunc(),
	}

	fn := converter.IndexConvertFuncWithHook(layerConvertFunc, docker2oci, platformMC, hooks)

	return func(ctx context.Context, cs content.Store, desc ocispec.Descriptor) (*ocispec.Descriptor, error) {
		logrus.Debugf("DefaultIndexConvertFunc called for desc mediaType=%s digest=%s", desc.MediaType, desc.Digest.String())

		nydusBlobs := collectNydusBlobDigests(ctx, cs, desc)
		if len(nydusBlobs) == 0 {
			return fn(ctx, cs, desc)
		}

		ws := &wrappedStore{
			Store:      cs,
			nydusBlobs: nydusBlobs,
		}
		return fn(ctx, ws, desc)
	}
}

// This is used to identify which blobs need the uncompressed label workaround.
func collectNydusBlobDigests(ctx context.Context, cs content.Store, desc ocispec.Descriptor) map[digest.Digest]bool {
	nydusBlobs := make(map[digest.Digest]bool)

	if images.IsIndexType(desc.MediaType) {
		var index ocispec.Index
		if _, err := readJSON(ctx, cs, &index, desc); err != nil {
			logrus.WithError(err).Warn("failed to read index")
			return nydusBlobs
		}
		for _, m := range index.Manifests {
			if images.IsManifestType(m.MediaType) {
				collectFromManifest(ctx, cs, m, nydusBlobs)
			}
		}
	} else if images.IsManifestType(desc.MediaType) {
		collectFromManifest(ctx, cs, desc, nydusBlobs)
	}

	logrus.Debugf("Collected %d Nydus blob digests", len(nydusBlobs))
	return nydusBlobs
}

func collectFromManifest(ctx context.Context, cs content.Store, desc ocispec.Descriptor, nydusBlobs map[digest.Digest]bool) {
	var manifest ocispec.Manifest
	if _, err := readJSON(ctx, cs, &manifest, desc); err != nil {
		logrus.WithError(err).Warnf("failed to read manifest %s", desc.Digest)
		return
	}

	for _, l := range manifest.Layers {
		if IsNydusBlob(l) {
			logrus.Debugf("Found Nydus blob: %s (mediaType=%s)", l.Digest, l.MediaType)
			nydusBlobs[l.Digest] = true
		}
	}
}

// wrappedStore wraps the content store to add containerd.io/uncompressed labels
// for Nydus blobs. This makes images.GetDiffID skip decompression and return
// the digest directly.
type wrappedStore struct {
	content.Store
	nydusBlobs map[digest.Digest]bool // Set of Nydus blob digests
}

func (s *wrappedStore) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	info, err := s.Store.Info(ctx, dgst)
	if err != nil {
		return info, err
	}

	// If this is a Nydus blob, add the uncompressed label
	if s.nydusBlobs[dgst] {
		if info.Labels == nil {
			info.Labels = make(map[string]string)
		}
		// Use the blob's own digest as the "uncompressed" digest
		// This makes GetDiffID return the blob digest directly
		info.Labels["containerd.io/uncompressed"] = dgst.String()
		logrus.Debugf("Added uncompressed label for Nydus blob: %s", dgst)
	}

	return info, nil
}

func ReconvertHookFunc() converter.ConvertHookFunc {
	return func(ctx context.Context, cs content.Store, _ ocispec.Descriptor, newDesc *ocispec.Descriptor) (*ocispec.Descriptor, error) {
		if newDesc == nil {
			return nil, nil
		}

		if !images.IsManifestType(newDesc.MediaType) {
			return newDesc, nil
		}

		var manifest ocispec.Manifest
		labels, err := readJSON(ctx, cs, &manifest, *newDesc)
		if err != nil {
			return nil, errors.Wrap(err, "read manifest")
		}

		var layersToKeep []ocispec.Descriptor
		bootstrapIndex := -1

		// 1. Filter Layers: Remove Nydus Bootstrap Layer
		for i, l := range manifest.Layers {
			if IsNydusBootstrap(l) {
				bootstrapIndex = i
				// Clean GC labels for the removed layer
				converter.ClearGCLabels(labels, l.Digest)
			} else {
				layersToKeep = append(layersToKeep, l)
			}
		}

		manifest.Layers = layersToKeep

		// 2. Read and Update Config
		var config ocispec.Image
		configLabels, err := readJSON(ctx, cs, &config, manifest.Config)
		if err != nil {
			return nil, errors.Wrap(err, "read image config")
		}

		// 2.1 Remove corresponding DiffID
		if bootstrapIndex != -1 && len(config.RootFS.DiffIDs) > bootstrapIndex {
			config.RootFS.DiffIDs = append(config.RootFS.DiffIDs[:bootstrapIndex], config.RootFS.DiffIDs[bootstrapIndex+1:]...)
		}

		// 2.2 Clean History
		var newHistory []ocispec.History
		for _, h := range config.History {
			// Remove Nydus Bootstrap History
			if h.Comment == "Nydus Bootstrap Layer" && h.CreatedBy == "Nydus Converter" {
				continue
			}
			newHistory = append(newHistory, h)
		}
		config.History = newHistory

		// 3. Write back Config
		newConfigDesc, err := writeJSON(ctx, cs, config, manifest.Config, configLabels)
		if err != nil {
			return nil, errors.Wrap(err, "write image config")
		}
		manifest.Config = *newConfigDesc
		// Update Manifest GC label for config
		labels["containerd.io/gc.ref.content.config"] = newConfigDesc.Digest.String()

		// 4. Write back Manifest
		return writeJSON(ctx, cs, manifest, *newDesc, labels)
	}
}

func LayerReconvertFunc(opt UnpackOption) converter.ConvertFunc {
	return func(ctx context.Context, cs content.Store, desc ocispec.Descriptor) (*ocispec.Descriptor, error) {
		logrus.Debugf("Reconvert layer: mediaType=%s digest=%s, anno=%v", desc.MediaType, desc.Digest.String(), desc.Annotations)
		if !images.IsLayerType(desc.MediaType) {
			return nil, nil
		}

		// Skip the nydus bootstrap layer.
		if IsNydusBootstrap(desc) {
			logrus.Debugf("skip nydus bootstrap layer %s", desc.Digest.String())
			return &desc, nil
		}

		ra, err := cs.ReaderAt(ctx, desc)
		if err != nil {
			return nil, errors.Wrap(err, "get reader")
		}
		defer ra.Close()

		ref := fmt.Sprintf("convert-oci-from-%s", desc.Digest)
		cw, err := content.OpenWriter(ctx, cs, content.WithRef(ref))
		if err != nil {
			return nil, errors.Wrap(err, "open blob writer")
		}
		defer cw.Close()

		var gw io.WriteCloser
		var mediaType string
		compressor := opt.Compressor
		if compressor == "" {
			compressor = "gzip"
		}
		switch compressor {
		case "gzip":
			gw = gzip.NewWriter(cw)
			mediaType = ocispec.MediaTypeImageLayerGzip
		case "zstd":
			gw, err = zstd.NewWriter(cw)
			if err != nil {
				return nil, errors.Wrap(err, "create zstd writer")
			}
			mediaType = ocispec.MediaTypeImageLayerZstd
		case "uncompressed":
			gw = cw
			mediaType = ocispec.MediaTypeImageLayer
		default:
			return nil, errors.Errorf("unsupported compressor type: %s (support: gzip, zstd, uncompressed)", opt.Compressor)
		}

		uncompressedDgster := digest.SHA256.Digester()
		pr, pw := io.Pipe()

		// Unpack nydus blob to pipe writer in background
		go func() {
			defer pw.Close()
			if err := Unpack(ctx, ra, pw, opt); err != nil {
				pw.CloseWithError(errors.Wrap(err, "unpack nydus to tar"))
			}
		}()

		// Stream data from pipe reader to compressed writer and digester
		compressed := io.MultiWriter(gw, uncompressedDgster.Hash())
		buffer := bufPool.Get().(*[]byte)
		defer bufPool.Put(buffer)
		if _, err = io.CopyBuffer(compressed, pr, *buffer); err != nil {
			return nil, errors.Wrapf(err, "copy to compressed writer")
		}

		// Close compressor writer if different from content writer
		if gw != cw {
			if err = gw.Close(); err != nil {
				return nil, errors.Wrap(err, "close compressor writer")
			}
		}

		uncompressedDigest := uncompressedDgster.Digest()
		compressedDgst := cw.Digest()
		if err = cw.Commit(ctx, 0, compressedDgst, content.WithLabels(map[string]string{
			LayerAnnotationUncompressed: uncompressedDigest.String(),
		})); err != nil {
			if !errdefs.IsAlreadyExists(err) {
				return nil, errors.Wrap(err, "commit to content store")
			}
		}
		if err = cw.Close(); err != nil {
			return nil, errors.Wrap(err, "close content store writer")
		}

		newDesc, err := makeOCIBlobDesc(ctx, cs, uncompressedDigest, compressedDgst, mediaType)
		if err != nil {
			return nil, err
		}

		if opt.Backend != nil {
			if err := opt.Backend.Push(ctx, cs, *newDesc); err != nil {
				return nil, errors.Wrap(err, "push to storage backend")
			}
		}
		return newDesc, nil
	}
}

func makeOCIBlobDesc(ctx context.Context, cs content.Store, uncompressedDigest, targetDigest digest.Digest, mediaType string) (*ocispec.Descriptor, error) {
	targetInfo, err := cs.Info(ctx, targetDigest)
	if err != nil {
		return nil, errors.Wrapf(err, "get target blob info %s", targetDigest)
	}
	if targetInfo.Labels == nil {
		targetInfo.Labels = map[string]string{}
	}

	targetDesc := ocispec.Descriptor{
		Digest:    targetDigest,
		Size:      targetInfo.Size,
		MediaType: mediaType,
		Annotations: map[string]string{
			// Use `containerd.io/uncompressed` to generate DiffID of
			// layer defined in OCI spec.
			LayerAnnotationUncompressed: uncompressedDigest.String(),
		},
	}

	return &targetDesc, nil
}
