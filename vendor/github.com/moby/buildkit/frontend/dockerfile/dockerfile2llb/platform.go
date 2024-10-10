package dockerfile2llb

import (
	"github.com/containerd/platforms"
	"github.com/moby/buildkit/client/llb"
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

func platformArgs(po *platformOpt, overrides map[string]string) *llb.EnvList {
	bp := po.buildPlatforms[0]
	tp := po.targetPlatform
	s := [...][2]string{
		{"BUILDPLATFORM", platforms.Format(bp)},
		{"BUILDOS", bp.OS},
		{"BUILDARCH", bp.Architecture},
		{"BUILDVARIANT", bp.Variant},
		{"TARGETPLATFORM", platforms.Format(tp)},
		{"TARGETOS", tp.OS},
		{"TARGETARCH", tp.Architecture},
		{"TARGETVARIANT", tp.Variant},
	}
	env := &llb.EnvList{}
	for _, kv := range s {
		v := kv[1]
		if ov, ok := overrides[kv[0]]; ok {
			v = ov
		}
		env = env.AddOrReplace(kv[0], v)
	}
	return env
}
