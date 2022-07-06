package containerd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	cerrdefs "github.com/containerd/containerd/errdefs"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	imagetype "github.com/docker/docker/api/types/image"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/images"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

var shortID = regexp.MustCompile(`^([a-f0-9]{4,64})$`)

type containerdStore struct {
	client *containerd.Client
}

func NewService(c *containerd.Client) *containerdStore {
	return &containerdStore{
		client: c,
	}
}

func (cs *containerdStore) PullImage(ctx context.Context, image, tag string, platform *ocispec.Platform, metaHeaders map[string][]string, authConfig *types.AuthConfig, outStream io.Writer) error {
	var opts []containerd.RemoteOpt
	if platform != nil {
		opts = append(opts, containerd.WithPlatform(platforms.Format(*platform)))
	}
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

	_, err = cs.client.Pull(ctx, ref.String(), opts...)
	return err
}

func (cs *containerdStore) Images(ctx context.Context, opts types.ImageListOptions) ([]*types.ImageSummary, error) {
	images, err := cs.client.ListImages(ctx)
	if err != nil {
		return nil, err
	}

	var ret []*types.ImageSummary
	for _, image := range images {
		size, err := image.Size(ctx)
		if err != nil {
			return nil, err
		}

		ret = append(ret, &types.ImageSummary{
			RepoDigests: []string{image.Name() + "@" + image.Target().Digest.String()}, // "hello-world@sha256:bfea6278a0a267fad2634554f4f0c6f31981eea41c553fdf5a83e95a41d40c38"},
			RepoTags:    []string{image.Name()},
			Containers:  -1,
			ParentID:    "",
			SharedSize:  -1,
			VirtualSize: 10,
			ID:          image.Target().Digest.String(),
			Created:     image.Metadata().CreatedAt.Unix(),
			Size:        size,
		})
	}

	return ret, nil
}

func (cs *containerdStore) LogImageEvent(imageID, refName, action string) {
	panic("not implemented")
}

func (cs *containerdStore) LogImageEventWithAttributes(imageID, refName, action string, attributes map[string]string) {
	panic("not implemented")
}

func (cs *containerdStore) GetLayerFolders(img *image.Image, rwLayer layer.RWLayer) ([]string, error) {
	panic("not implemented")
}

func (cs *containerdStore) Map() map[image.ID]*image.Image {
	panic("not implemented")
}

func (cs *containerdStore) GetLayerByID(string) (layer.RWLayer, error) {
	panic("not implemented")
}

func (cs *containerdStore) GetLayerMountID(string) (string, error) {
	panic("not implemented")
}

func (cs *containerdStore) Cleanup() error {
	return nil
}

func (cs *containerdStore) GraphDriverName() string {
	return "containerd-snapshotter"
}

func (cs *containerdStore) CommitBuildStep(c backend.CommitConfig) (image.ID, error) {
	panic("not implemented")
}

func (cs *containerdStore) CreateImage(config []byte, parent string) (builder.Image, error) {
	panic("not implemented")
}

func (cs *containerdStore) GetImageAndReleasableLayer(ctx context.Context, refOrID string, opts backend.GetImageAndLayerOptions) (builder.Image, builder.ROLayer, error) {
	panic("not implemented")
}

func (cs *containerdStore) MakeImageCache(ctx context.Context, cacheFrom []string) builder.ImageCache {
	panic("not implemented")
}

func (cs *containerdStore) TagImageWithReference(imageID image.ID, newTag reference.Named) error {
	panic("not implemented")
}

func (cs *containerdStore) SquashImage(id, parent string) (string, error) {
	panic("not implemented")
}

func (cs *containerdStore) ExportImage(names []string, outStream io.Writer) error {
	panic("not implemented")
}

func (cs *containerdStore) ImageDelete(imageRef string, force, prune bool) ([]types.ImageDeleteResponseItem, error) {
	panic("not implemented")
}

func (cs *containerdStore) ImageHistory(name string) ([]*imagetype.HistoryResponseItem, error) {
	panic("not implemented")
}

func (cs *containerdStore) ImagesPrune(ctx context.Context, pruneFilters filters.Args) (*types.ImagesPruneReport, error) {
	panic("not implemented")
}

func (cs *containerdStore) ImportImage(ctx context.Context, src string, repository string, platform *v1.Platform, tag string, msg string, inConfig io.ReadCloser, outStream io.Writer, changes []string) error {
	panic("not implemented")
}

