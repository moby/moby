package containerd

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/image"
	imagespec "github.com/docker/docker/image/spec/specs-go/v1"
	"github.com/docker/docker/layer"
	"github.com/docker/go-connections/nat"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// dockerOciImageToDockerImagePartial creates an image.Image from the imagespec.DockerOCIImage
// It doesn't set:
// - V1Image.ContainerConfig
// - V1Image.Container
// - Details
func dockerOciImageToDockerImagePartial(id image.ID, img imagespec.DockerOCIImage) *image.Image {
	v1Image := image.V1Image{
		DockerVersion: dockerversion.Version,
		Config:        dockerOCIImageConfigToContainerConfig(img.Config),
		Architecture:  img.Platform.Architecture,
		Variant:       img.Platform.Variant,
		OS:            img.Platform.OS,
		Author:        img.Author,
		Created:       img.Created,
	}

	rootFS := &image.RootFS{
		Type: img.RootFS.Type,
	}
	for _, diffId := range img.RootFS.DiffIDs {
		rootFS.DiffIDs = append(rootFS.DiffIDs, layer.DiffID(diffId))
	}

	out := image.NewImage(id)
	out.V1Image = v1Image
	out.RootFS = rootFS
	out.History = img.History
	out.OSFeatures = img.OSFeatures
	out.OSVersion = img.OSVersion
	return out
}

func dockerImageToDockerOCIImage(img image.Image) imagespec.DockerOCIImage {
	rootfs := ocispec.RootFS{
		Type:    img.RootFS.Type,
		DiffIDs: []digest.Digest{},
	}
	for _, diffId := range img.RootFS.DiffIDs {
		rootfs.DiffIDs = append(rootfs.DiffIDs, digest.Digest(diffId))
	}

	return imagespec.DockerOCIImage{
		Image: ocispec.Image{
			Created: img.Created,
			Author:  img.Author,
			Platform: ocispec.Platform{
				Architecture: img.Architecture,
				Variant:      img.Variant,
				OS:           img.OS,
				OSVersion:    img.OSVersion,
				OSFeatures:   img.OSFeatures,
			},
			RootFS:  rootfs,
			History: img.History,
		},
		Config: containerConfigToDockerOCIImageConfig(img.Config),
	}
}

func containerConfigToDockerOCIImageConfig(cfg *container.Config) imagespec.DockerOCIImageConfig {
	var ociCfg ocispec.ImageConfig
	var ext imagespec.DockerOCIImageConfigExt

	if cfg != nil {
		ociCfg = ocispec.ImageConfig{
			User:        cfg.User,
			Env:         cfg.Env,
			Entrypoint:  cfg.Entrypoint,
			Cmd:         cfg.Cmd,
			Volumes:     cfg.Volumes,
			WorkingDir:  cfg.WorkingDir,
			Labels:      cfg.Labels,
			StopSignal:  cfg.StopSignal,
			ArgsEscaped: cfg.ArgsEscaped, //nolint:staticcheck // Ignore SA1019. Need to keep it in image.
		}

		if len(cfg.ExposedPorts) > 0 {
			ociCfg.ExposedPorts = map[string]struct{}{}
			for k, v := range cfg.ExposedPorts {
				ociCfg.ExposedPorts[string(k)] = v
			}
		}
		ext.Healthcheck = cfg.Healthcheck
		ext.OnBuild = cfg.OnBuild
		ext.Shell = cfg.Shell
	}

	return imagespec.DockerOCIImageConfig{
		ImageConfig:             ociCfg,
		DockerOCIImageConfigExt: ext,
	}
}

func dockerOCIImageConfigToContainerConfig(cfg imagespec.DockerOCIImageConfig) *container.Config {
	exposedPorts := make(nat.PortSet, len(cfg.ExposedPorts))
	for k, v := range cfg.ExposedPorts {
		exposedPorts[nat.Port(k)] = v
	}

	return &container.Config{
		Entrypoint:   cfg.Entrypoint,
		Env:          cfg.Env,
		Cmd:          cfg.Cmd,
		User:         cfg.User,
		WorkingDir:   cfg.WorkingDir,
		ExposedPorts: exposedPorts,
		Volumes:      cfg.Volumes,
		Labels:       cfg.Labels,
		ArgsEscaped:  cfg.ArgsEscaped, //nolint:staticcheck // Ignore SA1019. Need to keep it in image.
		StopSignal:   cfg.StopSignal,
		Healthcheck:  cfg.Healthcheck,
		OnBuild:      cfg.OnBuild,
		Shell:        cfg.Shell,
	}
}
