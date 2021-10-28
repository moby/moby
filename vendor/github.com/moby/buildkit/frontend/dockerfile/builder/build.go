package builder

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/containerd/containerd/platforms"
	"github.com/docker/go-units"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/frontend/dockerfile/dockerfile2llb"
	"github.com/moby/buildkit/frontend/dockerfile/dockerignore"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/moby/buildkit/frontend/gateway/client"
	gwpb "github.com/moby/buildkit/frontend/gateway/pb"
	"github.com/moby/buildkit/solver/errdefs"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/apicaps"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

const (
	DefaultLocalNameContext    = "context"
	DefaultLocalNameDockerfile = "dockerfile"
	defaultDockerfileName      = "Dockerfile"
	dockerignoreFilename       = ".dockerignore"

	buildArgPrefix = "build-arg:"
	labelPrefix    = "label:"

	keyTarget            = "target"
	keyFilename          = "filename"
	keyCacheFrom         = "cache-from"    // for registry only. deprecated in favor of keyCacheImports
	keyCacheImports      = "cache-imports" // JSON representation of []CacheOptionsEntry
	keyContextSubDir     = "contextsubdir"
	keyForceNetwork      = "force-network-mode"
	keyGlobalAddHosts    = "add-hosts"
	keyHostname          = "hostname"
	keyImageResolveMode  = "image-resolve-mode"
	keyMultiPlatform     = "multi-platform"
	keyNameContext       = "contextkey"
	keyNameDockerfile    = "dockerfilekey"
	keyNoCache           = "no-cache"
	keyOverrideCopyImage = "override-copy-image" // remove after CopyOp implemented
	keyShmSize           = "shm-size"
	keyTargetPlatform    = "platform"
	keyUlimit            = "ulimit"

	// Don't forget to update frontend documentation if you add
	// a new build-arg: frontend/dockerfile/docs/syntax.md
	keyCacheNSArg           = "build-arg:BUILDKIT_CACHE_MOUNT_NS"
	keyContextKeepGitDirArg = "build-arg:BUILDKIT_CONTEXT_KEEP_GIT_DIR"
	keyHostnameArg          = "build-arg:BUILDKIT_SANDBOX_HOSTNAME"
	keyMultiPlatformArg     = "build-arg:BUILDKIT_MULTI_PLATFORM"
	keySyntaxArg            = "build-arg:BUILDKIT_SYNTAX"
)

var httpPrefix = regexp.MustCompile(`^https?://`)
var gitURLPathWithFragmentSuffix = regexp.MustCompile(`\.git(?:#.+)?$`)

