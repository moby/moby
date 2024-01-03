package dockerfile2llb

import (
	"github.com/moby/buildkit/exporter/containerimage/image"
	"github.com/moby/buildkit/util/system"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

func clone(src image.Image) image.Image {
	img := src
	img.Config = src.Config
	img.Config.Env = append([]string{}, src.Config.Env...)
	img.Config.Cmd = append([]string{}, src.Config.Cmd...)
	img.Config.Entrypoint = append([]string{}, src.Config.Entrypoint...)
	return img
}

func emptyImage(platform ocispecs.Platform) image.Image {
	img := image.Image{}
	img.Architecture = platform.Architecture
	img.OS = platform.OS
	img.Variant = platform.Variant
	img.RootFS.Type = "layers"
	img.Config.WorkingDir = "/"
	img.Config.Env = []string{"PATH=" + system.DefaultPathEnv(platform.OS)}
	return img
}
