package dockerfile2llb

import (
	"github.com/moby/buildkit/util/system"
	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

func clone(src dockerspec.DockerOCIImage) dockerspec.DockerOCIImage {
	img := src
	img.Config = src.Config
	img.Config.Env = append([]string{}, src.Config.Env...)
	img.Config.Cmd = append([]string{}, src.Config.Cmd...)
	img.Config.Entrypoint = append([]string{}, src.Config.Entrypoint...)
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
		img.OSFeatures = append([]string{}, platform.OSFeatures...)
	}
	img.Variant = platform.Variant
	img.RootFS.Type = "layers"
	img.Config.WorkingDir = "/"
	img.Config.Env = []string{"PATH=" + system.DefaultPathEnv(platform.OS)}
	return img
}
