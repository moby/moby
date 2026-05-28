package migration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/leases"
	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/containerd/v2/core/snapshots"
	"github.com/containerd/containerd/v2/pkg/archive/compression"
	"github.com/containerd/continuity/fs"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/internal/image"
	"github.com/moby/moby/v2/daemon/internal/layer"
	refstore "github.com/moby/moby/v2/daemon/internal/refstore"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/errgroup"
)

type LayerMigrator struct {
	layers  layer.Store
	refs    refstore.Store
	dis     image.Store
	leases  leases.Manager
	content content.Store
	cis     images.Store
}

type Config struct {
	ImageCount       int
	LayerStore       layer.Store
	ReferenceStore   refstore.Store
	DockerImageStore image.Store
	Leases           leases.Manager
	Content          content.Store
	ImageStore       images.Store
}

func NewLayerMigrator(config Config) *LayerMigrator {
	return &LayerMigrator{
		layers:  config.LayerStore,
		refs:    config.ReferenceStore,
		dis:     config.DockerImageStore,
		leases:  config.Leases,
		content: config.Content,
		cis:     config.ImageStore,
	}
}

// MigrateTocontainerd migrates containers from overlay2 to overlayfs or vfs to native
func (lm *LayerMigrator) MigrateTocontainerd(ctx context.Context, snKey string, sn snapshots.Snapshotter) error {
	if sn == nil {
		return fmt.Errorf("no snapshotter to migrate to: %w", cerrdefs.ErrNotImplemented)
	}

	switch driver := lm.layers.DriverName(); driver {
	case "overlay2":
	case "vfs":
	default:
		return fmt.Errorf("%q not supported for migration: %w", driver, cerrdefs.ErrNotImplemented)
	}

	var (
		// Zstd makes migration 10x faster
		// TODO: make configurable
		layerMediaType   = ocispec.MediaTypeImageLayerZstd
		layerCompression = compression.Zstd
	)

	l, err := lm.leases.Create(ctx, leases.WithRandomID(), leases.WithExpiration(24*time.Hour))
	if err != nil {
		return err
	}
	defer func() {
		lm.leases.Delete(ctx, l)
	}()
	ctx = leases.WithLease(ctx, l.ID)

	for imgID, img := range lm.dis.Heads() {
		diffids := img.RootFS.DiffIDs
		if len(diffids) == 0 {
			continue
		}
		var (
			parent   string
			manifest = ocispec.Manifest{
				MediaType: ocispec.MediaTypeImageManifest,
				Versioned: specs.Versioned{
					SchemaVersion: 2,
				},
				Layers: make([]ocispec.Descriptor, len(diffids)),
			}
			ml        sync.Mutex
			eg, egctx = errgroup.WithContext(ctx)
		)
		for i := range diffids {
			chainID := identity.ChainID(diffids[:i+1])
			l, err := lm.layers.Get(chainID)
			if err != nil {
				return fmt.Errorf("failed to get layer [%d] %q: %w", i, chainID, err)
			}
			layerIndex := i
			eg.Go(func() error {
				ctx := egctx
				t1 := time.Now()
				ts, err := l.TarStream()
				if err != nil {
					return err
				}

				desc := ocispec.Descriptor{
					MediaType: layerMediaType,
				}

				cw, err := lm.content.Writer(ctx,
					content.WithRef(fmt.Sprintf("ingest-%s", chainID)),
					content.WithDescriptor(desc))
				if err != nil {
					return fmt.Errorf("failed to get content writer: %w", err)
				}

				dgstr := digest.Canonical.Digester()
				cs, _ := compression.CompressStream(io.MultiWriter(cw, dgstr.Hash()), layerCompression)
				_, err = io.Copy(cs, ts)
				if err != nil {
					return fmt.Errorf("failed to copy to compressed stream: %w", err)
				}
				cs.Close()

				status, err := cw.Status()
				if err != nil {
					return err
				}

				desc.Size = status.Offset
				desc.Digest = dgstr.Digest()

				if err := cw.Commit(ctx, desc.Size, desc.Digest); err != nil && !cerrdefs.IsAlreadyExists(err) {
					return err
				}

				log.G(ctx).WithFields(log.Fields{
					"t":      time.Since(t1),
					"size":   desc.Size,
					"digest": desc.Digest,
				}).Debug("Converted layer to content tar")

				ml.Lock()
				manifest.Layers[layerIndex] = desc
				ml.Unlock()
				return nil
			})

			metadata, err := l.Metadata()
			if err != nil {
				return err
			}
			src, ok := metadata["UpperDir"]
			if !ok {
				src, ok = metadata["SourceDir"]
				if !ok {
					log.G(ctx).WithField("metadata", metadata).WithField("driver", lm.layers.DriverName()).Debug("no source directory metadata")
					return fmt.Errorf("graphdriver not supported: %w", cerrdefs.ErrNotImplemented)
				}
			}
			log.G(ctx).WithField("metadata", metadata).Debugf("migrating %s from %s", chainID, src)

			active := fmt.Sprintf("migration-%s", chainID)

			key := chainID.String()

			snapshotLabels := map[string]string{
				"containerd.io/snapshot.ref": key,
			}
			mounts, err := sn.Prepare(ctx, active, parent, snapshots.WithLabels(snapshotLabels))
			parent = key
			if err != nil {
				if cerrdefs.IsAlreadyExists(err) {
					continue
				}
				return err
			}

			dst, err := extractSource(mounts)
			if err != nil {
				return err
			}

			t1 := time.Now()
			if err := fs.CopyDir(dst, src); err != nil {
				return err
			}
			log.G(ctx).WithFields(log.Fields{
				"t":   time.Since(t1),
				"key": key,
			}).Debug("Copied layer to snapshot")

			if err := sn.Commit(ctx, key, active); err != nil && !cerrdefs.IsAlreadyExists(err) {
				return err
			}
		}

		configBytes := img.RawJSON()
		digest.FromBytes(configBytes)
		manifest.Config = ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageConfig,
			Digest:    digest.FromBytes(configBytes),
			Size:      int64(len(configBytes)),
		}

		configLabels := map[string]string{
			fmt.Sprintf("containerd.io/gc.ref.snapshot.%s", snKey): parent,
		}
		if err = content.WriteBlob(ctx, lm.content, "config"+manifest.Config.Digest.String(), bytes.NewReader(configBytes), manifest.Config, content.WithLabels(configLabels)); err != nil && !cerrdefs.IsAlreadyExists(err) {
			return err
		}

		if err := eg.Wait(); err != nil {
			return err
		}

		manifestBytes, err := json.MarshalIndent(manifest, "", "   ")
		if err != nil {
			return err
		}

		manifestDesc := ocispec.Descriptor{
			MediaType: manifest.MediaType,
			Digest:    digest.FromBytes(manifestBytes),
			Size:      int64(len(manifestBytes)),
		}

		manifestLabels := map[string]string{
			"containerd.io/gc.ref.content.config": manifest.Config.Digest.String(),
		}
		for i := range manifest.Layers {
			manifestLabels[fmt.Sprintf("containerd.io/gc.ref.content.%d", i)] = manifest.Layers[i].Digest.String()
		}

		if err = content.WriteBlob(ctx, lm.content, "manifest"+manifestDesc.Digest.String(), bytes.NewReader(manifestBytes), manifestDesc, content.WithLabels(manifestLabels)); err != nil && !cerrdefs.IsAlreadyExists(err) {
			return err
		}

		childrenHandler := images.ChildrenHandler(lm.content)
		childrenHandler = images.SetChildrenMappedLabels(lm.content, childrenHandler, nil)
		if err = images.Walk(ctx, childrenHandler, manifestDesc); err != nil {
			return err
		}

		var added bool
		for _, named := range lm.refs.References(digest.Digest(imgID)) {
			img := images.Image{
				Name:   named.String(),
				Target: manifestDesc,
				// TODO: Any labels?
			}
			img, err = lm.cis.Create(ctx, img)
			if err != nil && !cerrdefs.IsAlreadyExists(err) {
				return err
			} else if err != nil {
				log.G(ctx).Infof("Tag already exists: %s", named)
				continue
			}

			log.G(ctx).Infof("Migrated image %s to %s", img.Name, img.Target.Digest)
			added = true
		}

		if !added {
			img := images.Image{
				Name:   "moby-dangling@" + manifestDesc.Digest.String(),
				Target: manifestDesc,
				// TODO: Any labels?
			}
			img, err = lm.cis.Create(ctx, img)
			if err != nil && !cerrdefs.IsAlreadyExists(err) {
				return err
			} else if err == nil {
				log.G(ctx).Infof("Migrated image %s to %s", img.Name, img.Target.Digest)
			}
		}
	}

	return nil
}

func extractSource(mounts []mount.Mount) (string, error) {
	if len(mounts) != 1 {
		return "", fmt.Errorf("cannot support snapshotters with multiple mount sources: %w", cerrdefs.ErrNotImplemented)
	}
	switch mounts[0].Type {
	case "bind":
		return mounts[0].Source, nil
	case "overlay":
		for _, option := range mounts[0].Options {
			if strings.HasPrefix(option, "upperdir=") {
				return option[9:], nil
			}
		}
	default:
		return "", fmt.Errorf("mount type %q not supported: %w", mounts[0].Type, cerrdefs.ErrNotImplemented)
	}

	return "", fmt.Errorf("mount is missing upper option: %w", cerrdefs.ErrNotImplemented)
}
