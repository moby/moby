package images // import "github.com/docker/docker/daemon/images"

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/containerd/containerd"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
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

	// TODO: Unpack into layer store
	// TODO: only unpack image types (does containerd already do this?)

	// TODO: Update image with ID label
	// TODO(containerd): Create manifest reference and add image

	c.m.Lock()
	ci, ok := c.idCache[config.Digest]
	if ok {
		ci.addReference(ref)
		// TODO: Add manifest digest ref
	} else {
		ci = &cachedImage{
			config:     config,
			references: []reference.Named{ref},
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
