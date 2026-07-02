//go:build windows
// +build windows

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

package windows

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/Microsoft/hcsshim/pkg/cimfs"
	ocicimlayer "github.com/Microsoft/hcsshim/pkg/ociwclayer/cim"
	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/diff"
	"github.com/containerd/containerd/v2/core/metadata"
	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/containerd/platforms"
	"github.com/containerd/plugin"
	"github.com/containerd/plugin/registry"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

func init() {
	registry.Register(&plugin.Registration{
		Type: plugins.DiffPlugin,
		ID:   "cimfs",
		Requires: []plugin.Type{
			plugins.MetadataPlugin,
		},
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			md, err := ic.GetSingle(plugins.MetadataPlugin)
			if err != nil {
				return nil, err
			}

			if !cimfs.IsCimFSSupported() {
				return nil, fmt.Errorf("host windows version doesn't support CimFS: %w", plugin.ErrSkipPlugin)
			}
			ic.Meta.Platforms = append(ic.Meta.Platforms, platforms.DefaultSpec())
			return NewCimDiff(md.(*metadata.DB).ContentStore())
		},
	})

	registry.Register(&plugin.Registration{
		Type: plugins.DiffPlugin,
		ID:   "blockcim",
		Requires: []plugin.Type{
			plugins.MetadataPlugin,
		},
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			if !cimfs.IsBlockCimSupported() {
				return nil, fmt.Errorf("host OS version doesn't support block CIMs: %w", plugin.ErrSkipPlugin)
			}

			md, err := ic.GetSingle(plugins.MetadataPlugin)
			if err != nil {
				return nil, err
			}

			ic.Meta.Platforms = append(ic.Meta.Platforms, platforms.DefaultSpec())
			return NewBlockCimDiff(md.(*metadata.DB).ContentStore())
		},
	})
}

// cimApplyFunc is an applier function used when extracting layers into the CimFS format.
// Using the archive.applyFunc is very limiting for CimFS use cases as it forces you to
// represent layers as a single string. So for CimFS we skip the archive.Apply call and
// directly call into the layer writer
type cimApplyFunc func(context.Context, io.Reader) (int64, error)

// cimDiff does filesystem comparison and application
// for CimFS specific layer diffs.
type cimDiff struct {
	store content.Store
}

// NewCimDiff is the Windows cim container layer implementation
// for comparing and applying filesystem layers
func NewCimDiff(store content.Store) (CompareApplier, error) {
	return cimDiff{
		store: store,
	}, nil
}

// Apply applies the content associated with the provided digests onto the
// provided mounts. Archive content will be extracted and decompressed if
// necessary.
func (c cimDiff) Apply(ctx context.Context, desc ocispec.Descriptor, mounts []mount.Mount, opts ...diff.ApplyOpt) (d ocispec.Descriptor, err error) {
	if len(mounts) != 1 {
		return emptyDesc, fmt.Errorf("number of mounts should always be 1 for CimFS layers: %w", errdefs.ErrInvalidArgument)
	} else if mounts[0].Type != mount.CimFSMountType {
		return emptyDesc, fmt.Errorf("cimDiff does not support layer type %s: %w", mounts[0].Type, errdefs.ErrNotImplemented)
	}

	m := mounts[0]
	parentLayerPaths, err := m.GetParentPaths()
	if err != nil {
		return emptyDesc, err
	}
	parentLayerCimPaths, err := mount.GetParentCimPaths(&m)
	if err != nil {
		return emptyDesc, err
	}
	cimPath, err := mount.GetCimPath(&m)
	if err != nil {
		return emptyDesc, err
	}

	applyFunc := func(fCtx context.Context, r io.Reader) (int64, error) {
		return ocicimlayer.ImportCimLayerFromTar(fCtx, r, m.Source, cimPath, parentLayerPaths, parentLayerCimPaths)
	}

	return applyCIMLayerCommon(ctx, desc, c.store, applyFunc, opts...)
}

// Compare creates a diff between the given mounts and uploads the result
// to the content store.
func (c cimDiff) Compare(ctx context.Context, lower, upper []mount.Mount, opts ...diff.Opt) (d ocispec.Descriptor, err error) {
	// support for generating layer diff of cimfs layers will be added later.
	return emptyDesc, errdefs.ErrNotImplemented
}

// blockCIMDiff does filesystem comparison and application
// for blocked CIMs.
type blockCIMDiff struct {
	store content.Store
}

// NewBlockCimDiff is the Windows blocked cim container layer implementation for comparing
// and applying filesystem layers
func NewBlockCimDiff(store content.Store) (CompareApplier, error) {
	return blockCIMDiff{
		store: store,
	}, nil
}

