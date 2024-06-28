package migration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/continuity/fs"
	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/reference"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/errgroup"
)

type LayerMigrator struct {
	layers  layer.Store
	refs    reference.Store
	dis     image.Store
	leases  leases.Manager
	content content.Store
	cis     images.Store
}

type Config struct {
	LayerStore       layer.Store
	ReferenceStore   reference.Store
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

// MigrateTocontainerd migrates containers from overlay2 to overlayfs
func (lm *LayerMigrator) MigrateTocontainerd(ctx context.Context, snKey string, sn snapshots.Snapshotter) error {
	if sn == nil {
		return fmt.Errorf("no snapshotter to migrate to: %w", errdefs.ErrNotImplemented)
	}

	if lm.layers.DriverName() != "overlay2" {
		return fmt.Errorf("only overlay2 supported for migration: %w", errdefs.ErrNotImplemented)
	}

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
			ml    sync.Mutex
			eg, _ = errgroup.WithContext(ctx) // TODO: Use this context in error group calls
		)
		for i := range diffids {
			chainID := layer.CreateChainID(diffids[:i+1])
			l, err := lm.layers.Get(chainID)
			if err != nil {
				return fmt.Errorf("failed to get layer [%d] %q: %w", i, chainID, err)
			}
			layerIndex := i
			eg.Go(func() error {
				// TODO: Get tar stream, compress it, and add to content store
				//l.TarStream()

				ml.Lock()
				manifest.Layers[layerIndex] = ocispec.Descriptor{
					MediaType: ocispec.MediaTypeImageLayerGzip,
					// Size
					// Digest
				}
				ml.Unlock()
				return nil
			})

			metadata, err := l.Metadata()
			if err != nil {
				return err
			}
			upper, ok := metadata["UpperDir"]
			if !ok {
				return fmt.Errorf("graphdriver not supported: %w", errdefs.ErrNotImplemented)
			}
			log.G(ctx).WithField("metadata", metadata).Debugf("migrating %s from %s", chainID, upper)

			active := fmt.Sprintf("migration-%s", chainID)

			// TODO: Check if already exists
			mounts, err := sn.Prepare(ctx, active, parent)
			if err != nil {
				return err
			}

			dst, err := extractSource(mounts)
			if err != nil {
				return err
			}

			if err := fs.CopyDir(dst, upper); err != nil {
				return err
			}

			key := chainID.String()
			if err := sn.Commit(ctx, key, active); err != nil {
				return err
			}
			parent = key
		}

		configBytes := img.RawJSON()
		digest.FromBytes(configBytes)
		manifest.Config = ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageConfig,
			Digest:    digest.FromBytes(configBytes),
			Size:      int64(len(configBytes)),
		}

		if err = content.WriteBlob(ctx, lm.content, "config"+manifest.Config.Digest.String(), bytes.NewReader(configBytes), manifest.Config); err != nil && !errdefs.IsAlreadyExists(err) {
			return err
		}

		gcLabel := fmt.Sprintf("containerd.io/gc.ref.snapshot.%s", snKey)
		cinfo := content.Info{
			Digest: manifest.Config.Digest,
			Labels: map[string]string{
				gcLabel: parent,
			},
		}
		_, err = lm.content.Update(ctx, cinfo, fmt.Sprintf("labels.%s", gcLabel))
		if err != nil {
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

		if err = content.WriteBlob(ctx, lm.content, "manifest"+manifestDesc.Digest.String(), bytes.NewReader(manifestBytes), manifestDesc); err != nil && !errdefs.IsAlreadyExists(err) {
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
			if err != nil && !errdefs.IsAlreadyExists(err) {
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
			if err != nil && !errdefs.IsAlreadyExists(err) {
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
		return "", fmt.Errorf("cannot support snapshotters with multiple mount sources: %w", errdefs.ErrNotImplemented)
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
		return "", fmt.Errorf("mount type %q not supported: %w", mounts[0].Type, errdefs.ErrNotImplemented)
	}

	return "", fmt.Errorf("mount is missing upper option: %w", errdefs.ErrNotImplemented)
}
