package dockerfile2llb

import (
	"github.com/containerd/containerd/platforms"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

type platformOpt struct {
	targetPlatform ocispecs.Platform
	buildPlatforms []ocispecs.Platform
	implicitTarget bool
}

func buildPlatformOpt(opt *ConvertOpt) *platformOpt {
	buildPlatforms := opt.BuildPlatforms
	targetPlatform := opt.TargetPlatform
	implicitTargetPlatform := false

	if opt.TargetPlatform != nil && opt.BuildPlatforms == nil {
		buildPlatforms = []ocispecs.Platform{*opt.TargetPlatform}
	}
	if len(buildPlatforms) == 0 {
		buildPlatforms = []ocispecs.Platform{platforms.DefaultSpec()}
	}

	if opt.TargetPlatform == nil {
		implicitTargetPlatform = true
		targetPlatform = &buildPlatforms[0]
	}

	return &platformOpt{
		targetPlatform: *targetPlatform,
		buildPlatforms: buildPlatforms,
		implicitTarget: implicitTargetPlatform,
	}
}

func getPlatformArgs(po *platformOpt) []instructions.KeyValuePairOptional {
	bp := po.buildPlatforms[0]
	tp := po.targetPlatform
	m := map[string]string{
		"BUILDPLATFORM":  platforms.Format(bp),
		"BUILDOS":        bp.OS,
		"BUILDARCH":      bp.Architecture,
		"BUILDVARIANT":   bp.Variant,
		"TARGETPLATFORM": platforms.Format(tp),
		"TARGETOS":       tp.OS,
		"TARGETARCH":     tp.Architecture,
		"TARGETVARIANT":  tp.Variant,
	}
	opts := make([]instructions.KeyValuePairOptional, 0, len(m))
	for k, v := range m {
		s := v
		opts = append(opts, instructions.KeyValuePairOptional{Key: k, Value: &s})
	}
	return opts
}
