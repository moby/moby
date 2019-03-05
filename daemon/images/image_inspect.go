package images // import "github.com/docker/docker/daemon/images"

import (
	"context"
	"encoding/json"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	containertype "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/go-connections/nat"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// LookupImage looks up an image by name and returns it as an ImageInspect
// structure.
func (i *ImageService) LookupImage(ctx context.Context, name string) (*types.ImageInspect, error) {
	desc, err := i.ResolveImage(ctx, name)
	if err != nil {
		return nil, err
	}

	repoTags := []string{}
	repoDigests := []string{}
	imgs, err := i.client.ImageService().List(ctx, "target.digest=="+desc.Digest.String())
	if err != nil {
		return nil, err
	}
	for _, img := range imgs {
		// Parse name
		ref, err := reference.Parse(img.Name)
		if err != nil {
			log.G(ctx).WithError(err).WithField("target", desc.Digest.String()).Warnf("skipping reference %q", img.Name)
			continue
		}
		switch ref.(type) {
		case reference.Canonical:
			repoDigests = append(repoDigests, reference.FamiliarString(ref))
		case reference.NamedTagged:
			repoTags = append(repoTags, reference.FamiliarString(ref))
		}
	}

	cs := i.client.ContentStore()

	config, err := images.Config(ctx, cs, desc, i.platforms)
	if err != nil {
		log.G(ctx).WithError(err).Debugf("resolve failed")
		return nil, errors.Wrap(err, "failed to resolve config")
	}

	p, err := content.ReadBlob(ctx, cs, config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read config")
	}

	var img struct {
		ocispec.Image

		// Overwrite config for custom Docker fields
		Config imageConfig `json:"config,omitempty"`

		Comment    string   `json:"comment,omitempty"`
		OSVersion  string   `json:"os.version,omitempty"`
		OSFeatures []string `json:"os.features,omitempty"`
		Variant    string   `json:"variant,omitempty"`
		// TODO: Overwrite this with a label from config
		DockerVersion string `json:"docker_version,omitempty"`
	}

	if err := json.Unmarshal(p, &img); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal config")
	}

	var size int64
	var layerMetadata map[string]string
	layerID := identity.ChainID(img.RootFS.DiffIDs)
	if layerID != "" {
		// Read layer store from labels
		//l, err := i.layerStores[runtime.GOOS].Get(layer.ChainID(layerID))
		//if err != nil {
		//	return nil, err
		//}
		//defer layer.ReleaseAndLog(i.layerStores[runtime.GOOS], l)
		//size, err = l.Size()
		//if err != nil {
		//	return nil, err
		//}

		//layerMetadata, err = l.Metadata()
		//if err != nil {
		//	return nil, err
		//}
	}

	comment := img.Comment
	if img.Comment == "" && len(img.History) > 0 {
		comment = img.History[len(img.History)-1].Comment
	}

	// TODO(containerd): Get from label?
	//lastUpdated, err := i.imageStore.GetLastUpdated(img.ID())
	//if err != nil {
	//	return nil, err
	//}

	imageInspect := &types.ImageInspect{
		ID:          desc.Digest.String(),
		RepoTags:    repoTags,
		RepoDigests: repoDigests,
		//Parent:        ci.parent.String(),
		Comment:       comment,
		Created:       img.Created.Format(time.RFC3339Nano),
		DockerVersion: img.DockerVersion,
		Author:        img.Author,
		Config:        configToApiType(img.Config),
		Architecture:  img.Architecture,
		Os:            img.OS,
		OsVersion:     img.OSVersion,
		Size:          size,
		VirtualSize:   size, // TODO: field unused, deprecate
		RootFS:        rootFSToAPIType(img.RootFS),
		// TODO(containerd): Get from labels?
		//Metadata: types.ImageMetadata{
		//	LastTagTime: lastUpdated,
		//},
	}

	//imageInspect.GraphDriver.Name = i.layerStores[runtime.GOOS].DriverName()
	imageInspect.GraphDriver.Data = layerMetadata

	return imageInspect, nil
}

func rootFSToAPIType(rootfs ocispec.RootFS) types.RootFS {
	var layers []string
	for _, l := range rootfs.DiffIDs {
		layers = append(layers, l.String())
	}
	return types.RootFS{
		Type:   rootfs.Type,
		Layers: layers,
	}
}

func configToApiType(c imageConfig) *containertype.Config {
	return &containertype.Config{
		User:         c.User,
		ExposedPorts: portSetToApiType(c.ExposedPorts),
		Env:          c.Env,
		WorkingDir:   c.WorkingDir,
		Labels:       c.Labels,
		StopSignal:   c.StopSignal,
		Volumes:      c.Volumes,
		Entrypoint:   strslice.StrSlice(c.Entrypoint),
		Cmd:          strslice.StrSlice(c.Cmd),

		// From custom Docker type (aligned with what builder sets)
		Healthcheck: c.Healthcheck,
		ArgsEscaped: c.ArgsEscaped,
		OnBuild:     c.OnBuild,
		StopTimeout: c.StopTimeout,
		Shell:       c.Shell,
	}
}

func portSetToApiType(ports map[string]struct{}) nat.PortSet {
	ps := nat.PortSet{}
	for p := range ports {
		ps[nat.Port(p)] = struct{}{}
	}
	return ps
}

// imageConfig is a docker compatible config for an image
type imageConfig struct {
	ocispec.ImageConfig

	// Healthcheck defines healthchecks for the image
	// uses api type which matches what is set by the builder
	Healthcheck *containertype.HealthConfig `json:",omitempty"`

	// ArgsEscaped is true if command is already escaped (Windows specific)
	ArgsEscaped bool `json:",omitempty"`

	// OnBuild is ONBUILD metadata that were defined on the image Dockerfile
	OnBuild []string

	// StopTimeout (in seconds) to stop a container
	StopTimeout *int `json:",omitempty"`

	// Shell for shell-form of RUN, CMD, ENTRYPOINT
	Shell strslice.StrSlice `json:",omitempty"`
}
