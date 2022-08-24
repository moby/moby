package dockerfile2llb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/reference"
	"github.com/docker/go-connections/nat"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/client/llb/imagemetaresolver"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/moby/buildkit/frontend/dockerfile/shell"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/apicaps"
	binfotypes "github.com/moby/buildkit/util/buildinfo/types"
	"github.com/moby/buildkit/util/suggest"
	"github.com/moby/buildkit/util/system"
	"github.com/moby/sys/signal"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

const (
	emptyImageName          = "scratch"
	defaultContextLocalName = "context"
	historyComment          = "buildkit.dockerfile.v0"

	DefaultCopyImage = "docker/dockerfile-copy:v0.1.9@sha256:e8f159d3f00786604b93c675ee2783f8dc194bb565e61ca5788f6a6e9d304061"
)

type ConvertOpt struct {
	Target       string
	MetaResolver llb.ImageMetaResolver
	BuildArgs    map[string]string
	Labels       map[string]string
	SessionID    string
	BuildContext *llb.State
	Excludes     []string
	// IgnoreCache contains names of the stages that should not use build cache.
	// Empty slice means ignore cache for all stages. Nil doesn't disable cache.
	IgnoreCache []string
	// CacheIDNamespace scopes the IDs for different cache mounts
	CacheIDNamespace  string
	ImageResolveMode  llb.ResolveMode
	TargetPlatform    *ocispecs.Platform
	BuildPlatforms    []ocispecs.Platform
	PrefixPlatform    bool
	ExtraHosts        []llb.HostIP
	ShmSize           int64
	Ulimit            []pb.Ulimit
	CgroupParent      string
	ForceNetMode      pb.NetMode
	OverrideCopyImage string
	LLBCaps           *apicaps.CapSet
	ContextLocalName  string
	SourceMap         *llb.SourceMap
	Hostname          string
	Warn              func(short, url string, detail [][]byte, location *parser.Range)
	ContextByName     func(ctx context.Context, name, resolveMode string, p *ocispecs.Platform) (*llb.State, *Image, *binfotypes.BuildInfo, error)
}

