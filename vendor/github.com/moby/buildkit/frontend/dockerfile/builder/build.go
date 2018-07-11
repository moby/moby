package builder

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/containerd/containerd/platforms"
	"github.com/docker/docker/builder/dockerignore"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/dockerfile/dockerfile2llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

const (
	LocalNameContext      = "context"
	LocalNameDockerfile   = "dockerfile"
	keyTarget             = "target"
	keyFilename           = "filename"
	keyCacheFrom          = "cache-from"
	exporterImageConfig   = "containerimage.config"
	defaultDockerfileName = "Dockerfile"
	dockerignoreFilename  = ".dockerignore"
	buildArgPrefix        = "build-arg:"
	labelPrefix           = "label:"
	keyNoCache            = "no-cache"
	keyTargetPlatform     = "platform"
)

var httpPrefix = regexp.MustCompile("^https?://")
var gitUrlPathWithFragmentSuffix = regexp.MustCompile("\\.git(?:#.+)?$")

func Build(ctx context.Context, c client.Client) (*client.Result, error) {
	opts := c.BuildOpts().Opts

	defaultBuildPlatform := platforms.DefaultSpec()
	if workers := c.BuildOpts().Workers; len(workers) > 0 && len(workers[0].Platforms) > 0 {
		defaultBuildPlatform = workers[0].Platforms[0]
	}

	buildPlatforms := []specs.Platform{defaultBuildPlatform}
	targetPlatform := platforms.DefaultSpec()
	if v := opts[keyTargetPlatform]; v != "" {
		var err error
		targetPlatform, err = platforms.Parse(v)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse target platform %s", v)
		}
	}

	filename := opts[keyFilename]
	if filename == "" {
		filename = defaultDockerfileName
	}

	var ignoreCache []string
	if v, ok := opts[keyNoCache]; ok {
		if v == "" {
			ignoreCache = []string{} // means all stages
		} else {
			ignoreCache = strings.Split(v, ",")
		}
	}

	src := llb.Local(LocalNameDockerfile,
		llb.IncludePatterns([]string{filename}),
		llb.SessionID(c.BuildOpts().SessionID),
		llb.SharedKeyHint(defaultDockerfileName),
	)
	var buildContext *llb.State
	isScratchContext := false
	if st, ok := detectGitContext(opts[LocalNameContext]); ok {
		src = *st
		buildContext = &src
	} else if httpPrefix.MatchString(opts[LocalNameContext]) {
		httpContext := llb.HTTP(opts[LocalNameContext], llb.Filename("context"))
		def, err := httpContext.Marshal()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal httpcontext")
		}
		res, err := c.Solve(ctx, client.SolveRequest{
			Definition: def.ToPB(),
		})
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve httpcontext")
		}

		ref, err := res.SingleRef()
		if err != nil {
			return nil, err
		}

		dt, err := ref.ReadFile(ctx, client.ReadRequest{
			Filename: "context",
			Range: &client.FileRange{
				Length: 1024,
			},
		})
		if err != nil {
			return nil, errors.Errorf("failed to read downloaded context")
		}
		if isArchive(dt) {
			unpack := llb.Image(dockerfile2llb.CopyImage).
				Run(llb.Shlex("copy --unpack /src/context /out/"), llb.ReadonlyRootFS())
			unpack.AddMount("/src", httpContext, llb.Readonly)
			src = unpack.AddMount("/out", llb.Scratch())
			buildContext = &src
		} else {
			filename = "context"
			src = httpContext
			buildContext = &src
			isScratchContext = true
		}
	}

	def, err := src.Marshal()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal local source")
	}

	eg, ctx2 := errgroup.WithContext(ctx)
	var dtDockerfile []byte
	eg.Go(func() error {
		res, err := c.Solve(ctx2, client.SolveRequest{
			Definition: def.ToPB(),
		})
		if err != nil {
			return errors.Wrapf(err, "failed to resolve dockerfile")
		}

		ref, err := res.SingleRef()
		if err != nil {
			return err
		}

		dtDockerfile, err = ref.ReadFile(ctx2, client.ReadRequest{
			Filename: filename,
		})
		if err != nil {
			return errors.Wrapf(err, "failed to read dockerfile")
		}
		return nil
	})
	var excludes []string
	if !isScratchContext {
		eg.Go(func() error {
			dockerignoreState := buildContext
			if dockerignoreState == nil {
				st := llb.Local(LocalNameContext,
					llb.SessionID(c.BuildOpts().SessionID),
					llb.IncludePatterns([]string{dockerignoreFilename}),
					llb.SharedKeyHint(dockerignoreFilename),
				)
				dockerignoreState = &st
			}
			def, err := dockerignoreState.Marshal()
			if err != nil {
				return err
			}
			res, err := c.Solve(ctx2, client.SolveRequest{
				Definition: def.ToPB(),
			})
			if err != nil {
				return err
			}
			ref, err := res.SingleRef()
			if err != nil {
				return err
			}
			dtDockerignore, err := ref.ReadFile(ctx2, client.ReadRequest{
				Filename: dockerignoreFilename,
			})
			if err == nil {
				excludes, err = dockerignore.ReadAll(bytes.NewBuffer(dtDockerignore))
				if err != nil {
					return errors.Wrap(err, "failed to parse dockerignore")
				}
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	if _, ok := opts["cmdline"]; !ok {
		ref, cmdline, ok := dockerfile2llb.DetectSyntax(bytes.NewBuffer(dtDockerfile))
		if ok {
			return forwardGateway(ctx, c, ref, cmdline)
		}
	}

	st, img, err := dockerfile2llb.Dockerfile2LLB(ctx, dtDockerfile, dockerfile2llb.ConvertOpt{
		Target:         opts[keyTarget],
		MetaResolver:   c,
		BuildArgs:      filter(opts, buildArgPrefix),
		Labels:         filter(opts, labelPrefix),
		SessionID:      c.BuildOpts().SessionID,
		BuildContext:   buildContext,
		Excludes:       excludes,
		IgnoreCache:    ignoreCache,
		TargetPlatform: &targetPlatform,
		BuildPlatforms: buildPlatforms,
	})

	if err != nil {
		return nil, errors.Wrapf(err, "failed to create LLB definition")
	}

	def, err = st.Marshal()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal LLB definition")
	}

	config, err := json.Marshal(img)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal image config")
	}

	var cacheFrom []string
	if cacheFromStr := opts[keyCacheFrom]; cacheFromStr != "" {
		cacheFrom = strings.Split(cacheFromStr, ",")
	}

	res, err := c.Solve(ctx, client.SolveRequest{
		Definition:      def.ToPB(),
		ImportCacheRefs: cacheFrom,
	})
	if err != nil {
		return nil, err
	}

	res.AddMeta(exporterImageConfig, config)

	return res, nil
}

func forwardGateway(ctx context.Context, c client.Client, ref string, cmdline string) (*client.Result, error) {
	opts := c.BuildOpts().Opts
	if opts == nil {
		opts = map[string]string{}
	}
	opts["cmdline"] = cmdline
	opts["source"] = ref
	return c.Solve(ctx, client.SolveRequest{
		Frontend:    "gateway.v0",
		FrontendOpt: opts,
	})
}

func filter(opt map[string]string, key string) map[string]string {
	m := map[string]string{}
	for k, v := range opt {
		if strings.HasPrefix(k, key) {
			m[strings.TrimPrefix(k, key)] = v
		}
	}
	return m
}

func detectGitContext(ref string) (*llb.State, bool) {
	found := false
	if httpPrefix.MatchString(ref) && gitUrlPathWithFragmentSuffix.MatchString(ref) {
		found = true
	}

	for _, prefix := range []string{"git://", "github.com/", "git@"} {
		if strings.HasPrefix(ref, prefix) {
			found = true
			break
		}
	}
	if !found {
		return nil, false
	}

	parts := strings.SplitN(ref, "#", 2)
	branch := ""
	if len(parts) > 1 {
		branch = parts[1]
	}
	st := llb.Git(parts[0], branch)
	return &st, true
}

func isArchive(header []byte) bool {
	for _, m := range [][]byte{
		{0x42, 0x5A, 0x68},                   // bzip2
		{0x1F, 0x8B, 0x08},                   // gzip
		{0xFD, 0x37, 0x7A, 0x58, 0x5A, 0x00}, // xz
	} {
		if len(header) < len(m) {
			continue
		}
		if bytes.Equal(m, header[:len(m)]) {
			return true
		}
	}

	r := tar.NewReader(bytes.NewBuffer(header))
	_, err := r.Next()
	return err == nil
}
