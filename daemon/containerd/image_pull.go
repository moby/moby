package containerd

import (
	"context"
	"io"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// PullImage initiates a pull operation. image is the repository name to pull, and
// tagOrDigest may be either empty, or indicate a specific tag or digest to pull.
func (i *ImageService) PullImage(ctx context.Context, image, tagOrDigest string, platform *specs.Platform, metaHeaders map[string][]string, authConfig *registry.AuthConfig, outStream io.Writer) error {
	var opts []containerd.RemoteOpt
	if platform != nil {
		opts = append(opts, containerd.WithPlatform(platforms.Format(*platform)))
	}
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return errdefs.InvalidParameter(err)
	}

	// TODO(thaJeztah) this could use a WithTagOrDigest() utility
	if tagOrDigest != "" {
		// The "tag" could actually be a digest.
		var dgst digest.Digest
		dgst, err = digest.Parse(tagOrDigest)
		if err == nil {
			ref, err = reference.WithDigest(reference.TrimNamed(ref), dgst)
		} else {
			ref, err = reference.WithTag(ref, tagOrDigest)
		}
		if err != nil {
			return errdefs.InvalidParameter(err)
		}
	}

	_, err = i.client.Pull(ctx, ref.String(), opts...)
	return err
}

// GetRepository returns a repository from the registry.
func (i *ImageService) GetRepository(ctx context.Context, ref reference.Named, authConfig *registry.AuthConfig) (distribution.Repository, error) {
	panic("not implemented")
}