func Dockerfile2LLB(ctx context.Context, dt []byte, opt ConvertOpt) (*llb.State, *Image, *binfotypes.BuildInfo, error) {
	buildInfo := &binfotypes.BuildInfo{}
	contextByName := opt.ContextByName
	opt.ContextByName = func(ctx context.Context, name, resolveMode string, p *ocispecs.Platform) (*llb.State, *Image, *binfotypes.BuildInfo, error) {
		if !strings.EqualFold(name, "scratch") && !strings.EqualFold(name, "context") {
			if contextByName != nil {
				if p == nil {
					p = opt.TargetPlatform
				}
				st, img, bi, err := contextByName(ctx, name, resolveMode, p)
				if err != nil {
					return nil, nil, nil, err
				}
				if bi != nil && bi.Deps != nil {
					for k := range bi.Deps {
						if buildInfo.Deps == nil {
							buildInfo.Deps = make(map[string]binfotypes.BuildInfo)
						}
						buildInfo.Deps[k] = bi.Deps[k]
					}
				}
				return st, img, bi, nil
			}
		}
		return nil, nil, nil, nil
	}

	if len(dt) == 0 {
		return nil, nil, nil, errors.Errorf("the Dockerfile cannot be empty")
	}

	if opt.ContextLocalName == "" {
		opt.ContextLocalName = defaultContextLocalName
	}

	platformOpt := buildPlatformOpt(&opt)

	optMetaArgs := getPlatformArgs(platformOpt)
	for i, arg := range optMetaArgs {
		optMetaArgs[i] = setKVValue(arg, opt.BuildArgs)
	}

	dockerfile, err := parser.Parse(bytes.NewReader(dt))
	if err != nil {
		return nil, nil, nil, err
	}

	for _, w := range dockerfile.Warnings {
		opt.Warn(w.Short, w.URL, w.Detail, w.Location)
	}

	proxyEnv := proxyEnvFromBuildArgs(opt.BuildArgs)

	stages, metaArgs, err := instructions.Parse(dockerfile.AST)
	if err != nil {
		return nil, nil, nil, err
	}

	shlex := shell.NewLex(dockerfile.EscapeToken)

	for _, cmd := range metaArgs {
		for _, metaArg := range cmd.Args {
			if metaArg.Value != nil {
				*metaArg.Value, _ = shlex.ProcessWordWithMap(*metaArg.Value, metaArgsToMap(optMetaArgs))
			}
			optMetaArgs = append(optMetaArgs, setKVValue(metaArg, opt.BuildArgs))
		}
	}

	metaResolver := opt.MetaResolver
	if metaResolver == nil {
		metaResolver = imagemetaresolver.Default()
	}

	allDispatchStates := newDispatchStates()

	// set base state for every image
	for i, st := range stages {
		name, err := shlex.ProcessWordWithMap(st.BaseName, metaArgsToMap(optMetaArgs))
		if err != nil {
			return nil, nil, nil, parser.WithLocation(err, st.Location)
		}
		if name == "" {
			return nil, nil, nil, parser.WithLocation(errors.Errorf("base name (%s) should not be blank", st.BaseName), st.Location)
		}
		st.BaseName = name

		ds := &dispatchState{
			stage:          st,
			deps:           make(map[*dispatchState]struct{}),
			ctxPaths:       make(map[string]struct{}),
			stageName:      st.Name,
			prefixPlatform: opt.PrefixPlatform,
		}

		if v := st.Platform; v != "" {
			v, err := shlex.ProcessWordWithMap(v, metaArgsToMap(optMetaArgs))
			if err != nil {
				return nil, nil, nil, parser.WithLocation(errors.Wrapf(err, "failed to process arguments for platform %s", v), st.Location)
			}

			p, err := platforms.Parse(v)
			if err != nil {
				return nil, nil, nil, parser.WithLocation(errors.Wrapf(err, "failed to parse platform %s", v), st.Location)
			}
			ds.platform = &p
		}

		if st.Name != "" {
			s, img, bi, err := opt.ContextByName(ctx, st.Name, opt.ImageResolveMode.String(), ds.platform)
			if err != nil {
				return nil, nil, nil, err
			}
			if s != nil {
				ds.noinit = true
				ds.state = *s
				if img != nil {
					ds.image = *img
					if img.Architecture != "" && img.OS != "" {
						ds.platform = &ocispecs.Platform{
							OS:           img.OS,
							Architecture: img.Architecture,
							Variant:      img.Variant,
						}
					}
				}
				if bi != nil {
					ds.buildInfo = *bi
				}
				allDispatchStates.addState(ds)
				continue
			}
		}

		if st.Name == "" {
			ds.stageName = fmt.Sprintf("stage-%d", i)
		}

		allDispatchStates.addState(ds)

		total := 0
		if ds.stage.BaseName != emptyImageName && ds.base == nil {
			total = 1
		}
		for _, cmd := range ds.stage.Commands {
			switch cmd.(type) {
			case *instructions.AddCommand, *instructions.CopyCommand, *instructions.RunCommand:
				total++
			case *instructions.WorkdirCommand:
				if useFileOp(opt.BuildArgs, opt.LLBCaps) {
					total++
				}
			}
		}
		ds.cmdTotal = total

		if opt.IgnoreCache != nil {
			if len(opt.IgnoreCache) == 0 {
				ds.ignoreCache = true
			} else if st.Name != "" {
				for _, n := range opt.IgnoreCache {
					if strings.EqualFold(n, st.Name) {
						ds.ignoreCache = true
					}
				}
			}
		}
	}

	var target *dispatchState
	if opt.Target == "" {
		target = allDispatchStates.lastTarget()
	} else {
		var ok bool
		target, ok = allDispatchStates.findStateByName(opt.Target)
		if !ok {
			return nil, nil, nil, errors.Errorf("target stage %s could not be found", opt.Target)
		}
	}

	// fill dependencies to stages so unreachable ones can avoid loading image configs
	for _, d := range allDispatchStates.states {
		d.commands = make([]command, len(d.stage.Commands))
		for i, cmd := range d.stage.Commands {
			newCmd, err := toCommand(cmd, allDispatchStates)
			if err != nil {
				return nil, nil, nil, err
			}
			d.commands[i] = newCmd
			for _, src := range newCmd.sources {
				if src != nil {
					d.deps[src] = struct{}{}
					if src.unregistered {
						allDispatchStates.addState(src)
					}
				}
			}
		}
	}

	if has, state := hasCircularDependency(allDispatchStates.states); has {
		return nil, nil, nil, errors.Errorf("circular dependency detected on stage: %s", state.stageName)
	}

	if len(allDispatchStates.states) == 1 {
		allDispatchStates.states[0].stageName = ""
	}

	allStageNames := make([]string, 0, len(allDispatchStates.states))
	for _, s := range allDispatchStates.states {
		if s.stageName != "" {
			allStageNames = append(allStageNames, s.stageName)
		}
	}

	eg, ctx := errgroup.WithContext(ctx)
	for i, d := range allDispatchStates.states {
		reachable := isReachable(target, d)
		// resolve image config for every stage
		if d.base == nil && !d.noinit {
			if d.stage.BaseName == emptyImageName {
				d.state = llb.Scratch()
				d.image = emptyImage(platformOpt.targetPlatform)
				continue
			}
			func(i int, d *dispatchState) {
				eg.Go(func() (err error) {
					defer func() {
						if err != nil {
							err = parser.WithLocation(err, d.stage.Location)
						}
					}()
					origName := d.stage.BaseName
					ref, err := reference.ParseNormalizedNamed(d.stage.BaseName)
					if err != nil {
						return errors.Wrapf(err, "failed to parse stage name %q", d.stage.BaseName)
					}
					platform := d.platform
					if platform == nil {
						platform = &platformOpt.targetPlatform
					}
					d.stage.BaseName = reference.TagNameOnly(ref).String()

					var isScratch bool
					st, img, bi, err := opt.ContextByName(ctx, d.stage.BaseName, opt.ImageResolveMode.String(), platform)
					if err != nil {
						return err
					}
					if st != nil {
						if img != nil {
							d.image = *img
						} else {
							d.image = emptyImage(platformOpt.targetPlatform)
						}
						if bi != nil {
							d.buildInfo = *bi
						}
						d.state = st.Platform(*platform)
						d.platform = platform
						return nil
					}
					if reachable {
						prefix := "["
						if opt.PrefixPlatform && platform != nil {
							prefix += platforms.Format(*platform) + " "
						}
						prefix += "internal]"
						dgst, dt, err := metaResolver.ResolveImageConfig(ctx, d.stage.BaseName, llb.ResolveImageConfigOpt{
							Platform:    platform,
							ResolveMode: opt.ImageResolveMode.String(),
							LogName:     fmt.Sprintf("%s load metadata for %s", prefix, d.stage.BaseName),
						})
						if err != nil {
							return suggest.WrapError(errors.Wrap(err, origName), origName, append(allStageNames, commonImageNames()...), true)
						}
						var img Image
						if err := json.Unmarshal(dt, &img); err != nil {
							return errors.Wrap(err, "failed to parse image config")
						}
						img.Created = nil
						// if there is no explicit target platform, try to match based on image config
						if d.platform == nil && platformOpt.implicitTarget {
							p := autoDetectPlatform(img, *platform, platformOpt.buildPlatforms)
							platform = &p
						}
						if dgst != "" {
							ref, err = reference.WithDigest(ref, dgst)
							if err != nil {
								return err
							}
						}
						d.stage.BaseName = ref.String()
						if len(img.RootFS.DiffIDs) == 0 {
							isScratch = true
							// schema1 images can't return diffIDs so double check :(
							for _, h := range img.History {
								if !h.EmptyLayer {
									isScratch = false
									break
								}
							}
						}
						if !isScratch {
							// if image not scratch set original image name as ref
							// and actual reference as alias in binfotypes.Source
							d.buildInfo.Sources = append(d.buildInfo.Sources, binfotypes.Source{
								Type:  binfotypes.SourceTypeDockerImage,
								Ref:   origName,
								Alias: ref.String(),
								Pin:   dgst.String(),
							})
						}
						d.image = img
					}
					if isScratch {
						d.state = llb.Scratch()
					} else {
						d.state = llb.Image(d.stage.BaseName,
							dfCmd(d.stage.SourceCode),
							llb.Platform(*platform),
							opt.ImageResolveMode,
							llb.WithCustomName(prefixCommand(d, "FROM "+d.stage.BaseName, opt.PrefixPlatform, platform, nil)),
							location(opt.SourceMap, d.stage.Location),
						)
					}
					d.platform = platform
					return nil
				})
			}(i, d)
		}
	}

	if err := eg.Wait(); err != nil {
		return nil, nil, nil, err
	}

	buildContext := &mutableOutput{}
	ctxPaths := map[string]struct{}{}

	for _, d := range allDispatchStates.states {
		if !isReachable(target, d) || d.noinit {
			continue
		}

		// collect build sources and dependencies
		if len(d.buildInfo.Sources) > 0 {
			buildInfo.Sources = append(buildInfo.Sources, d.buildInfo.Sources...)
		}
		if d.buildInfo.Deps != nil {
			for name, bi := range d.buildInfo.Deps {
				if buildInfo.Deps == nil {
					buildInfo.Deps = make(map[string]binfotypes.BuildInfo)
				}
				buildInfo.Deps[name] = bi
			}
		}

		if d.base != nil {
			d.state = d.base.state
			d.platform = d.base.platform
			d.image = clone(d.base.image)
		}

		// make sure that PATH is always set
		if _, ok := shell.BuildEnvs(d.image.Config.Env)["PATH"]; !ok {
			var os string
			if d.platform != nil {
				os = d.platform.OS
			}
			d.image.Config.Env = append(d.image.Config.Env, "PATH="+system.DefaultPathEnv(os))
		}

		// initialize base metadata from image conf
		for _, env := range d.image.Config.Env {
			k, v := parseKeyValue(env)
			d.state = d.state.AddEnv(k, v)
		}
		if opt.Hostname != "" {
			d.state = d.state.Hostname(opt.Hostname)
		}
		if d.image.Config.WorkingDir != "" {
			if err = dispatchWorkdir(d, &instructions.WorkdirCommand{Path: d.image.Config.WorkingDir}, false, nil); err != nil {
				return nil, nil, nil, parser.WithLocation(err, d.stage.Location)
			}
		}
		if d.image.Config.User != "" {
			if err = dispatchUser(d, &instructions.UserCommand{User: d.image.Config.User}, false); err != nil {
				return nil, nil, nil, parser.WithLocation(err, d.stage.Location)
			}
		}
		d.state = d.state.Network(opt.ForceNetMode)

		opt := dispatchOpt{
			allDispatchStates: allDispatchStates,
			metaArgs:          optMetaArgs,
			buildArgValues:    opt.BuildArgs,
			shlex:             shlex,
			sessionID:         opt.SessionID,
			buildContext:      llb.NewState(buildContext),
			proxyEnv:          proxyEnv,
			cacheIDNamespace:  opt.CacheIDNamespace,
			buildPlatforms:    platformOpt.buildPlatforms,
			targetPlatform:    platformOpt.targetPlatform,
			extraHosts:        opt.ExtraHosts,
			shmSize:           opt.ShmSize,
			ulimit:            opt.Ulimit,
			cgroupParent:      opt.CgroupParent,
			copyImage:         opt.OverrideCopyImage,
			llbCaps:           opt.LLBCaps,
			sourceMap:         opt.SourceMap,
		}
		if opt.copyImage == "" {
			opt.copyImage = DefaultCopyImage
		}

		if err = dispatchOnBuildTriggers(d, d.image.Config.OnBuild, opt); err != nil {
			return nil, nil, nil, parser.WithLocation(err, d.stage.Location)
		}
		d.image.Config.OnBuild = nil

		for _, cmd := range d.commands {
			if err := dispatch(d, cmd, opt); err != nil {
				return nil, nil, nil, parser.WithLocation(err, cmd.Location())
			}
		}

		for p := range d.ctxPaths {
			ctxPaths[p] = struct{}{}
		}
	}

	// sort build sources
	if len(buildInfo.Sources) > 0 {
		sort.Slice(buildInfo.Sources, func(i, j int) bool {
			return buildInfo.Sources[i].Ref < buildInfo.Sources[j].Ref
		})
	}

	if len(opt.Labels) != 0 && target.image.Config.Labels == nil {
		target.image.Config.Labels = make(map[string]string, len(opt.Labels))
	}
	for k, v := range opt.Labels {
		target.image.Config.Labels[k] = v
	}

	opts := []llb.LocalOption{
		llb.SessionID(opt.SessionID),
		llb.ExcludePatterns(opt.Excludes),
		llb.SharedKeyHint(opt.ContextLocalName),
		WithInternalName("load build context"),
	}
	if includePatterns := normalizeContextPaths(ctxPaths); includePatterns != nil {
		opts = append(opts, llb.FollowPaths(includePatterns))
	}

	bc := llb.Local(opt.ContextLocalName, opts...)
	if opt.BuildContext != nil {
		bc = *opt.BuildContext
	}
	buildContext.Output = bc.Output()

	defaults := []llb.ConstraintsOpt{
		llb.Platform(platformOpt.targetPlatform),
	}
	if opt.LLBCaps != nil {
		defaults = append(defaults, llb.WithCaps(*opt.LLBCaps))
	}
	st := target.state.SetMarshalDefaults(defaults...)

	if !platformOpt.implicitTarget {
		target.image.OS = platformOpt.targetPlatform.OS
		target.image.Architecture = platformOpt.targetPlatform.Architecture
		target.image.Variant = platformOpt.targetPlatform.Variant
	}

	return &st, &target.image, buildInfo, nil
}