func (cs *containerdStore) LoadImage(inTar io.ReadCloser, outStream io.Writer, quiet bool) error {
	panic("not implemented")
}

func (cs *containerdStore) LookupImage(ctx context.Context, name string) (*types.ImageInspect, error) {
	panic("not implemented")
}

func (cs *containerdStore) PushImage(ctx context.Context, image, tag string, metaHeaders map[string][]string, authConfig *types.AuthConfig, outStream io.Writer) error {
	panic("not implemented")
}

func (cs *containerdStore) SearchRegistryForImages(ctx context.Context, searchFilters filters.Args, term string, limit int, authConfig *types.AuthConfig, metaHeaders map[string][]string) (*registrytypes.SearchResults, error) {
	panic("not implemented")
}

func (cs *containerdStore) TagImage(imageName, repository, tag string) (string, error) {
	panic("not implemented")
}

func (cs *containerdStore) GetRepository(context.Context, reference.Named, *types.AuthConfig) (distribution.Repository, error) {
	panic("not implemented")
}

func (cs *containerdStore) ImageDiskUsage(ctx context.Context) ([]*types.ImageSummary, error) {
	panic("not implemented")
}

func (cs *containerdStore) LayerDiskUsage(ctx context.Context) (int64, error) {
	panic("not implemented")
}

func (cs *containerdStore) ReleaseLayer(rwlayer layer.RWLayer) error {
	panic("not implemented")
}

func (cs *containerdStore) CommitImage(c backend.CommitConfig) (image.ID, error) {
	panic("not implemented")
}

func (cs *containerdStore) GetImage(ctx context.Context, refOrID string, platform *v1.Platform) (*image.Image, error) {
	desc, err := cs.ResolveImage(ctx, refOrID)
	if err != nil {
		return nil, err
	}

	ctrdimg, err := cs.resolveImageName2(ctx, refOrID)
	if err != nil {
		return nil, err
	}
	ii := containerd.NewImage(cs.client, ctrdimg)
	provider := cs.client.ContentStore()
	conf, err := ctrdimg.Config(ctx, provider, ii.Platform())
	if err != nil {
		return nil, err
	}

	var ociimage v1.Image
	imageConfigBytes, err := content.ReadBlob(ctx, ii.ContentStore(), conf)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(imageConfigBytes, &ociimage); err != nil {
		return nil, err
	}

	return &image.Image{
		V1Image: image.V1Image{
			ID:           string(desc.Digest),
			OS:           ociimage.OS,
			Architecture: ociimage.Architecture,
			Config: &containertypes.Config{
				Entrypoint: ociimage.Config.Entrypoint,
				Env:        ociimage.Config.Env,
				Cmd:        ociimage.Config.Cmd,
				User:       ociimage.Config.User,
				WorkingDir: ociimage.Config.WorkingDir,
			},
		},
	}, nil
}

func (cs *containerdStore) CreateLayer(container *container.Container, initFunc layer.MountInit) (layer.RWLayer, error) {
	panic("not implemented")
}

func (cs *containerdStore) DistributionServices() images.DistributionServices {
	return images.DistributionServices{}
}

func (cs *containerdStore) CountImages() int {
	imgs, err := cs.client.ListImages(context.TODO())
	if err != nil {
		return 0
	}

	return len(imgs)
}

func (cs *containerdStore) LayerStoreStatus() [][2]string {
	return [][2]string{}
}

func (cs *containerdStore) GetContainerLayerSize(containerID string) (int64, int64) {
	panic("not implemented")
}

func (cs *containerdStore) UpdateConfig(maxDownloads, maxUploads int) {
	panic("not implemented")
}

func (cs *containerdStore) Children(id image.ID) []image.ID {
	panic("not implemented")
}

// ResolveImage searches for an image based on the given
// reference or identifier. Returns the descriptor of
// the image, could be manifest list, manifest, or config.
func (cs *containerdStore) ResolveImage(ctx context.Context, refOrID string) (d ocispec.Descriptor, err error) {
	d, _, err = cs.resolveImageName(ctx, refOrID)
	return
}

