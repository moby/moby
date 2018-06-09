package builder

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/docker/docker/builder/dockerignore"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/dockerfile/dockerfile2llb"
	"github.com/moby/buildkit/frontend/gateway/client"
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
)

var httpPrefix = regexp.MustCompile("^https?://")
var gitUrlPathWithFragmentSuffix = regexp.MustCompile(".git(?:#.+)?$")

func Build(ctx context.Context, c client.Client) error {
	opts := c.Opts()

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
		llb.SessionID(c.SessionID()),
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
			return err
		}
		ref, err := c.Solve(ctx, client.SolveRequest{
			Definition: def.ToPB(),
		}, nil, false)
		if err != nil {
			return err
		}

		dt, err := ref.ReadFile(ctx, client.ReadRequest{
			Filename: "context",
			Range: &client.FileRange{
				Length: 1024,
			},
		})
		if err != nil {
			return err
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
		return err
	}

	eg, ctx2 := errgroup.WithContext(ctx)
	var dtDockerfile []byte
	eg.Go(func() error {
		ref, err := c.Solve(ctx2, client.SolveRequest{
			Definition: def.ToPB(),
		}, nil, false)
		if err != nil {
			return err
		}

		dtDockerfile, err = ref.ReadFile(ctx2, client.ReadRequest{
			Filename: filename,
		})
		if err != nil {
			return err
		}
		return nil
	})
	var excludes []string
	if !isScratchContext {
		eg.Go(func() error {
			dockerignoreState := buildContext
			if dockerignoreState == nil {
				st := llb.Local(LocalNameContext,
					llb.SessionID(c.SessionID()),
					llb.IncludePatterns([]string{dockerignoreFilename}),
					llb.SharedKeyHint(dockerignoreFilename),
				)
				dockerignoreState = &st
			}
			def, err := dockerignoreState.Marshal()
			if err != nil {
				return err
			}
			ref, err := c.Solve(ctx2, client.SolveRequest{
				Definition: def.ToPB(),
			}, nil, false)
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
		return err
	}

	if _, ok := c.Opts()["cmdline"]; !ok {
		ref, cmdline, ok := dockerfile2llb.DetectSyntax(bytes.NewBuffer(dtDockerfile))
		if ok {
			return forwardGateway(ctx, c, ref, cmdline)
		}
	}

	st, img, err := dockerfile2llb.Dockerfile2LLB(ctx, dtDockerfile, dockerfile2llb.ConvertOpt{
		Target:       opts[keyTarget],
		MetaResolver: c,
		BuildArgs:    filter(opts, buildArgPrefix),
		Labels:       filter(opts, labelPrefix),
		SessionID:    c.SessionID(),
		BuildContext: buildContext,
		Excludes:     excludes,
		IgnoreCache:  ignoreCache,
	})

	if err != nil {
		return err
	}

	def, err = st.Marshal()
	if err != nil {
		return err
	}

	config, err := json.Marshal(img)
	if err != nil {
		return err
	}

	var cacheFrom []string
	if cacheFromStr := opts[keyCacheFrom]; cacheFromStr != "" {
		cacheFrom = strings.Split(cacheFromStr, ",")
	}

	_, err = c.Solve(ctx, client.SolveRequest{
		Definition:      def.ToPB(),
		ImportCacheRefs: cacheFrom,
	}, map[string][]byte{
		exporterImageConfig: config,
	}, true)
	if err != nil {
		return err
	}
	return nil
}

func forwardGateway(ctx context.Context, c client.Client, ref string, cmdline string) error {
	opts := c.Opts()
	if opts == nil {
		opts = map[string]string{}
	}
	opts["cmdline"] = cmdline
	opts["source"] = ref
	_, err := c.Solve(ctx, client.SolveRequest{
		Frontend:    "gateway.v0",
		FrontendOpt: opts,
	}, nil, true)
	return err
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