func metaArgsToMap(metaArgs []instructions.KeyValuePairOptional) map[string]string {
	m := map[string]string{}

	for _, arg := range metaArgs {
		m[arg.Key] = arg.ValueString()
	}

	return m
}

func toCommand(ic instructions.Command, allDispatchStates *dispatchStates) (command, error) {
	cmd := command{Command: ic}
	if c, ok := ic.(*instructions.CopyCommand); ok {
		if c.From != "" {
			var stn *dispatchState
			index, err := strconv.Atoi(c.From)
			if err != nil {
				stn, ok = allDispatchStates.findStateByName(c.From)
				if !ok {
					stn = &dispatchState{
						stage:        instructions.Stage{BaseName: c.From, Location: ic.Location()},
						deps:         make(map[*dispatchState]struct{}),
						unregistered: true,
					}
				}
			} else {
				stn, err = allDispatchStates.findStateByIndex(index)
				if err != nil {
					return command{}, err
				}
			}
			cmd.sources = []*dispatchState{stn}
		}
	}

	if ok := detectRunMount(&cmd, allDispatchStates); ok {
		return cmd, nil
	}

	return cmd, nil
}

type dispatchOpt struct {
	allDispatchStates *dispatchStates
	metaArgs          []instructions.KeyValuePairOptional
	buildArgValues    map[string]string
	shlex             *shell.Lex
	sessionID         string
	buildContext      llb.State
	proxyEnv          *llb.ProxyEnv
	cacheIDNamespace  string
	targetPlatform    ocispecs.Platform
	buildPlatforms    []ocispecs.Platform
	extraHosts        []llb.HostIP
	shmSize           int64
	ulimit            []pb.Ulimit
	cgroupParent      string
	copyImage         string
	llbCaps           *apicaps.CapSet
	sourceMap         *llb.SourceMap
}

func dispatch(d *dispatchState, cmd command, opt dispatchOpt) error {
	if ex, ok := cmd.Command.(instructions.SupportsSingleWordExpansion); ok {
		err := ex.Expand(func(word string) (string, error) {
			env, err := d.state.Env(context.TODO())
			if err != nil {
				return "", err
			}
			return opt.shlex.ProcessWord(word, env)
		})
		if err != nil {
			return err
		}
	}
	if ex, ok := cmd.Command.(instructions.SupportsSingleWordExpansionRaw); ok {
		err := ex.ExpandRaw(func(word string) (string, error) {
			env, err := d.state.Env(context.TODO())
			if err != nil {
				return "", err
			}

			lex := shell.NewLex('\\')
			lex.SkipProcessQuotes = true
			return lex.ProcessWord(word, env)
		})
		if err != nil {
			return err
		}
	}

	var err error
	switch c := cmd.Command.(type) {
	case *instructions.MaintainerCommand:
		err = dispatchMaintainer(d, c)
	case *instructions.EnvCommand:
		err = dispatchEnv(d, c)
	case *instructions.RunCommand:
		err = dispatchRun(d, c, opt.proxyEnv, cmd.sources, opt)
	case *instructions.WorkdirCommand:
		err = dispatchWorkdir(d, c, true, &opt)
	case *instructions.AddCommand:
		err = dispatchCopy(d, copyConfig{
			params:       c.SourcesAndDest,
			source:       opt.buildContext,
			isAddCommand: true,
			cmdToPrint:   c,
			chown:        c.Chown,
			chmod:        c.Chmod,
			link:         c.Link,
			location:     c.Location(),
			opt:          opt,
		})
		if err == nil {
			for _, src := range c.SourcePaths {
				if !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://") {
					d.ctxPaths[path.Join("/", filepath.ToSlash(src))] = struct{}{}
				}
			}
		}
	case *instructions.LabelCommand:
		err = dispatchLabel(d, c)
	case *instructions.OnbuildCommand:
		err = dispatchOnbuild(d, c)
	case *instructions.CmdCommand:
		err = dispatchCmd(d, c)
	case *instructions.EntrypointCommand:
		err = dispatchEntrypoint(d, c)
	case *instructions.HealthCheckCommand:
		err = dispatchHealthcheck(d, c)
	case *instructions.ExposeCommand:
		err = dispatchExpose(d, c, opt.shlex)
	case *instructions.UserCommand:
		err = dispatchUser(d, c, true)
	case *instructions.VolumeCommand:
		err = dispatchVolume(d, c)
	case *instructions.StopSignalCommand:
		err = dispatchStopSignal(d, c)
	case *instructions.ShellCommand:
		err = dispatchShell(d, c)
	case *instructions.ArgCommand:
		err = dispatchArg(d, c, opt.metaArgs, opt.buildArgValues)
	case *instructions.CopyCommand:
		l := opt.buildContext
		if len(cmd.sources) != 0 {
			l = cmd.sources[0].state
		}
		err = dispatchCopy(d, copyConfig{
			params:       c.SourcesAndDest,
			source:       l,
			isAddCommand: false,
			cmdToPrint:   c,
			chown:        c.Chown,
			chmod:        c.Chmod,
			link:         c.Link,
			location:     c.Location(),
			opt:          opt,
		})
		if err == nil && len(cmd.sources) == 0 {
			for _, src := range c.SourcePaths {
				d.ctxPaths[path.Join("/", filepath.ToSlash(src))] = struct{}{}
			}
		}
	default:
	}
	return err
}