func (cs *containerdStore) resolveImageName2(ctx context.Context, refOrID string) (img containerdimages.Image, err error) {
	parsed, err := reference.ParseAnyReference(refOrID)
	if err != nil {
		return img, errdefs.InvalidParameter(err)
	}

	is := cs.client.ImageService()

	namedRef, ok := parsed.(reference.Named)
	if !ok {
		digested, ok := parsed.(reference.Digested)
		if !ok {
			return img, errdefs.InvalidParameter(errors.New("bad reference"))
		}

		imgs, err := is.List(ctx, fmt.Sprintf("target.digest==%s", digested.Digest()))
		if err != nil {
			return img, errors.Wrap(err, "failed to lookup digest")
		}
		if len(imgs) == 0 {
			return img, errdefs.NotFound(errors.New("image not found with digest"))
		}

		return imgs[0], nil
	}

	namedRef = reference.TagNameOnly(namedRef)

	// If the identifier could be a short ID, attempt to match
	if shortID.MatchString(refOrID) {
		ref := namedRef.String()
		filters := []string{
			fmt.Sprintf("name==%q", ref),
			fmt.Sprintf(`target.digest~=/sha256:%s[0-9a-fA-F]{%d}/`, refOrID, 64-len(refOrID)),
		}
		imgs, err := is.List(ctx, filters...)
		if err != nil {
			return img, err
		}

		if len(imgs) == 0 {
			return img, errdefs.NotFound(errors.New("list returned no images"))
		}
		if len(imgs) > 1 {
			digests := map[digest.Digest]struct{}{}
			for _, img := range imgs {
				if img.Name == ref {
					return img, nil
				}
				digests[img.Target.Digest] = struct{}{}
			}

			if len(digests) > 1 {
				return img, errdefs.NotFound(errors.New("ambiguous reference"))
			}
		}

		if imgs[0].Name != ref {
			namedRef = nil
		}
		return imgs[0], nil
	}
	img, err = is.Get(ctx, namedRef.String())
	if err != nil {
		// TODO(containerd): error translation can use common function
		if !cerrdefs.IsNotFound(err) {
			return img, err
		}
		return img, errdefs.NotFound(errors.New("id not found"))
	}

	return img, nil
}

func (cs *containerdStore) resolveImageName(ctx context.Context, refOrID string) (ocispec.Descriptor, reference.Named, error) {
	parsed, err := reference.ParseAnyReference(refOrID)
	if err != nil {
		return ocispec.Descriptor{}, nil, errdefs.InvalidParameter(err)
	}

	is := cs.client.ImageService()

	namedRef, ok := parsed.(reference.Named)
	if !ok {
		digested, ok := parsed.(reference.Digested)
		if !ok {
			return ocispec.Descriptor{}, nil, errdefs.InvalidParameter(errors.New("bad reference"))
		}

		imgs, err := is.List(ctx, fmt.Sprintf("target.digest==%s", digested.Digest()))
		if err != nil {
			return ocispec.Descriptor{}, nil, errors.Wrap(err, "failed to lookup digest")
		}
		if len(imgs) == 0 {
			return ocispec.Descriptor{}, nil, errdefs.NotFound(errors.New("image not found with digest"))
		}

		return imgs[0].Target, nil, nil
	}

	namedRef = reference.TagNameOnly(namedRef)

	// If the identifier could be a short ID, attempt to match
	if shortID.MatchString(refOrID) {
		ref := namedRef.String()
		filters := []string{
			fmt.Sprintf("name==%q", ref),
			fmt.Sprintf(`target.digest~=/sha256:%s[0-9a-fA-F]{%d}/`, refOrID, 64-len(refOrID)),
		}
		imgs, err := is.List(ctx, filters...)
		if err != nil {
			return ocispec.Descriptor{}, nil, err
		}

		if len(imgs) == 0 {
			return ocispec.Descriptor{}, nil, errdefs.NotFound(errors.New("list returned no images"))
		}
		if len(imgs) > 1 {
			digests := map[digest.Digest]struct{}{}
			for _, img := range imgs {
				if img.Name == ref {
					return img.Target, namedRef, nil
				}
				digests[img.Target.Digest] = struct{}{}
			}

			if len(digests) > 1 {
				return ocispec.Descriptor{}, nil, errdefs.NotFound(errors.New("ambiguous reference"))
			}
		}

		if imgs[0].Name != ref {
			namedRef = nil
		}
		return imgs[0].Target, namedRef, nil
	}
	img, err := is.Get(ctx, namedRef.String())
	if err != nil {
		// TODO(containerd): error translation can use common function
		if !cerrdefs.IsNotFound(err) {
			return ocispec.Descriptor{}, nil, err
		}
		return ocispec.Descriptor{}, nil, errdefs.NotFound(errors.New("id not found"))
	}

	return img.Target, namedRef, nil
}
