package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	containerdimages "github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/daemon/images"
	"github.com/docker/docker/distribution"
	"github.com/docker/docker/distribution/metadata"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	dockerreference "github.com/docker/docker/reference"
	refstore "github.com/docker/docker/reference"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Migrate migrates the given root directory to containerd
func (d *Daemon) Migrate(ctx context.Context, root string) error {
	if d.containerdCli == nil {
		return errors.New("unable to migrate without containerd")
	}

	// TODO(containerd): Migrate ALL configured graph drivers, in reverse order
	// to keep latest configured driver with the image store
	imageRoot := filepath.Join(root, "image", d.graphDrivers[runtime.GOOS])
	ifs, err := image.NewFSStoreBackend(filepath.Join(imageRoot, "imagedb"))
	if err != nil {
		return err
	}

	// We have a single tag/reference store for the daemon globally. However, it's
	// stored under the graphdriver. On host platforms which only support a single
	// container OS, but multiple selectable graphdrivers, this means depending on which
	// graphdriver is chosen, the global reference store is under there. For
	// platforms which support multiple container operating systems, this is slightly
	// more problematic as where does the global ref store get located? Fortunately,
	// for Windows, which is currently the only daemon supporting multiple container
	// operating systems, the list of graphdrivers available isn't user configurable.
	// For backwards compatibility, we just put it under the windowsfilter
	// directory regardless.
	refStoreLocation := filepath.Join(imageRoot, `repositories.json`)
	rs, err := refstore.NewReferenceStore(refStoreLocation)
	if err != nil {
		return fmt.Errorf("Couldn't create reference store repository: %s", err)
	}

	ctx, done, err := d.containerdCli.WithLease(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if err := done(context.Background()); err != nil {
			logrus.WithError(err).Error("failed to remove lease")
		}
	}()

	if err := image.MigrateImageStore(ctx, ifs, d.containerdCli.ContentStore(), images.LabelImageParent); err != nil {
		return err
	}

	print("Migrating references ")
	numRef := 0
	rs.Walk(func(ref reference.Named) error {
		id, err := rs.Get(ref)
		if err != nil {
			logrus.WithError(err).Warnf("can't get digest for %s", id)
			return nil
		}
		config, err := ifs.Get(id)
		if err != nil {
			logrus.WithError(err).Warnf("can't get config for %s", id)
			return nil
		}

		desc := ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageConfig,
			Digest:    id,
			Size:      int64(len(config)),
		}

		// find out created time
		var img image.Image
		if err := json.Unmarshal(config, &img); err != nil {
			logrus.WithError(err).Warn("can't parse image")
			return nil
		}
		created := img.Created
		// find out updated time
		updated := created
		if updatedStr, err := ifs.GetMetadata(id, "lastUpdated"); err == nil {
			updated, err = time.Parse(time.RFC3339Nano, string(updatedStr))
			if err != nil {
				logrus.WithError(err).Warn("can't parse lastUpdated time %q for %s", string(updatedStr), id)
				updated = created
			}
		}
		_, err = d.containerdCli.ImageService().Create(ctx, containerdimages.Image{
			Name:      ref.String(),
			Target:    desc,
			CreatedAt: created,
			UpdatedAt: updated,
			Labels:    map[string]string{}, // TODO any labels here?
		})
		if err != nil {
			logrus.WithError(err).Warn("can't create image")
			return nil
		}
		print(".")
		// TODO
		// rs.Delete(ref)

		numRef++
		return nil
	})
	println(" done,", numRef, "references")
	return nil
}

// DEPRECATED AFTER MIGRATION

// DistributionServices provides daemon image storage services
type DistributionServices struct {
	DownloadManager   distribution.RootFSDownloadManager
	V2MetadataService metadata.V2MetadataService
	LayerStore        layer.Store
	ImageStore        image.Store
	ReferenceStore    dockerreference.Store
}

// DistributionServices returns services controlling daemon storage
// TODO(containerd): Remove this
func (d *Daemon) DistributionServices() (DistributionServices, error) {
	ls, err := d.imageService.GetLayerStore(platforms.DefaultSpec())
	if err != nil {
		return DistributionServices{}, err
	}
	return DistributionServices{
		LayerStore: ls,
	}, nil
}