type dispatchState struct {
	state          llb.State
	image          Image
	platform       *ocispecs.Platform
	stage          instructions.Stage
	base           *dispatchState
	noinit         bool
	deps           map[*dispatchState]struct{}
	buildArgs      []instructions.KeyValuePairOptional
	commands       []command
	ctxPaths       map[string]struct{}
	ignoreCache    bool
	cmdSet         bool
	unregistered   bool
	stageName      string
	cmdIndex       int
	cmdTotal       int
	prefixPlatform bool
	buildInfo      binfotypes.BuildInfo
}

type dispatchStates struct {
	states       []*dispatchState
	statesByName map[string]*dispatchState
}

func newDispatchStates() *dispatchStates {
	return &dispatchStates{statesByName: map[string]*dispatchState{}}
}

func (dss *dispatchStates) addState(ds *dispatchState) {
	dss.states = append(dss.states, ds)

	if d, ok := dss.statesByName[ds.stage.BaseName]; ok {
		ds.base = d
	}
	if ds.stage.Name != "" {
		dss.statesByName[strings.ToLower(ds.stage.Name)] = ds
	}
}

func (dss *dispatchStates) findStateByName(name string) (*dispatchState, bool) {
	ds, ok := dss.statesByName[strings.ToLower(name)]
	return ds, ok
}

func (dss *dispatchStates) findStateByIndex(index int) (*dispatchState, error) {
	if index < 0 || index >= len(dss.states) {
		return nil, errors.Errorf("invalid stage index %d", index)
	}

	return dss.states[index], nil
}

func (dss *dispatchStates) lastTarget() *dispatchState {
	return dss.states[len(dss.states)-1]
}

type command struct {
	instructions.Command
	sources []*dispatchState
}

func dispatchOnBuildTriggers(d *dispatchState, triggers []string, opt dispatchOpt) error {
	for _, trigger := range triggers {
		ast, err := parser.Parse(strings.NewReader(trigger))
		if err != nil {
			return err
		}
		if len(ast.AST.Children) != 1 {
			return errors.New("onbuild trigger should be a single expression")
		}
		ic, err := instructions.ParseCommand(ast.AST.Children[0])
		if err != nil {
			return err
		}
		cmd, err := toCommand(ic, opt.allDispatchStates)
		if err != nil {
			return err
		}
		if err := dispatch(d, cmd, opt); err != nil {
			return err
		}
	}
	return nil
}

func dispatchEnv(d *dispatchState, c *instructions.EnvCommand) error {
	commitMessage := bytes.NewBufferString("ENV")
	for _, e := range c.Env {
		commitMessage.WriteString(" " + e.String())
		d.state = d.state.AddEnv(e.Key, e.Value)
		d.image.Config.Env = addEnv(d.image.Config.Env, e.Key, e.Value)
	}
	return commitToHistory(&d.image, commitMessage.String(), false, nil)
}

func dispatchRun(d *dispatchState, c *instructions.RunCommand, proxy *llb.ProxyEnv, sources []*dispatchState, dopt dispatchOpt) error {
	var opt []llb.RunOption

	customname := c.String()

	var args []string = c.CmdLine
	if len(c.Files) > 0 {
		if len(args) != 1 || !c.PrependShell {
			return fmt.Errorf("parsing produced an invalid run command: %v", args)
		}

		if heredoc := parser.MustParseHeredoc(args[0]); heredoc != nil {
			if d.image.OS != "windows" && strings.HasPrefix(c.Files[0].Data, "#!") {
				// This is a single heredoc with a shebang, so create a file
				// and run it.
				// NOTE: choosing to expand doesn't really make sense here, so
				// we silently ignore that option if it was provided.
				sourcePath := "/"
				destPath := "/dev/pipes/"

				f := c.Files[0].Name
				data := c.Files[0].Data
				if c.Files[0].Chomp {
					data = parser.ChompHeredocContent(data)
				}
				st := llb.Scratch().Dir(sourcePath).File(
					llb.Mkfile(f, 0755, []byte(data)),
					WithInternalName("preparing inline document"),
				)

				mount := llb.AddMount(destPath, st, llb.SourcePath(sourcePath), llb.Readonly)
				opt = append(opt, mount)

				args = []string{path.Join(destPath, f)}
			} else {
				// Just a simple heredoc, so just run the contents in the
				// shell: this creates the effect of a "fake"-heredoc, so that
				// the syntax can still be used for shells that don't support
				// heredocs directly.
				// NOTE: like above, we ignore the expand option.
				data := c.Files[0].Data
				if c.Files[0].Chomp {
					data = parser.ChompHeredocContent(data)
				}
				args = []string{data}
			}
			customname += fmt.Sprintf(" (%s)", summarizeHeredoc(c.Files[0].Data))
		} else {
			// More complex heredoc, so reconstitute it, and pass it to the
			// shell to handle.
			full := args[0]
			for _, file := range c.Files {
				full += "\n" + file.Data + file.Name
			}
			args = []string{full}
		}
	}
	if c.PrependShell {
		args = withShell(d.image, args)
	}

	env, err := d.state.Env(context.TODO())
	if err != nil {
		return err
	}
	opt = append(opt, llb.Args(args), dfCmd(c), location(dopt.sourceMap, c.Location()))
	if d.ignoreCache {
		opt = append(opt, llb.IgnoreCache)
	}
	if proxy != nil {
		opt = append(opt, llb.WithProxy(*proxy))
	}

	runMounts, err := dispatchRunMounts(d, c, sources, dopt)
	if err != nil {
		return err
	}
	opt = append(opt, runMounts...)

	securityOpt, err := dispatchRunSecurity(c)
	if err != nil {
		return err
	}
	if securityOpt != nil {
		opt = append(opt, securityOpt)
	}

	networkOpt, err := dispatchRunNetwork(c)
	if err != nil {
		return err
	}
	if networkOpt != nil {
		opt = append(opt, networkOpt)
	}

	if dopt.llbCaps != nil && dopt.llbCaps.Supports(pb.CapExecMetaUlimit) == nil {
		for _, u := range dopt.ulimit {
			opt = append(opt, llb.AddUlimit(llb.UlimitName(u.Name), u.Soft, u.Hard))
		}
	}

	shlex := *dopt.shlex
	shlex.RawQuotes = true
	shlex.SkipUnsetEnv = true

	pl, err := d.state.GetPlatform(context.TODO())
	if err != nil {
		return err
	}
	opt = append(opt, llb.WithCustomName(prefixCommand(d, uppercaseCmd(processCmdEnv(&shlex, customname, env)), d.prefixPlatform, pl, env)))
	for _, h := range dopt.extraHosts {
		opt = append(opt, llb.AddExtraHost(h.Host, h.IP))
	}

	if dopt.llbCaps != nil && dopt.llbCaps.Supports(pb.CapExecMountTmpfsSize) == nil {
		if dopt.shmSize > 0 {
			opt = append(opt, llb.AddMount("/dev/shm", llb.Scratch(), llb.Tmpfs(llb.TmpfsSize(dopt.shmSize))))
		}
	}

	if dopt.llbCaps != nil && dopt.llbCaps.Supports(pb.CapExecMetaCgroupParent) == nil {
		if len(dopt.cgroupParent) > 0 {
			opt = append(opt, llb.WithCgroupParent(dopt.cgroupParent))
		}
	}

	d.state = d.state.Run(opt...).Root()
	return commitToHistory(&d.image, "RUN "+runCommandString(args, d.buildArgs, shell.BuildEnvs(env)), true, &d.state)
}