// parseBlockCIMMount parses the mount returned by the BlockCIM snapshotter and returns
func parseBlockCIMMount(m *mount.Mount) (*cimfs.BlockCIM, []*cimfs.BlockCIM, error) {
	var (
		parentPaths []string
	)

	for _, option := range m.Options {
		if val, ok := strings.CutPrefix(option, mount.ParentLayerCimPathsFlag); ok {
			err := json.Unmarshal([]byte(val), &parentPaths)
			if err != nil {
				return nil, nil, err
			}
		} else if val, ok = strings.CutPrefix(option, mount.BlockCIMTypeFlag); ok {
			// only support single file for extraction for now
			if val != mount.BlockCIMTypeFile {
				return nil, nil, fmt.Errorf("extraction doesn't support layer type `%s`", val)
			}
		}
	}

	var (
		parentLayers    []*cimfs.BlockCIM
		extractionLayer *cimfs.BlockCIM
	)

	extractionLayer = &cimfs.BlockCIM{
		Type:      cimfs.BlockCIMTypeSingleFile,
		BlockPath: filepath.Dir(m.Source),
		CimName:   filepath.Base(m.Source),
	}
	for _, p := range parentPaths {
		parentLayers = append(parentLayers, &cimfs.BlockCIM{
			Type:      cimfs.BlockCIMTypeSingleFile,
			BlockPath: filepath.Dir(p),
			CimName:   filepath.Base(p),
		})
	}
	return extractionLayer, parentLayers, nil
}

// Apply applies the content associated with the provided digests onto the
// provided mounts. Archive content will be extracted and decompressed if
// necessary.
func (c blockCIMDiff) Apply(ctx context.Context, desc ocispec.Descriptor, mounts []mount.Mount, opts ...diff.ApplyOpt) (d ocispec.Descriptor, err error) {
	if len(mounts) != 1 {
		return emptyDesc, fmt.Errorf("number of mounts should always be 1 for CimFS layers: %w", errdefs.ErrInvalidArgument)
	} else if mounts[0].Type != mount.BlockCIMMountType {
		return emptyDesc, fmt.Errorf("blockCIMDiff does not support layer type %s: %w", mounts[0].Type, errdefs.ErrNotImplemented)
	}

	m := mounts[0]

	log.G(ctx).WithFields(logrus.Fields{
		"mount": m,
	}).Info("applying blockCIM diff")

	layer, parentLayers, err := parseBlockCIMMount(&m)
	if err != nil {
		return emptyDesc, err
	}

	enableLayerIntegrity := mount.GetEnableLayerIntegrity(&m)
	appendVHDFooter := mount.GetAppendVHDFooter(&m)

	// Build import options based on mount configuration
	var importOpts []ocicimlayer.BlockCIMLayerImportOpt
	importOpts = append(importOpts, ocicimlayer.WithParentLayers(parentLayers))

	if appendVHDFooter {
		importOpts = append(importOpts, ocicimlayer.WithVHDFooter())
	}

	if enableLayerIntegrity {
		importOpts = append(importOpts, ocicimlayer.WithLayerIntegrity())
	}

	applyFunc := func(ctx context.Context, r io.Reader) (int64, error) {
		return ocicimlayer.ImportBlockCIMLayerWithOpts(ctx, r, layer, importOpts...)
	}

	return applyCIMLayerCommon(ctx, desc, c.store, applyFunc, opts...)

}

// Compare creates a diff between the given mounts and uploads the result
// to the content store.
func (c blockCIMDiff) Compare(ctx context.Context, lower, upper []mount.Mount, opts ...diff.Opt) (d ocispec.Descriptor, err error) {
	// support for generating layer diff of cimfs layers will be added later.
	return emptyDesc, errdefs.ErrNotImplemented
}

// applyCimFSCommon is a common function used for applying all diffs to a cim layer.
func applyCIMLayerCommon(ctx context.Context, desc ocispec.Descriptor, store content.Store, applyFunc cimApplyFunc, opts ...diff.ApplyOpt) (_ ocispec.Descriptor, err error) {
	var config diff.ApplyConfig
	for _, o := range opts {
		if err := o(ctx, desc, &config); err != nil {
			return emptyDesc, fmt.Errorf("failed to apply config opt: %w", err)
		}
	}

	t1 := time.Now()
	defer func() {
		if err == nil {
			log.G(ctx).WithFields(log.Fields{
				"d":      time.Since(t1),
				"digest": desc.Digest,
				"size":   desc.Size,
				"media":  desc.MediaType,
			}).Debug("diff applied")
		}
	}()

	ra, err := store.ReaderAt(ctx, desc)
	if err != nil {
		return emptyDesc, fmt.Errorf("failed to get reader from content store: %w", err)
	}
	defer ra.Close()

	processor := diff.NewProcessorChain(desc.MediaType, content.NewReader(ra))
	for {
		if processor, err = diff.GetProcessor(ctx, processor, config.ProcessorPayloads); err != nil {
			return emptyDesc, fmt.Errorf("failed to get stream processor for %s: %w", desc.MediaType, err)
		}
		if processor.MediaType() == ocispec.MediaTypeImageLayer {
			break
		}
	}
	defer processor.Close()

	digester := digest.Canonical.Digester()
	rc := &readCounter{
		r: io.TeeReader(processor, digester.Hash()),
	}

	if _, err = applyFunc(ctx, rc); err != nil {
		return emptyDesc, err
	}

	// Read any trailing data
	if _, err := io.Copy(io.Discard, rc); err != nil {
		return emptyDesc, err
	}

	return ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayer,
		Size:      rc.c,
		Digest:    digester.Digest(),
	}, nil

}
