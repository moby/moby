package containerd

import (
	"context"
	"io"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/pkg/snapshotters"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/streamformatter"
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

	resolver, _ := i.newResolverFromAuthConfig(authConfig)
	opts = append(opts, containerd.WithResolver(resolver))

	jobs := newJobs()
	h := images.HandlerFunc(func(ctx context.Context, desc specs.Descriptor) ([]specs.Descriptor, error) {
		if desc.MediaType != images.MediaTypeDockerSchema1Manifest {
			jobs.Add(desc)
		}
		return nil, nil
	})
	opts = append(opts, containerd.WithImageHandler(h))

	out := streamformatter.NewJSONProgressOutput(outStream, false)
	finishProgress := jobs.showProgress(ctx, out, pullProgress{Store: i.client.ContentStore(), ShowExists: true})
	defer finishProgress()

	opts = append(opts, containerd.WithPullUnpack)
	// TODO(thaJeztah): we may have to pass the snapshotter to use if the pull is part of a "docker run" (container create -> pull image if missing). See https://github.com/moby/moby/issues/45273
	opts = append(opts, containerd.WithPullSnapshotter(i.snapshotter))

	// AppendInfoHandlerWrapper will annotate the image with basic information like manifest and layer digests as labels;
	// this information is used to enable remote snapshotters like nydus and stargz to query a registry.
	infoHandler := snapshotters.AppendInfoHandlerWrapper(ref.String())
	opts = append(opts, containerd.WithImageHandlerWrapper(infoHandler))

	_, err = i.client.Pull(ctx, ref.String(), opts...)
	return err
}