func dispatchWorkdir(d *dispatchState, c *instructions.WorkdirCommand, commit bool, opt *dispatchOpt) error {
	d.state = d.state.Dir(c.Path)
	wd := c.Path
	if !path.IsAbs(c.Path) {
		wd = path.Join("/", d.image.Config.WorkingDir, wd)
	}
	d.image.Config.WorkingDir = wd
	if commit {
		withLayer := false
		if wd != "/" && opt != nil && useFileOp(opt.buildArgValues, opt.llbCaps) {
			mkdirOpt := []llb.MkdirOption{llb.WithParents(true)}
			if user := d.image.Config.User; user != "" {
				mkdirOpt = append(mkdirOpt, llb.WithUser(user))
			}
			platform := opt.targetPlatform
			if d.platform != nil {
				platform = *d.platform
			}
			env, err := d.state.Env(context.TODO())
			if err != nil {
				return err
			}
			d.state = d.state.File(llb.Mkdir(wd, 0755, mkdirOpt...),
				llb.WithCustomName(prefixCommand(d, uppercaseCmd(processCmdEnv(opt.shlex, c.String(), env)), d.prefixPlatform, &platform, env)),
				location(opt.sourceMap, c.Location()),
			)
			withLayer = true
		}
		return commitToHistory(&d.image, "WORKDIR "+wd, withLayer, nil)
	}
	return nil
}

func dispatchCopyFileOp(d *dispatchState, cfg copyConfig) error {
	pp, err := pathRelativeToWorkingDir(d.state, cfg.params.DestPath)
	if err != nil {
		return err
	}
	dest := path.Join("/", pp)
	if cfg.params.DestPath == "." || cfg.params.DestPath == "" || cfg.params.DestPath[len(cfg.params.DestPath)-1] == filepath.Separator {
		dest += string(filepath.Separator)
	}

	var copyOpt []llb.CopyOption

	if cfg.chown != "" {
		copyOpt = append(copyOpt, llb.WithUser(cfg.chown))
	}

	var mode *os.FileMode
	if cfg.chmod != "" {
		p, err := strconv.ParseUint(cfg.chmod, 8, 32)
		if err == nil {
			perm := os.FileMode(p)
			mode = &perm
		}
	}

	commitMessage := bytes.NewBufferString("")
	if cfg.isAddCommand {
		commitMessage.WriteString("ADD")
	} else {
		commitMessage.WriteString("COPY")
	}

	var a *llb.FileAction

	for _, src := range cfg.params.SourcePaths {
		commitMessage.WriteString(" " + src)
		if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
			if !cfg.isAddCommand {
				return errors.New("source can't be a URL for COPY")
			}

			// Resources from remote URLs are not decompressed.
			// https://docs.docker.com/engine/reference/builder/#add
			//
			// Note: mixing up remote archives and local archives in a single ADD instruction
			// would result in undefined behavior: https://github.com/moby/buildkit/pull/387#discussion_r189494717
			u, err := url.Parse(src)
			f := "__unnamed__"
			if err == nil {
				if base := path.Base(u.Path); base != "." && base != "/" {
					f = base
				}
			}

			st := llb.HTTP(src, llb.Filename(f), dfCmd(cfg.params))

			opts := append([]llb.CopyOption{&llb.CopyInfo{
				Mode:           mode,
				CreateDestPath: true,
			}}, copyOpt...)

			if a == nil {
				a = llb.Copy(st, f, dest, opts...)
			} else {
				a = a.Copy(st, f, dest, opts...)
			}
		} else {
			opts := append([]llb.CopyOption{&llb.CopyInfo{
				Mode:                mode,
				FollowSymlinks:      true,
				CopyDirContentsOnly: true,
				AttemptUnpack:       cfg.isAddCommand,
				CreateDestPath:      true,
				AllowWildcard:       true,
				AllowEmptyWildcard:  true,
			}}, copyOpt...)

			if a == nil {
				a = llb.Copy(cfg.source, filepath.Join("/", src), dest, opts...)
			} else {
				a = a.Copy(cfg.source, filepath.Join("/", src), dest, opts...)
			}
		}
	}

	for _, src := range cfg.params.SourceContents {
		commitMessage.WriteString(" <<" + src.Path)

		data := src.Data
		f := src.Path
		st := llb.Scratch().File(
			llb.Mkfile(f, 0664, []byte(data)),
			WithInternalName("preparing inline document"),
		)

		opts := append([]llb.CopyOption{&llb.CopyInfo{
			Mode:           mode,
			CreateDestPath: true,
		}}, copyOpt...)

		if a == nil {
			a = llb.Copy(st, f, dest, opts...)
		} else {
			a = a.Copy(st, f, dest, opts...)
		}
	}

	commitMessage.WriteString(" " + cfg.params.DestPath)

	platform := cfg.opt.targetPlatform
	if d.platform != nil {
		platform = *d.platform
	}

	env, err := d.state.Env(context.TODO())
	if err != nil {
		return err
	}

	name := uppercaseCmd(processCmdEnv(cfg.opt.shlex, cfg.cmdToPrint.String(), env))
	fileOpt := []llb.ConstraintsOpt{
		llb.WithCustomName(prefixCommand(d, name, d.prefixPlatform, &platform, env)),
		location(cfg.opt.sourceMap, cfg.location),
	}
	if d.ignoreCache {
		fileOpt = append(fileOpt, llb.IgnoreCache)
	}

	if cfg.opt.llbCaps.Supports(pb.CapMergeOp) == nil && cfg.link && cfg.chmod == "" {
		pgID := identity.NewID()
		d.cmdIndex-- // prefixCommand increases it
		pgName := prefixCommand(d, name, d.prefixPlatform, &platform, env)

		var copyOpts []llb.ConstraintsOpt
		copy(copyOpts, fileOpt)
		copyOpts = append(copyOpts, llb.ProgressGroup(pgID, pgName, true))

		var mergeOpts []llb.ConstraintsOpt
		copy(mergeOpts, fileOpt)
		d.cmdIndex--
		mergeOpts = append(mergeOpts, llb.ProgressGroup(pgID, pgName, false), llb.WithCustomName(prefixCommand(d, "LINK "+name, d.prefixPlatform, &platform, env)))

		d.state = d.state.WithOutput(llb.Merge([]llb.State{d.state, llb.Scratch().File(a, copyOpts...)}, mergeOpts...).Output())
	} else {
		d.state = d.state.File(a, fileOpt...)
	}

	return commitToHistory(&d.image, commitMessage.String(), true, &d.state)
}

