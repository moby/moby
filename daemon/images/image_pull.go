package images // import "github.com/docker/docker/daemon/images"

import (
	"context"
	"io"
	"runtime"
	"strings"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/archive/compression"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/layer"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// PullImage initiates a pull operation. image is the repository name to pull, and
// tag may be either empty, or indicate a specific tag to pull.
func (i *ImageService) PullImage(ctx context.Context, image, tag string, platform *specs.Platform, metaHeaders map[string][]string, authConfig *types.AuthConfig, outStream io.Writer) error {
	start := time.Now()
	// Special case: "pull -a" may send an image name with a
	// trailing :. This is ugly, but let's not break API
	// compatibility.
	image = strings.TrimSuffix(image, ":")

	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return errdefs.InvalidParameter(err)
	}

	if tag != "" {
		// The "tag" could actually be a digest.
		var dgst digest.Digest
		dgst, err = digest.Parse(tag)
		if err == nil {
			ref, err = reference.WithDigest(reference.TrimNamed(ref), dgst)
		} else {
			ref, err = reference.WithTag(ref, tag)
		}
		if err != nil {
			return errdefs.InvalidParameter(err)
		}
	}

	err = i.pullImageWithReference(ctx, ref, platform, metaHeaders, authConfig, outStream)
	imageActions.WithValues("pull").UpdateSince(start)
	return err
}

func (i *ImageService) pullImageWithReference(ctx context.Context, ref reference.Named, platform *specs.Platform, metaHeaders map[string][]string, authConfig *types.AuthConfig, outStream io.Writer) error {
	c, err := i.getCache(ctx)
	if err != nil {
		return err
	}

	// Include a buffer so that slow client connections don't affect
	// transfer performance.
	//progressChan := make(chan progress.Progress, 100)

	//writesDone := make(chan struct{})

	//ctx, cancelFunc := context.WithCancel(ctx)

	// TODO: Lease

	opts := []containerd.RemoteOpt{}
	// TODO: Custom resolver
	//  - Auth config
	//  - Custom headers
	// TODO: Platforms using `platform`
	// TODO: progress tracking
	// TODO: unpack tracking, use download manager for now?

	img, err := i.client.Pull(ctx, ref.String(), opts...)

	config, err := img.Config(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to resolve configuration")
	}

	l, err := i.unpack(ctx, img.Target())
	if err != nil {
		return errors.Wrapf(err, "failed to unpack %s", img.Target().Digest)
	}

	// TODO: Unpack into layer store
	// TODO: only unpack image types (does containerd already do this?)

	// TODO: Update image with ID label
	// TODO(containerd): Create manifest reference and add image

	c.m.Lock()
	ci, ok := c.idCache[config.Digest]
	if ok {
		ll := ci.layer
		ci.layer = l
		if ll != nil {
			metadata, err := i.layerStores[runtime.GOOS].Release(ll)
			if err != nil {
				return errors.Wrap(err, "failed to release layer")
			}
			layer.LogReleaseMetadata(metadata)
		}

		ci.addReference(ref)
		// TODO: Add manifest digest ref
	} else {
		ci = &cachedImage{
			config:     config,
			references: []reference.Named{ref},
			layer:      l,
		}
		c.idCache[config.Digest] = ci
	}
	c.tCache[img.Target().Digest] = ci
	c.m.Unlock()

	//go func() {
	//	progressutils.WriteDistributionProgress(cancelFunc, outStream, progressChan)
	//	close(writesDone)
	//}()

	//close(progressChan)
	//<-writesDone
	return err
}

// TODO: Add shallow pull function which returns descriptor

func (i *ImageService) unpack(ctx context.Context, target ocispec.Descriptor) (layer.Layer, error) {
	var (
		cs = i.client.ContentStore()
	)

	manifest, err := images.Manifest(ctx, cs, target, platforms.Default())
	if err != nil {
		return nil, err
	}

	diffIDs, err := images.RootFS(ctx, cs, manifest.Config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve rootfs")
	}
	if len(diffIDs) != len(manifest.Layers) {
		return nil, errors.Errorf("mismatched image rootfs and manifest layers")
	}

	var (
		chain = []digest.Digest{}
		l     layer.Layer
	)
	for d := range diffIDs {
		chain = append(chain, diffIDs[d])

		nl, err := i.applyLayer(ctx, manifest.Layers[d], chain)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to apply layer %d", d)
		}
		logrus.Debugf("Layer applied: %s (%s)", nl.DiffID(), diffIDs[d])

		if l != nil {
			metadata, err := i.layerStores[runtime.GOOS].Release(l)
			if err != nil {
				return nil, errors.Wrap(err, "failed to release layer")
			}
			layer.LogReleaseMetadata(metadata)
		}

		// TODO(containerd): verify diff ID

		l = nl
	}
	return l, nil
}

func (i *ImageService) applyLayer(ctx context.Context, blob ocispec.Descriptor, layers []digest.Digest) (layer.Layer, error) {
	var (
		cs = i.client.ContentStore()
		ls = i.layerStores[runtime.GOOS]
	)

	l, err := ls.Get(layer.ChainID(identity.ChainID(layers)))
	if err == nil {
		return l, nil
	} else if err != layer.ErrLayerDoesNotExist {
		return nil, err
	}

	ra, err := cs.ReaderAt(ctx, blob)
	if err != nil {
		return nil, err
	}
	defer ra.Close()

	dc, err := compression.DecompressStream(content.NewReader(ra))
	if err != nil {
		return nil, err
	}
	defer dc.Close()

	var parent digest.Digest
	if len(layers) > 1 {
		parent = identity.ChainID(layers[:len(layers)-1])
	}

	return ls.Register(dc, layer.ChainID(parent))
}
