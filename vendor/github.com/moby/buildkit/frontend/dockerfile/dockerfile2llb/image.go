package dockerfile2llb

import (
	"slices"

	"github.com/moby/buildkit/util/system"
	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

func clone(src dockerspec.DockerOCIImage) dockerspec.DockerOCIImage {
	img := src
	img.Config = src.Config
	img.Config.Env = slices.Clone(src.Config.Env)
	img.Config.Cmd = slices.Clone(src.Config.Cmd)
	img.Config.Entrypoint = slices.Clone(src.Config.Entrypoint)
	img.Config.OnBuild = slices.Clone(src.Config.OnBuild)
	return img
}

func cloneX(src *dockerspec.DockerOCIImage) *dockerspec.DockerOCIImage {
	if src == nil {
		return nil
	}
	img := clone(*src)
	return &img
}

func emptyImage(platform ocispecs.Platform) dockerspec.DockerOCIImage {
	img := dockerspec.DockerOCIImage{}
	img.Architecture = platform.Architecture
	img.OS = platform.OS
	img.OSVersion = platform.OSVersion
	if platform.OSFeatures != nil {
		img.OSFeatures = slices.Clone(platform.OSFeatures)
	}
	img.Variant = platform.Variant
	img.RootFS.Type = "layers"
	img.Config.WorkingDir = "/"
	// don't set path for Windows, leave it to the OS. #5445
	if platform.OS != "windows" {
		img.Config.Env = []string{"PATH=" + system.DefaultPathEnv(platform.OS)}
	}
	return img
}