type copyConfig struct {
	params       instructions.SourcesAndDest
	source       llb.State
	isAddCommand bool
	cmdToPrint   fmt.Stringer
	chown        string
	chmod        string
	link         bool
	location     []parser.Range
	opt          dispatchOpt
}

func dispatchCopy(d *dispatchState, cfg copyConfig) error {
	if useFileOp(cfg.opt.buildArgValues, cfg.opt.llbCaps) {
		return dispatchCopyFileOp(d, cfg)
	}

	if len(cfg.params.SourceContents) > 0 {
		return errors.New("inline content copy is not supported")
	}

	if cfg.chmod != "" {
		if cfg.opt.llbCaps != nil && cfg.opt.llbCaps.Supports(pb.CapFileBase) != nil {
			return errors.Wrap(cfg.opt.llbCaps.Supports(pb.CapFileBase), "chmod is not supported")
		}
		return errors.New("chmod is not supported")
	}

	img := llb.Image(cfg.opt.copyImage, llb.MarkImageInternal, llb.Platform(cfg.opt.buildPlatforms[0]), WithInternalName("helper image for file operations"))
	pp, err := pathRelativeToWorkingDir(d.state, cfg.params.DestPath)
	if err != nil {
		return err
	}
	dest := path.Join(".", pp)
	if cfg.params.DestPath == "." || cfg.params.DestPath == "" || cfg.params.DestPath[len(cfg.params.DestPath)-1] == filepath.Separator {
		dest += string(filepath.Separator)
	}
	args := []string{"copy"}
	unpack := cfg.isAddCommand

	mounts := make([]llb.RunOption, 0, len(cfg.params.SourcePaths))
	if cfg.chown != "" {
		args = append(args, fmt.Sprintf("--chown=%s", cfg.chown))
		_, _, err := parseUser(cfg.chown)
		if err != nil {
			mounts = append(mounts, llb.AddMount("/etc/passwd", d.state, llb.SourcePath("/etc/passwd"), llb.Readonly))
			mounts = append(mounts, llb.AddMount("/etc/group", d.state, llb.SourcePath("/etc/group"), llb.Readonly))
		}
	}

	commitMessage := bytes.NewBufferString("")
	if cfg.isAddCommand {
		commitMessage.WriteString("ADD")
	} else {
		commitMessage.WriteString("COPY")
	}

	for i, src := range cfg.params.SourcePaths {
		commitMessage.WriteString(" " + src)
		if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
			if !cfg.isAddCommand {
				return errors.New("source can't be a URL for COPY")
			}

			// Resources from remote URLs are not decompressed.
			// https://docs.docker.com/engine/reference/builder/#add
			//
			// Note: mixing up remote archives and local archives in a single ADD instruction
			// would result in undefined behavior: https://github.com/moby/buildkit/pull/387#discussion_r189494717
			unpack = false
			u, err := url.Parse(src)
			f := "__unnamed__"
			if err == nil {
				if base := path.Base(u.Path); base != "." && base != "/" {
					f = base
				}
			}
			target := path.Join(fmt.Sprintf("/src-%d", i), f)
			args = append(args, target)
			mounts = append(mounts, llb.AddMount(path.Dir(target), llb.HTTP(src, llb.Filename(f), dfCmd(cfg.params)), llb.Readonly))
		} else {
			d, f := splitWildcards(src)
			targetCmd := fmt.Sprintf("/src-%d", i)
			targetMount := targetCmd
			if f == "" {
				f = path.Base(src)
				targetMount = path.Join(targetMount, f)
			}
			targetCmd = path.Join(targetCmd, f)
			args = append(args, targetCmd)
			mounts = append(mounts, llb.AddMount(targetMount, cfg.source, llb.SourcePath(d), llb.Readonly))
		}
	}

	commitMessage.WriteString(" " + cfg.params.DestPath)

	args = append(args, dest)
	if unpack {
		args = append(args[:1], append([]string{"--unpack"}, args[1:]...)...)
	}

	platform := cfg.opt.targetPlatform
	if d.platform != nil {
		platform = *d.platform
	}

	env, err := d.state.Env(context.TODO())
	if err != nil {
		return err
	}

	runOpt := []llb.RunOption{
		llb.Args(args),
		llb.Dir("/dest"),
		llb.ReadonlyRootFS(),
		dfCmd(cfg.cmdToPrint),
		llb.WithCustomName(prefixCommand(d, uppercaseCmd(processCmdEnv(cfg.opt.shlex, cfg.cmdToPrint.String(), env)), d.prefixPlatform, &platform, env)),
		location(cfg.opt.sourceMap, cfg.location),
	}
	if d.ignoreCache {
		runOpt = append(runOpt, llb.IgnoreCache)
	}

	if cfg.opt.llbCaps != nil {
		if err := cfg.opt.llbCaps.Supports(pb.CapExecMetaNetwork); err == nil {
			runOpt = append(runOpt, llb.Network(llb.NetModeNone))
		}
	}

	run := img.Run(append(runOpt, mounts...)...)
	d.state = run.AddMount("/dest", d.state).Platform(platform)

	return commitToHistory(&d.image, commitMessage.String(), true, &d.state)
}

func dispatchMaintainer(d *dispatchState, c *instructions.MaintainerCommand) error {
	d.image.Author = c.Maintainer
	return commitToHistory(&d.image, fmt.Sprintf("MAINTAINER %v", c.Maintainer), false, nil)
}

func dispatchLabel(d *dispatchState, c *instructions.LabelCommand) error {
	commitMessage := bytes.NewBufferString("LABEL")
	if d.image.Config.Labels == nil {
		d.image.Config.Labels = make(map[string]string, len(c.Labels))
	}
	for _, v := range c.Labels {
		d.image.Config.Labels[v.Key] = v.Value
		commitMessage.WriteString(" " + v.String())
	}
	return commitToHistory(&d.image, commitMessage.String(), false, nil)
}

func dispatchOnbuild(d *dispatchState, c *instructions.OnbuildCommand) error {
	d.image.Config.OnBuild = append(d.image.Config.OnBuild, c.Expression)
	return nil
}

func dispatchCmd(d *dispatchState, c *instructions.CmdCommand) error {
	var args []string = c.CmdLine
	if c.PrependShell {
		args = withShell(d.image, args)
	}
	d.image.Config.Cmd = args
	d.image.Config.ArgsEscaped = true
	d.cmdSet = true
	return commitToHistory(&d.image, fmt.Sprintf("CMD %q", args), false, nil)
}