func Build(ctx context.Context, c client.Client) (*client.Result, error) {
	opts := c.BuildOpts().Opts
	caps := c.BuildOpts().LLBCaps
	gwcaps := c.BuildOpts().Caps

	allowForward, capsError := validateCaps(opts["frontend.caps"])
	if !allowForward && capsError != nil {
		return nil, capsError
	}

	marshalOpts := []llb.ConstraintsOpt{llb.WithCaps(caps)}

	localNameContext := DefaultLocalNameContext
	if v, ok := opts[keyNameContext]; ok {
		localNameContext = v
	}

	forceLocalDockerfile := false
	localNameDockerfile := DefaultLocalNameDockerfile
	if v, ok := opts[keyNameDockerfile]; ok {
		forceLocalDockerfile = true
		localNameDockerfile = v
	}

	defaultBuildPlatform := platforms.DefaultSpec()
	if workers := c.BuildOpts().Workers; len(workers) > 0 && len(workers[0].Platforms) > 0 {
		defaultBuildPlatform = workers[0].Platforms[0]
	}

	buildPlatforms := []ocispecs.Platform{defaultBuildPlatform}
	targetPlatforms := []*ocispecs.Platform{nil}
	if v := opts[keyTargetPlatform]; v != "" {
		var err error
		targetPlatforms, err = parsePlatforms(v)
		if err != nil {
			return nil, err
		}
	}

	resolveMode, err := parseResolveMode(opts[keyImageResolveMode])
	if err != nil {
		return nil, err
	}

	extraHosts, err := parseExtraHosts(opts[keyGlobalAddHosts])
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse additional hosts")
	}

	shmSize, err := parseShmSize(opts[keyShmSize])
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse shm size")
	}

	ulimit, err := parseUlimits(opts[keyUlimit])
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse ulimit")
	}

	defaultNetMode, err := parseNetMode(opts[keyForceNetwork])
	if err != nil {
		return nil, err
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

	name := "load build definition from " + filename

	filenames := []string{filename, filename + ".dockerignore"}

	// dockerfile is also supported casing moby/moby#10858
	if path.Base(filename) == defaultDockerfileName {
		filenames = append(filenames, path.Join(path.Dir(filename), strings.ToLower(defaultDockerfileName)))
	}

	src := llb.Local(localNameDockerfile,
		llb.FollowPaths(filenames),
		llb.SessionID(c.BuildOpts().SessionID),
		llb.SharedKeyHint(localNameDockerfile),
		dockerfile2llb.WithInternalName(name),
		llb.Differ(llb.DiffNone, false),
	)

	fileop := useFileOp(opts, &caps)

	var buildContext *llb.State
	isNotLocalContext := false
	if st, ok := detectGitContext(opts[localNameContext], opts[keyContextKeepGitDirArg]); ok {
		if !forceLocalDockerfile {
			src = *st
		}
		buildContext = st
	} else if httpPrefix.MatchString(opts[localNameContext]) {
		httpContext := llb.HTTP(opts[localNameContext], llb.Filename("context"), dockerfile2llb.WithInternalName("load remote build context"))
		def, err := httpContext.Marshal(ctx, marshalOpts...)
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
			return nil, errors.Wrapf(err, "failed to read downloaded context")
		}
		if isArchive(dt) {
			if fileop {
				bc := llb.Scratch().File(llb.Copy(httpContext, "/context", "/", &llb.CopyInfo{
					AttemptUnpack: true,
				}))
				if !forceLocalDockerfile {
					src = bc
				}
				buildContext = &bc
			} else {
				copyImage := opts[keyOverrideCopyImage]
				if copyImage == "" {
					copyImage = dockerfile2llb.DefaultCopyImage
				}
				unpack := llb.Image(copyImage, dockerfile2llb.WithInternalName("helper image for file operations")).
					Run(llb.Shlex("copy --unpack /src/context /out/"), llb.ReadonlyRootFS(), dockerfile2llb.WithInternalName("extracting build context"))
				unpack.AddMount("/src", httpContext, llb.Readonly)
				bc := unpack.AddMount("/out", llb.Scratch())
				if !forceLocalDockerfile {
					src = bc
				}
				buildContext = &bc
			}
		} else {
			filename = "context"
			if !forceLocalDockerfile {
				src = httpContext
			}
			buildContext = &httpContext
			isNotLocalContext = true
		}
	} else if (&gwcaps).Supports(gwpb.CapFrontendInputs) == nil {
		inputs, err := c.Inputs(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get frontend inputs")
		}

		if !forceLocalDockerfile {
			inputDockerfile, ok := inputs[DefaultLocalNameDockerfile]
			if ok {
				src = inputDockerfile
			}
		}

		inputCtx, ok := inputs[DefaultLocalNameContext]
		if ok {
			buildContext = &inputCtx
			isNotLocalContext = true
		}
	}

	if buildContext != nil {
		if sub, ok := opts[keyContextSubDir]; ok {
			buildContext = scopeToSubDir(buildContext, fileop, sub)
		}
	}

	def, err := src.Marshal(ctx, marshalOpts...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal local source")
	}

	var sourceMap *llb.SourceMap

	eg, ctx2 := errgroup.WithContext(ctx)
	var dtDockerfile []byte
	var dtDockerignore []byte
	var dtDockerignoreDefault []byte
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
			fallback := false
			if path.Base(filename) == defaultDockerfileName {
				var err1 error
				dtDockerfile, err1 = ref.ReadFile(ctx2, client.ReadRequest{
					Filename: path.Join(path.Dir(filename), strings.ToLower(defaultDockerfileName)),
				})
				if err1 == nil {
					fallback = true
				}
			}
			if !fallback {
				return errors.Wrapf(err, "failed to read dockerfile")
			}
		}

		sourceMap = llb.NewSourceMap(&src, filename, dtDockerfile)
		sourceMap.Definition = def

		dt, err := ref.ReadFile(ctx2, client.ReadRequest{
			Filename: filename + ".dockerignore",
		})
		if err == nil {
			dtDockerignore = dt
		}
		return nil
	})
	var excludes []string
	if !isNotLocalContext {
		eg.Go(func() error {
			dockerignoreState := buildContext
			if dockerignoreState == nil {
				st := llb.Local(localNameContext,
					llb.SessionID(c.BuildOpts().SessionID),
					llb.FollowPaths([]string{dockerignoreFilename}),
					llb.SharedKeyHint(localNameContext+"-"+dockerignoreFilename),
					dockerfile2llb.WithInternalName("load "+dockerignoreFilename),
					llb.Differ(llb.DiffNone, false),
				)
				dockerignoreState = &st
			}
			def, err := dockerignoreState.Marshal(ctx, marshalOpts...)
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
			dtDockerignoreDefault, err = ref.ReadFile(ctx2, client.ReadRequest{
				Filename: dockerignoreFilename,
			})
			if err != nil {
				return nil
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	if dtDockerignore == nil {
		dtDockerignore = dtDockerignoreDefault
	}
	if dtDockerignore != nil {
		excludes, err = dockerignore.ReadAll(bytes.NewBuffer(dtDockerignore))
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse dockerignore")
		}
	}

	if _, ok := opts["cmdline"]; !ok {
		if cmdline, ok := opts[keySyntaxArg]; ok {
			p := strings.SplitN(strings.TrimSpace(cmdline), " ", 2)
			res, err := forwardGateway(ctx, c, p[0], cmdline)
			if err != nil && len(errdefs.Sources(err)) == 0 {
				return nil, errors.Wrapf(err, "failed with %s = %s", keySyntaxArg, cmdline)
			}
			return res, err
		} else if ref, cmdline, loc, ok := dockerfile2llb.DetectSyntax(bytes.NewBuffer(dtDockerfile)); ok {
			res, err := forwardGateway(ctx, c, ref, cmdline)
			if err != nil && len(errdefs.Sources(err)) == 0 {
				return nil, wrapSource(err, sourceMap, loc)
			}
			return res, err
		}
	}

	if capsError != nil {
		return nil, capsError
	}

	if res, ok, err := checkSubRequest(ctx, opts); ok {
		return res, err
	}

	exportMap := len(targetPlatforms) > 1

	if v := opts[keyMultiPlatformArg]; v != "" {
		opts[keyMultiPlatform] = v
	}
	if v := opts[keyMultiPlatform]; v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, errors.Errorf("invalid boolean value %s", v)
		}
		if !b && exportMap {
			return nil, errors.Errorf("returning multiple target plaforms is not allowed")
		}
		exportMap = b
	}

	expPlatforms := &exptypes.Platforms{
		Platforms: make([]exptypes.Platform, len(targetPlatforms)),
	}
	res := client.NewResult()

	if v, ok := opts[keyHostnameArg]; ok && len(v) > 0 {
		opts[keyHostname] = v
	}

	eg, ctx = errgroup.WithContext(ctx)

	for i, tp := range targetPlatforms {
		func(i int, tp *ocispecs.Platform) {
			eg.Go(func() (err error) {
				defer func() {
					var el *parser.ErrorLocation
					if errors.As(err, &el) {
						err = wrapSource(err, sourceMap, el.Location)
					}
				}()
				st, img, err := dockerfile2llb.Dockerfile2LLB(ctx, dtDockerfile, dockerfile2llb.ConvertOpt{
					Target:            opts[keyTarget],
					MetaResolver:      c,
					BuildArgs:         filter(opts, buildArgPrefix),
					Labels:            filter(opts, labelPrefix),
					CacheIDNamespace:  opts[keyCacheNSArg],
					SessionID:         c.BuildOpts().SessionID,
					BuildContext:      buildContext,
					Excludes:          excludes,
					IgnoreCache:       ignoreCache,
					TargetPlatform:    tp,
					BuildPlatforms:    buildPlatforms,
					ImageResolveMode:  resolveMode,
					PrefixPlatform:    exportMap,
					ExtraHosts:        extraHosts,
					ShmSize:           shmSize,
					Ulimit:            ulimit,
					ForceNetMode:      defaultNetMode,
					OverrideCopyImage: opts[keyOverrideCopyImage],
					LLBCaps:           &caps,
					SourceMap:         sourceMap,
					Hostname:          opts[keyHostname],
				})

				if err != nil {
					return err
				}

				def, err := st.Marshal(ctx)
				if err != nil {
					return errors.Wrapf(err, "failed to marshal LLB definition")
				}

				config, err := json.Marshal(img)
				if err != nil {
					return errors.Wrapf(err, "failed to marshal image config")
				}

				var cacheImports []client.CacheOptionsEntry
				// new API
				if cacheImportsStr := opts[keyCacheImports]; cacheImportsStr != "" {
					var cacheImportsUM []controlapi.CacheOptionsEntry
					if err := json.Unmarshal([]byte(cacheImportsStr), &cacheImportsUM); err != nil {
						return errors.Wrapf(err, "failed to unmarshal %s (%q)", keyCacheImports, cacheImportsStr)
					}
					for _, um := range cacheImportsUM {
						cacheImports = append(cacheImports, client.CacheOptionsEntry{Type: um.Type, Attrs: um.Attrs})
					}
				}
				// old API
				if cacheFromStr := opts[keyCacheFrom]; cacheFromStr != "" {
					cacheFrom := strings.Split(cacheFromStr, ",")
					for _, s := range cacheFrom {
						im := client.CacheOptionsEntry{
							Type: "registry",
							Attrs: map[string]string{
								"ref": s,
							},
						}
						// FIXME(AkihiroSuda): skip append if already exists
						cacheImports = append(cacheImports, im)
					}
				}

				r, err := c.Solve(ctx, client.SolveRequest{
					Definition:   def.ToPB(),
					CacheImports: cacheImports,
				})
				if err != nil {
					return err
				}

				ref, err := r.SingleRef()
				if err != nil {
					return err
				}

				if !exportMap {
					res.AddMeta(exptypes.ExporterImageConfigKey, config)
					res.SetRef(ref)
				} else {
					p := platforms.DefaultSpec()
					if tp != nil {
						p = *tp
					}

					k := platforms.Format(p)
					res.AddMeta(fmt.Sprintf("%s/%s", exptypes.ExporterImageConfigKey, k), config)
					res.AddRef(k, ref)
					expPlatforms.Platforms[i] = exptypes.Platform{
						ID:       k,
						Platform: p,
					}
				}
				return nil
			})
		}(i, tp)
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	if exportMap {
		dt, err := json.Marshal(expPlatforms)
		if err != nil {
			return nil, err
		}
		res.AddMeta(exptypes.ExporterPlatformsKey, dt)
	}

	return res, nil
}

func forwardGateway(ctx context.Context, c client.Client, ref string, cmdline string) (*client.Result, error) {
	opts := c.BuildOpts().Opts
	if opts == nil {
		opts = map[string]string{}
	}
	opts["cmdline"] = cmdline
	opts["source"] = ref

	gwcaps := c.BuildOpts().Caps
	var frontendInputs map[string]*pb.Definition
	if (&gwcaps).Supports(gwpb.CapFrontendInputs) == nil {
		inputs, err := c.Inputs(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get frontend inputs")
		}

		frontendInputs = make(map[string]*pb.Definition)
		for name, state := range inputs {
			def, err := state.Marshal(ctx)
			if err != nil {
				return nil, err
			}
			frontendInputs[name] = def.ToPB()
		}
	}

	return c.Solve(ctx, client.SolveRequest{
		Frontend:       "gateway.v0",
		FrontendOpt:    opts,
		FrontendInputs: frontendInputs,
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

func detectGitContext(ref, gitContext string) (*llb.State, bool) {
	found := false
	if httpPrefix.MatchString(ref) && gitURLPathWithFragmentSuffix.MatchString(ref) {
		found = true
	}

	keepGit := false
	if gitContext != "" {
		if v, err := strconv.ParseBool(gitContext); err == nil {
			keepGit = v
		}
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
	gitOpts := []llb.GitOption{dockerfile2llb.WithInternalName("load git source " + ref)}
	if keepGit {
		gitOpts = append(gitOpts, llb.KeepGitDir())
	}

	st := llb.Git(parts[0], branch, gitOpts...)
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

func parsePlatforms(v string) ([]*ocispecs.Platform, error) {
	var pp []*ocispecs.Platform
	for _, v := range strings.Split(v, ",") {
		p, err := platforms.Parse(v)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse target platform %s", v)
		}
		p = platforms.Normalize(p)
		pp = append(pp, &p)
	}
	return pp, nil
}

func parseResolveMode(v string) (llb.ResolveMode, error) {
	switch v {
	case pb.AttrImageResolveModeDefault, "":
		return llb.ResolveModeDefault, nil
	case pb.AttrImageResolveModeForcePull:
		return llb.ResolveModeForcePull, nil
	case pb.AttrImageResolveModePreferLocal:
		return llb.ResolveModePreferLocal, nil
	default:
		return 0, errors.Errorf("invalid image-resolve-mode: %s", v)
	}
}

func parseExtraHosts(v string) ([]llb.HostIP, error) {
	if v == "" {
		return nil, nil
	}
	out := make([]llb.HostIP, 0)
	csvReader := csv.NewReader(strings.NewReader(v))
	fields, err := csvReader.Read()
	if err != nil {
		return nil, err
	}
	for _, field := range fields {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 {
			return nil, errors.Errorf("invalid key-value pair %s", field)
		}
		key := strings.ToLower(parts[0])
		val := strings.ToLower(parts[1])
		ip := net.ParseIP(val)
		if ip == nil {
			return nil, errors.Errorf("failed to parse IP %s", val)
		}
		out = append(out, llb.HostIP{Host: key, IP: ip})
	}
	return out, nil
}

func parseShmSize(v string) (int64, error) {
	if len(v) == 0 {
		return 0, nil
	}
	kb, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, err
	}
	return kb, nil
}

func parseUlimits(v string) ([]pb.Ulimit, error) {
	if v == "" {
		return nil, nil
	}
	out := make([]pb.Ulimit, 0)
	csvReader := csv.NewReader(strings.NewReader(v))
	fields, err := csvReader.Read()
	if err != nil {
		return nil, err
	}
	for _, field := range fields {
		ulimit, err := units.ParseUlimit(field)
		if err != nil {
			return nil, err
		}
		out = append(out, pb.Ulimit{
			Name: ulimit.Name,
			Soft: ulimit.Soft,
			Hard: ulimit.Hard,
		})
	}
	return out, nil
}

func parseNetMode(v string) (pb.NetMode, error) {
	if v == "" {
		return llb.NetModeSandbox, nil
	}
	switch v {
	case "none":
		return llb.NetModeNone, nil
	case "host":
		return llb.NetModeHost, nil
	case "sandbox":
		return llb.NetModeSandbox, nil
	default:
		return 0, errors.Errorf("invalid netmode %s", v)
	}
}

func useFileOp(args map[string]string, caps *apicaps.CapSet) bool {
	enabled := true
	if v, ok := args["build-arg:BUILDKIT_DISABLE_FILEOP"]; ok {
		if b, err := strconv.ParseBool(v); err == nil {
			enabled = !b
		}
	}
	return enabled && caps != nil && caps.Supports(pb.CapFileBase) == nil
}

func scopeToSubDir(c *llb.State, fileop bool, dir string) *llb.State {
	if fileop {
		bc := llb.Scratch().File(llb.Copy(*c, dir, "/", &llb.CopyInfo{
			CopyDirContentsOnly: true,
		}))
		return &bc
	}
	unpack := llb.Image(dockerfile2llb.DefaultCopyImage, dockerfile2llb.WithInternalName("helper image for file operations")).
		Run(llb.Shlexf("copy %s/. /out/", path.Join("/src", dir)), llb.ReadonlyRootFS(), dockerfile2llb.WithInternalName("filtering build context"))
	unpack.AddMount("/src", *c, llb.Readonly)
	bc := unpack.AddMount("/out", llb.Scratch())
	return &bc
}

func wrapSource(err error, sm *llb.SourceMap, ranges []parser.Range) error {
	if sm == nil {
		return err
	}
	s := errdefs.Source{
		Info: &pb.SourceInfo{
			Data:       sm.Data,
			Filename:   sm.Filename,
			Definition: sm.Definition.ToPB(),
		},
		Ranges: make([]*pb.Range, 0, len(ranges)),
	}
	for _, r := range ranges {
		s.Ranges = append(s.Ranges, &pb.Range{
			Start: pb.Position{
				Line:      int32(r.Start.Line),
				Character: int32(r.Start.Character),
			},
			End: pb.Position{
				Line:      int32(r.End.Line),
				Character: int32(r.End.Character),
			},
		})
	}
	return errdefs.WithSource(err, s)
}
