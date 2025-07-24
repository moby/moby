package images

import (
	imagespec "github.com/moby/docker-image-spec/specs-go/v1"
	"github.com/moby/moby/api/types/container"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

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