func dispatchEntrypoint(d *dispatchState, c *instructions.EntrypointCommand) error {
	var args []string = c.CmdLine
	if c.PrependShell {
		args = withShell(d.image, args)
	}
	d.image.Config.Entrypoint = args
	if !d.cmdSet {
		d.image.Config.Cmd = nil
	}
	return commitToHistory(&d.image, fmt.Sprintf("ENTRYPOINT %q", args), false, nil)
}

func dispatchHealthcheck(d *dispatchState, c *instructions.HealthCheckCommand) error {
	d.image.Config.Healthcheck = &HealthConfig{
		Test:        c.Health.Test,
		Interval:    c.Health.Interval,
		Timeout:     c.Health.Timeout,
		StartPeriod: c.Health.StartPeriod,
		Retries:     c.Health.Retries,
	}
	return commitToHistory(&d.image, fmt.Sprintf("HEALTHCHECK %q", d.image.Config.Healthcheck), false, nil)
}

func dispatchExpose(d *dispatchState, c *instructions.ExposeCommand, shlex *shell.Lex) error {
	ports := []string{}
	env, err := d.state.Env(context.TODO())
	if err != nil {
		return err
	}
	for _, p := range c.Ports {
		ps, err := shlex.ProcessWords(p, env)
		if err != nil {
			return err
		}
		ports = append(ports, ps...)
	}
	c.Ports = ports

	ps, _, err := nat.ParsePortSpecs(c.Ports)
	if err != nil {
		return err
	}

	if d.image.Config.ExposedPorts == nil {
		d.image.Config.ExposedPorts = make(map[string]struct{})
	}
	for p := range ps {
		d.image.Config.ExposedPorts[string(p)] = struct{}{}
	}

	return commitToHistory(&d.image, fmt.Sprintf("EXPOSE %v", ps), false, nil)
}

func dispatchUser(d *dispatchState, c *instructions.UserCommand, commit bool) error {
	d.state = d.state.User(c.User)
	d.image.Config.User = c.User
	if commit {
		return commitToHistory(&d.image, fmt.Sprintf("USER %v", c.User), false, nil)
	}
	return nil
}

func dispatchVolume(d *dispatchState, c *instructions.VolumeCommand) error {
	if d.image.Config.Volumes == nil {
		d.image.Config.Volumes = map[string]struct{}{}
	}
	for _, v := range c.Volumes {
		if v == "" {
			return errors.New("VOLUME specified can not be an empty string")
		}
		d.image.Config.Volumes[v] = struct{}{}
	}
	return commitToHistory(&d.image, fmt.Sprintf("VOLUME %v", c.Volumes), false, nil)
}

func dispatchStopSignal(d *dispatchState, c *instructions.StopSignalCommand) error {
	if _, err := signal.ParseSignal(c.Signal); err != nil {
		return err
	}
	d.image.Config.StopSignal = c.Signal
	return commitToHistory(&d.image, fmt.Sprintf("STOPSIGNAL %v", c.Signal), false, nil)
}

func dispatchShell(d *dispatchState, c *instructions.ShellCommand) error {
	d.image.Config.Shell = c.Shell
	return commitToHistory(&d.image, fmt.Sprintf("SHELL %v", c.Shell), false, nil)
}

func dispatchArg(d *dispatchState, c *instructions.ArgCommand, metaArgs []instructions.KeyValuePairOptional, buildArgValues map[string]string) error {
	commitStrs := make([]string, 0, len(c.Args))
	for _, arg := range c.Args {
		buildArg := setKVValue(arg, buildArgValues)

		commitStr := arg.Key
		if arg.Value != nil {
			commitStr += "=" + *arg.Value
		}
		commitStrs = append(commitStrs, commitStr)
		if buildArg.Value == nil {
			for _, ma := range metaArgs {
				if ma.Key == buildArg.Key {
					buildArg.Value = ma.Value
				}
			}
		}

		if buildArg.Value != nil {
			d.state = d.state.AddEnv(buildArg.Key, *buildArg.Value)
		}

		d.buildArgs = append(d.buildArgs, buildArg)
	}
	return commitToHistory(&d.image, "ARG "+strings.Join(commitStrs, " "), false, nil)
}

func pathRelativeToWorkingDir(s llb.State, p string) (string, error) {
	if path.IsAbs(p) {
		return p, nil
	}
	dir, err := s.GetDir(context.TODO())
	if err != nil {
		return "", err
	}
	return path.Join(dir, p), nil
}

func splitWildcards(name string) (string, string) {
	i := 0
	for ; i < len(name); i++ {
		ch := name[i]
		if ch == '\\' {
			i++
		} else if ch == '*' || ch == '?' || ch == '[' {
			break
		}
	}
	if i == len(name) {
		return name, ""
	}

	base := path.Base(name[:i])
	if name[:i] == "" || strings.HasSuffix(name[:i], string(filepath.Separator)) {
		base = ""
	}
	return path.Dir(name[:i]), base + name[i:]
}

func addEnv(env []string, k, v string) []string {
	gotOne := false
	for i, envVar := range env {
		key, _ := parseKeyValue(envVar)
		if shell.EqualEnvKeys(key, k) {
			env[i] = k + "=" + v
			gotOne = true
			break
		}
	}
	if !gotOne {
		env = append(env, k+"="+v)
	}
	return env
}

func parseKeyValue(env string) (string, string) {
	parts := strings.SplitN(env, "=", 2)
	v := ""
	if len(parts) > 1 {
		v = parts[1]
	}

	return parts[0], v
}

func setKVValue(kvpo instructions.KeyValuePairOptional, values map[string]string) instructions.KeyValuePairOptional {
	if v, ok := values[kvpo.Key]; ok {
		kvpo.Value = &v
	}
	return kvpo
}

func dfCmd(cmd interface{}) llb.ConstraintsOpt {
	// TODO: add fmt.Stringer to instructions.Command to remove interface{}
	var cmdStr string
	if cmd, ok := cmd.(fmt.Stringer); ok {
		cmdStr = cmd.String()
	}
	if cmd, ok := cmd.(string); ok {
		cmdStr = cmd
	}
	return llb.WithDescription(map[string]string{
		"com.docker.dockerfile.v1.command": cmdStr,
	})
}

func runCommandString(args []string, buildArgs []instructions.KeyValuePairOptional, envMap map[string]string) string {
	var tmpBuildEnv []string
	for _, arg := range buildArgs {
		v, ok := envMap[arg.Key]
		if !ok {
			v = arg.ValueString()
		}
		tmpBuildEnv = append(tmpBuildEnv, arg.Key+"="+v)
	}
	if len(tmpBuildEnv) > 0 {
		tmpBuildEnv = append([]string{fmt.Sprintf("|%d", len(tmpBuildEnv))}, tmpBuildEnv...)
	}

	return strings.Join(append(tmpBuildEnv, args...), " ")
}

func commitToHistory(img *Image, msg string, withLayer bool, st *llb.State) error {
	if st != nil {
		msg += " # buildkit"
	}

	img.History = append(img.History, ocispecs.History{
		CreatedBy:  msg,
		Comment:    historyComment,
		EmptyLayer: !withLayer,
	})
	return nil
}

func isReachable(from, to *dispatchState) (ret bool) {
	if from == nil {
		return false
	}
	if from == to || isReachable(from.base, to) {
		return true
	}
	for d := range from.deps {
		if isReachable(d, to) {
			return true
		}
	}
	return false
}

func hasCircularDependency(states []*dispatchState) (bool, *dispatchState) {
	var visit func(state *dispatchState) bool
	if states == nil {
		return false, nil
	}
	visited := make(map[*dispatchState]struct{})
	path := make(map[*dispatchState]struct{})

	visit = func(state *dispatchState) bool {
		_, ok := visited[state]
		if ok {
			return false
		}
		visited[state] = struct{}{}
		path[state] = struct{}{}
		for dep := range state.deps {
			_, ok = path[dep]
			if ok {
				return true
			}
			if visit(dep) {
				return true
			}
		}
		delete(path, state)
		return false
	}
	for _, state := range states {
		if visit(state) {
			return true, state
		}
	}
	return false, nil
}

func parseUser(str string) (uid uint32, gid uint32, err error) {
	if str == "" {
		return 0, 0, nil
	}
	parts := strings.SplitN(str, ":", 2)
	for i, v := range parts {
		switch i {
		case 0:
			uid, err = parseUID(v)
			if err != nil {
				return 0, 0, err
			}
			if len(parts) == 1 {
				gid = uid
			}
		case 1:
			gid, err = parseUID(v)
			if err != nil {
				return 0, 0, err
			}
		}
	}
	return
}

func parseUID(str string) (uint32, error) {
	if str == "root" {
		return 0, nil
	}
	uid, err := strconv.ParseUint(str, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(uid), nil
}

func normalizeContextPaths(paths map[string]struct{}) []string {
	pathSlice := make([]string, 0, len(paths))
	for p := range paths {
		if p == "/" {
			return nil
		}
		pathSlice = append(pathSlice, path.Join(".", p))
	}

	sort.Slice(pathSlice, func(i, j int) bool {
		return pathSlice[i] < pathSlice[j]
	})
	return pathSlice
}

func proxyEnvFromBuildArgs(args map[string]string) *llb.ProxyEnv {
	pe := &llb.ProxyEnv{}
	isNil := true
	for k, v := range args {
		if strings.EqualFold(k, "http_proxy") {
			pe.HTTPProxy = v
			isNil = false
		}
		if strings.EqualFold(k, "https_proxy") {
			pe.HTTPSProxy = v
			isNil = false
		}
		if strings.EqualFold(k, "ftp_proxy") {
			pe.FTPProxy = v
			isNil = false
		}
		if strings.EqualFold(k, "no_proxy") {
			pe.NoProxy = v
			isNil = false
		}
		if strings.EqualFold(k, "all_proxy") {
			pe.AllProxy = v
			isNil = false
		}
	}
	if isNil {
		return nil
	}
	return pe
}

type mutableOutput struct {
	llb.Output
}

func withShell(img Image, args []string) []string {
	var shell []string
	if len(img.Config.Shell) > 0 {
		shell = append([]string{}, img.Config.Shell...)
	} else {
		shell = defaultShell(img.OS)
	}
	return append(shell, strings.Join(args, " "))
}

func autoDetectPlatform(img Image, target ocispecs.Platform, supported []ocispecs.Platform) ocispecs.Platform {
	os := img.OS
	arch := img.Architecture
	if target.OS == os && target.Architecture == arch {
		return target
	}
	for _, p := range supported {
		if p.OS == os && p.Architecture == arch {
			return p
		}
	}
	return target
}

func WithInternalName(name string) llb.ConstraintsOpt {
	return llb.WithCustomName("[internal] " + name)
}

func uppercaseCmd(str string) string {
	p := strings.SplitN(str, " ", 2)
	p[0] = strings.ToUpper(p[0])
	return strings.Join(p, " ")
}

func processCmdEnv(shlex *shell.Lex, cmd string, env []string) string {
	w, err := shlex.ProcessWord(cmd, env)
	if err != nil {
		return cmd
	}
	return w
}

func prefixCommand(ds *dispatchState, str string, prefixPlatform bool, platform *ocispecs.Platform, env []string) string {
	if ds.cmdTotal == 0 {
		return str
	}
	out := "["
	if prefixPlatform && platform != nil {
		out += platforms.Format(*platform) + formatTargetPlatform(*platform, platformFromEnv(env)) + " "
	}
	if ds.stageName != "" {
		out += ds.stageName + " "
	}
	ds.cmdIndex++
	out += fmt.Sprintf("%*d/%d] ", int(1+math.Log10(float64(ds.cmdTotal))), ds.cmdIndex, ds.cmdTotal)
	return out + str
}

// formatTargetPlatform formats a secondary platform string for cross compilation cases
func formatTargetPlatform(base ocispecs.Platform, target *ocispecs.Platform) string {
	if target == nil {
		return ""
	}
	if target.OS == "" {
		target.OS = base.OS
	}
	if target.Architecture == "" {
		target.Architecture = base.Architecture
	}
	p := platforms.Normalize(*target)

	if p.OS == base.OS && p.Architecture != base.Architecture {
		archVariant := p.Architecture
		if p.Variant != "" {
			archVariant += "/" + p.Variant
		}
		return "->" + archVariant
	}
	if p.OS != base.OS {
		return "->" + platforms.Format(p)
	}
	return ""
}

// platformFromEnv returns defined platforms based on TARGET* environment variables
func platformFromEnv(env []string) *ocispecs.Platform {
	var p ocispecs.Platform
	var set bool
	for _, v := range env {
		parts := strings.SplitN(v, "=", 2)
		switch parts[0] {
		case "TARGETPLATFORM":
			p, err := platforms.Parse(parts[1])
			if err != nil {
				continue
			}
			return &p
		case "TARGETOS":
			p.OS = parts[1]
			set = true
		case "TARGETARCH":
			p.Architecture = parts[1]
			set = true
		case "TARGETVARIANT":
			p.Variant = parts[1]
			set = true
		}
	}
	if !set {
		return nil
	}
	return &p
}

func useFileOp(args map[string]string, caps *apicaps.CapSet) bool {
	enabled := true
	if v, ok := args["BUILDKIT_DISABLE_FILEOP"]; ok {
		if b, err := strconv.ParseBool(v); err == nil {
			enabled = !b
		}
	}
	return enabled && caps != nil && caps.Supports(pb.CapFileBase) == nil
}

func location(sm *llb.SourceMap, locations []parser.Range) llb.ConstraintsOpt {
	loc := make([]*pb.Range, 0, len(locations))
	for _, l := range locations {
		loc = append(loc, &pb.Range{
			Start: pb.Position{
				Line:      int32(l.Start.Line),
				Character: int32(l.Start.Character),
			},
			End: pb.Position{
				Line:      int32(l.End.Line),
				Character: int32(l.End.Character),
			},
		})
	}
	return sm.Location(loc)
}

func summarizeHeredoc(doc string) string {
	doc = strings.TrimSpace(doc)
	lines := strings.Split(strings.ReplaceAll(doc, "\r\n", "\n"), "\n")
	summary := lines[0]
	if len(lines) > 1 {
		summary += "..."
	}
	return summary
}

func commonImageNames() []string {
	repos := []string{
		"alpine", "busybox", "centos", "debian", "golang", "ubuntu", "fedora",
	}
	out := make([]string, 0, len(repos)*4)
	for _, name := range repos {
		out = append(out, name, "docker.io/library"+name, name+":latest", "docker.io/library"+name+":latest")
	}
	return out
}
