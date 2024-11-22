package dockerfile2llb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/containerd/platforms"
	"github.com/distribution/reference"
	"github.com/docker/go-connections/nat"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/client/llb/imagemetaresolver"
	"github.com/moby/buildkit/client/llb/sourceresolver"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/linter"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/moby/buildkit/frontend/dockerfile/shell"
	"github.com/moby/buildkit/frontend/dockerui"
	"github.com/moby/buildkit/frontend/subrequests/lint"
	"github.com/moby/buildkit/frontend/subrequests/outline"
	"github.com/moby/buildkit/frontend/subrequests/targets"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/apicaps"
	"github.com/moby/buildkit/util/gitutil"
	"github.com/moby/buildkit/util/suggest"
	"github.com/moby/buildkit/util/system"
	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	"github.com/moby/patternmatcher"
	"github.com/moby/sys/signal"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

const (
	emptyImageName = "scratch"
	historyComment = "buildkit.dockerfile.v0"

	sbomScanContext = "BUILDKIT_SBOM_SCAN_CONTEXT"
	sbomScanStage   = "BUILDKIT_SBOM_SCAN_STAGE"
)

var (
	secretsRegexpOnce  sync.Once
	secretsRegexp      *regexp.Regexp
	secretsAllowRegexp *regexp.Regexp
)

var nonEnvArgs = map[string]struct{}{
	sbomScanContext: {},
	sbomScanStage:   {},
}

type ConvertOpt struct {
	dockerui.Config
	Client         *dockerui.Client
	MainContext    *llb.State
	SourceMap      *llb.SourceMap
	TargetPlatform *ocispecs.Platform
	MetaResolver   llb.ImageMetaResolver
	LLBCaps        *apicaps.CapSet
	Warn           linter.LintWarnFunc
	AllStages      bool
}

type SBOMTargets struct {
	Core   llb.State
	Extras map[string]llb.State

	IgnoreCache bool
}

func Dockerfile2LLB(ctx context.Context, dt []byte, opt ConvertOpt) (st *llb.State, img, baseImg *dockerspec.DockerOCIImage, sbom *SBOMTargets, err error) {
	ds, err := toDispatchState(ctx, dt, opt)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	sbom = &SBOMTargets{
		Core:   ds.state,
		Extras: map[string]llb.State{},
	}
	if ds.scanContext {
		sbom.Extras["context"] = ds.opt.buildContext
	}
	if ds.ignoreCache {
		sbom.IgnoreCache = true
	}
	for dsi := range allReachableStages(ds) {
		if ds != dsi && dsi.scanStage {
			sbom.Extras[dsi.stageName] = dsi.state
			if dsi.ignoreCache {
				sbom.IgnoreCache = true
			}
		}
	}

	return &ds.state, &ds.image, ds.baseImg, sbom, nil
}

func Dockerfile2Outline(ctx context.Context, dt []byte, opt ConvertOpt) (*outline.Outline, error) {
	ds, err := toDispatchState(ctx, dt, opt)
	if err != nil {
		return nil, err
	}
	o := ds.Outline(dt)
	return &o, nil
}

func DockerfileLint(ctx context.Context, dt []byte, opt ConvertOpt) (*lint.LintResults, error) {
	results := &lint.LintResults{}
	sourceIndex := results.AddSource(opt.SourceMap)
	opt.Warn = func(rulename, description, url, fmtmsg string, location []parser.Range) {
		results.AddWarning(rulename, description, url, fmtmsg, sourceIndex, location)
	}
	// for lint, no target means all targets
	if opt.Target == "" {
		opt.AllStages = true
	}

	_, err := toDispatchState(ctx, dt, opt)

	var errLoc *parser.ErrorLocation
	if err != nil {
		buildErr := &lint.BuildError{
			Message: err.Error(),
		}
		if errors.As(err, &errLoc) {
			ranges := mergeLocations(errLoc.Locations...)
			buildErr.Location = toPBLocation(sourceIndex, ranges)
		}
		results.Error = buildErr
	}
	return results, nil
}

func ListTargets(ctx context.Context, dt []byte) (*targets.List, error) {
	dockerfile, err := parser.Parse(bytes.NewReader(dt))
	if err != nil {
		return nil, err
	}

	stages, _, err := instructions.Parse(dockerfile.AST, nil)
	if err != nil {
		return nil, err
	}

	l := &targets.List{
		Sources: [][]byte{dt},
	}

	for i, s := range stages {
		t := targets.Target{
			Name:        s.Name,
			Description: s.Comment,
			Default:     i == len(stages)-1,
			Base:        s.BaseName,
			Platform:    s.Platform,
			Location:    toSourceLocation(s.Location),
		}
		l.Targets = append(l.Targets, t)
	}
	return l, nil
}

func newRuleLinter(dt []byte, opt *ConvertOpt) (*linter.Linter, error) {
	var lintConfig *linter.Config
	if opt.Client != nil && opt.Client.LinterConfig != nil {
		lintConfig = opt.Client.LinterConfig
	} else {
		var err error
		lintOptionStr, _, _, _ := parser.ParseDirective("check", dt)
		lintConfig, err = linter.ParseLintOptions(lintOptionStr)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse check options")
		}
	}
	lintConfig.Warn = opt.Warn
	return linter.New(lintConfig), nil
}

func toDispatchState(ctx context.Context, dt []byte, opt ConvertOpt) (*dispatchState, error) {
	if len(dt) == 0 {
		return nil, errors.Errorf("the Dockerfile cannot be empty")
	}

	if opt.Client != nil && opt.MainContext != nil {
		return nil, errors.Errorf("Client and MainContext cannot both be provided")
	}

	namedContext := func(ctx context.Context, name string, copt dockerui.ContextOpt) (*llb.State, *dockerspec.DockerOCIImage, error) {
		if opt.Client == nil {
			return nil, nil, nil
		}
		if !strings.EqualFold(name, "scratch") && !strings.EqualFold(name, "context") {
			if copt.Platform == nil {
				copt.Platform = opt.TargetPlatform
			}
			return opt.Client.NamedContext(ctx, name, copt)
		}
		return nil, nil, nil
	}

	lint, err := newRuleLinter(dt, &opt)
	if err != nil {
		return nil, err
	}

	if opt.Client != nil && opt.LLBCaps == nil {
		caps := opt.Client.BuildOpts().LLBCaps
		opt.LLBCaps = &caps
	}

	dockerfile, err := parser.Parse(bytes.NewReader(dt))
	if err != nil {
		return nil, err
	}

	// Moby still uses the `dockerfile.PrintWarnings` method to print non-empty
	// continuation line warnings. We iterate over those warnings here.
	for _, warning := range dockerfile.Warnings {
		// The `dockerfile.Warnings` *should* only contain warnings about empty continuation
		// lines, but we'll check the warning message to be sure, so that we don't accidentally
		// process warnings that are not related to empty continuation lines twice.
		if warning.URL == linter.RuleNoEmptyContinuation.URL {
			location := []parser.Range{*warning.Location}
			msg := linter.RuleNoEmptyContinuation.Format()
			lint.Run(&linter.RuleNoEmptyContinuation, location, msg)
		}
	}

	proxyEnv := proxyEnvFromBuildArgs(opt.BuildArgs)

	stages, argCmds, err := instructions.Parse(dockerfile.AST, lint)
	if err != nil {
		return nil, err
	}
	if len(stages) == 0 {
		return nil, errors.New("dockerfile contains no stages to build")
	}
	validateStageNames(stages, lint)
	validateCommandCasing(stages, lint)

	platformOpt := buildPlatformOpt(&opt)
	targetName := opt.Target
	if targetName == "" {
		targetName = stages[len(stages)-1].Name
	}
	globalArgs := defaultArgs(platformOpt, opt.BuildArgs, targetName)

	shlex := shell.NewLex(dockerfile.EscapeToken)
	outline := newOutlineCapture()

	// Validate that base images continue to be valid even
	// when no build arguments are used.
	validateBaseImagesWithDefaultArgs(stages, shlex, globalArgs, argCmds, lint)

	// Rebuild the arguments using the provided build arguments
	// for the remainder of the build.
	globalArgs, outline.allArgs, err = buildMetaArgs(globalArgs, shlex, argCmds, opt.BuildArgs)
	if err != nil {
		return nil, err
	}

	metaResolver := opt.MetaResolver
	if metaResolver == nil {
		metaResolver = imagemetaresolver.Default()
	}

	allDispatchStates := newDispatchStates()

	// set base state for every image
	for i, st := range stages {
		nameMatch, err := shlex.ProcessWordWithMatches(st.BaseName, globalArgs)
		argKeys := unusedFromArgsCheckKeys(globalArgs, outline.allArgs)
		reportUnusedFromArgs(argKeys, nameMatch.Unmatched, st.Location, lint)
		used := nameMatch.Matched
		if used == nil {
			used = map[string]struct{}{}
		}

		if err != nil {
			return nil, parser.WithLocation(err, st.Location)
		}
		if nameMatch.Result == "" {
			return nil, parser.WithLocation(errors.Errorf("base name (%s) should not be blank", st.BaseName), st.Location)
		}
		st.BaseName = nameMatch.Result

		ds := &dispatchState{
			stage:          st,
			deps:           make(map[*dispatchState]instructions.Command),
			ctxPaths:       make(map[string]struct{}),
			paths:          make(map[string]struct{}),
			stageName:      st.Name,
			prefixPlatform: opt.MultiPlatformRequested,
			outline:        outline.clone(),
			epoch:          opt.Epoch,
		}

		if v := st.Platform; v != "" {
			platMatch, err := shlex.ProcessWordWithMatches(v, globalArgs)
			argKeys := unusedFromArgsCheckKeys(globalArgs, outline.allArgs)
			reportUnusedFromArgs(argKeys, platMatch.Unmatched, st.Location, lint)
			reportRedundantTargetPlatform(st.Platform, platMatch, st.Location, globalArgs, lint)
			reportConstPlatformDisallowed(st.Name, platMatch, st.Location, lint)

			if err != nil {
				return nil, parser.WithLocation(errors.Wrapf(err, "failed to process arguments for platform %s", platMatch.Result), st.Location)
			}

			if platMatch.Result == "" {
				err := errors.Errorf("empty platform value from expression %s", v)
				err = parser.WithLocation(err, st.Location)
				err = wrapSuggestAny(err, platMatch.Unmatched, globalArgs.Keys())
				return nil, err
			}

			p, err := platforms.Parse(platMatch.Result)
			if err != nil {
				err = parser.WithLocation(err, st.Location)
				err = wrapSuggestAny(err, platMatch.Unmatched, globalArgs.Keys())
				return nil, parser.WithLocation(errors.Wrapf(err, "failed to parse platform %s", v), st.Location)
			}

			for k := range platMatch.Matched {
				used[k] = struct{}{}
			}

			ds.platform = &p
		}

		if st.Name != "" {
			s, img, err := namedContext(ctx, st.Name, dockerui.ContextOpt{
				Platform:       ds.platform,
				ResolveMode:    opt.ImageResolveMode.String(),
				AsyncLocalOpts: ds.asyncLocalOpts,
			})
			if err != nil {
				return nil, err
			}
			if s != nil {
				ds.dispatched = true
				ds.state = *s
				if img != nil {
					// timestamps are inherited as-is, regardless to SOURCE_DATE_EPOCH
					// https://github.com/moby/buildkit/issues/4614
					ds.image = *img
					if img.Architecture != "" && img.OS != "" {
						ds.platform = &ocispecs.Platform{
							OS:           img.OS,
							Architecture: img.Architecture,
							Variant:      img.Variant,
							OSVersion:    img.OSVersion,
						}
						if img.OSFeatures != nil {
							ds.platform.OSFeatures = append([]string{}, img.OSFeatures...)
						}
					}
				}
				allDispatchStates.addState(ds)
				continue
			}
		}

		if st.Name == "" {
			ds.stageName = fmt.Sprintf("stage-%d", i)
		}

		allDispatchStates.addState(ds)

		for k := range used {
			ds.outline.usedArgs[k] = struct{}{}
		}

		total := 0
		if ds.stage.BaseName != emptyImageName && ds.base == nil {
			total = 1
		}
		for _, cmd := range ds.stage.Commands {
			switch cmd.(type) {
			case *instructions.AddCommand, *instructions.CopyCommand, *instructions.RunCommand:
				total++
			case *instructions.WorkdirCommand:
				total++
			}
		}
		ds.cmdTotal = total
		if opt.Client != nil {
			ds.ignoreCache = opt.Client.IsNoCache(st.Name)
		}
	}

	var target *dispatchState
	if opt.Target == "" {
		target = allDispatchStates.lastTarget()
	} else {
		var ok bool
		target, ok = allDispatchStates.findStateByName(opt.Target)
		if !ok {
			return nil, errors.Errorf("target stage %q could not be found", opt.Target)
		}
	}

	// fill dependencies to stages so unreachable ones can avoid loading image configs
	for _, d := range allDispatchStates.states {
		d.commands = make([]command, len(d.stage.Commands))
		for i, cmd := range d.stage.Commands {
			newCmd, err := toCommand(cmd, allDispatchStates)
			if err != nil {
				return nil, err
			}
			d.commands[i] = newCmd
			for _, src := range newCmd.sources {
				if src != nil {
					d.deps[src] = cmd
					if src.unregistered {
						allDispatchStates.addState(src)
					}
				}
			}
		}
	}

	if err := validateCircularDependency(allDispatchStates.states); err != nil {
		return nil, err
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

	resolveReachableStages := func(ctx context.Context, all []*dispatchState, target *dispatchState) (map[*dispatchState]struct{}, error) {
		allReachable := allReachableStages(target)
		eg, ctx := errgroup.WithContext(ctx)
		for i, d := range all {
			_, reachable := allReachable[d]
			if opt.AllStages {
				reachable = true
			}
			// resolve image config for every stage
			if d.base == nil && !d.dispatched && !d.resolved {
				d.resolved = reachable // avoid re-resolving if called again after onbuild
				if d.stage.BaseName == emptyImageName {
					d.state = llb.Scratch()
					d.image = emptyImage(platformOpt.targetPlatform)
					d.platform = &platformOpt.targetPlatform
					if d.unregistered {
						d.dispatched = true
					}
					continue
				}
				func(i int, d *dispatchState) {
					eg.Go(func() (err error) {
						defer func() {
							if err != nil {
								err = parser.WithLocation(err, d.stage.Location)
							}
							if d.unregistered {
								// implicit stages don't need further dispatch
								d.dispatched = true
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
						st, img, err := namedContext(ctx, d.stage.BaseName, dockerui.ContextOpt{
							ResolveMode:    opt.ImageResolveMode.String(),
							Platform:       platform,
							AsyncLocalOpts: d.asyncLocalOpts,
						})
						if err != nil {
							return err
						}
						if st != nil {
							if img != nil {
								d.image = *img
							} else {
								d.image = emptyImage(platformOpt.targetPlatform)
							}
							d.state = st.Platform(*platform)
							d.platform = platform
							return nil
						}
						if reachable {
							prefix := "["
							if opt.MultiPlatformRequested && platform != nil {
								prefix += platforms.Format(*platform) + " "
							}
							prefix += "internal]"
							mutRef, dgst, dt, err := metaResolver.ResolveImageConfig(ctx, d.stage.BaseName, sourceresolver.Opt{
								LogName:  fmt.Sprintf("%s load metadata for %s", prefix, d.stage.BaseName),
								Platform: platform,
								ImageOpt: &sourceresolver.ResolveImageOpt{
									ResolveMode: opt.ImageResolveMode.String(),
								},
							})
							if err != nil {
								return suggest.WrapError(errors.Wrap(err, origName), origName, append(allStageNames, commonImageNames()...), true)
							}

							if ref.String() != mutRef {
								ref, err = reference.ParseNormalizedNamed(mutRef)
								if err != nil {
									return errors.Wrapf(err, "failed to parse ref %q", mutRef)
								}
							}
							var img dockerspec.DockerOCIImage
							if err := json.Unmarshal(dt, &img); err != nil {
								return errors.Wrap(err, "failed to parse image config")
							}
							d.baseImg = cloneX(&img) // immutable
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
							d.image = img
						}
						if isScratch {
							d.state = llb.Scratch()
						} else {
							d.state = llb.Image(d.stage.BaseName,
								dfCmd(d.stage.SourceCode),
								llb.Platform(*platform),
								opt.ImageResolveMode,
								llb.WithCustomName(prefixCommand(d, "FROM "+d.stage.BaseName, opt.MultiPlatformRequested, platform, emptyEnvs{})),
								location(opt.SourceMap, d.stage.Location),
							)
							if reachable {
								validateBaseImagePlatform(origName, *platform, d.image.Platform, d.stage.Location, lint)
							}
						}
						d.platform = platform
						return nil
					})
				}(i, d)
			}
		}

		if err := eg.Wait(); err != nil {
			return nil, err
		}
		return allReachable, nil
	}

	var allReachable map[*dispatchState]struct{}
	for {
		allReachable, err = resolveReachableStages(ctx, allDispatchStates.states, target)
		if err != nil {
			return nil, err
		}

		// initialize onbuild triggers in case they create new dependencies
		newDeps := false
		for d := range allReachable {
			d.init()

			onbuilds := slices.Clone(d.image.Config.OnBuild)
			if d.base != nil && !d.onBuildInit {
				for _, cmd := range d.base.commands {
					if obCmd, ok := cmd.Command.(*instructions.OnbuildCommand); ok {
						onbuilds = append(onbuilds, obCmd.Expression)
					}
				}
				d.onBuildInit = true
			}

			if len(onbuilds) > 0 {
				if b, err := initOnBuildTriggers(d, onbuilds, allDispatchStates); err != nil {
					return nil, parser.SetLocation(err, d.stage.Location)
				} else if b {
					newDeps = true
				}
				d.image.Config.OnBuild = nil
			}
		}
		// in case new dependencies were added, we need to re-resolve reachable stages
		if !newDeps {
			break
		}
	}

	buildContext := &mutableOutput{}
	ctxPaths := map[string]struct{}{}

	var dockerIgnoreMatcher *patternmatcher.PatternMatcher
	if opt.Client != nil {
		dockerIgnorePatterns, err := opt.Client.DockerIgnorePatterns(ctx)
		if err != nil {
			return nil, err
		}
		if len(dockerIgnorePatterns) > 0 {
			dockerIgnoreMatcher, err = patternmatcher.New(dockerIgnorePatterns)
			if err != nil {
				return nil, err
			}
		}
	}

	for _, d := range allDispatchStates.states {
		if !opt.AllStages {
			if _, ok := allReachable[d]; !ok || d.dispatched {
				continue
			}
		}
		d.init()
		d.dispatched = true

		// Ensure platform is set.
		if d.platform == nil {
			d.platform = &d.opt.targetPlatform
		}

		// make sure that PATH is always set
		if _, ok := shell.EnvsFromSlice(d.image.Config.Env).Get("PATH"); !ok {
			var osName string
			if d.platform != nil {
				osName = d.platform.OS
			}
			d.image.Config.Env = append(d.image.Config.Env, "PATH="+system.DefaultPathEnv(osName))
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
				return nil, parser.WithLocation(err, d.stage.Location)
			}
		}
		if d.image.Config.User != "" {
			if err = dispatchUser(d, &instructions.UserCommand{User: d.image.Config.User}, false); err != nil {
				return nil, parser.WithLocation(err, d.stage.Location)
			}
		}

		d.state = d.state.Network(opt.NetworkMode)

		opt := dispatchOpt{
			allDispatchStates:   allDispatchStates,
			globalArgs:          globalArgs,
			buildArgValues:      opt.BuildArgs,
			shlex:               shlex,
			buildContext:        llb.NewState(buildContext),
			proxyEnv:            proxyEnv,
			cacheIDNamespace:    opt.CacheIDNamespace,
			buildPlatforms:      platformOpt.buildPlatforms,
			targetPlatform:      platformOpt.targetPlatform,
			extraHosts:          opt.ExtraHosts,
			shmSize:             opt.ShmSize,
			ulimit:              opt.Ulimits,
			cgroupParent:        opt.CgroupParent,
			llbCaps:             opt.LLBCaps,
			sourceMap:           opt.SourceMap,
			lint:                lint,
			dockerIgnoreMatcher: dockerIgnoreMatcher,
		}

		for _, cmd := range d.commands {
			if err := dispatch(d, cmd, opt); err != nil {
				return nil, parser.WithLocation(err, cmd.Location())
			}
		}
		d.opt = opt

		for p := range d.ctxPaths {
			ctxPaths[p] = struct{}{}
		}

		for _, name := range []string{sbomScanContext, sbomScanStage} {
			var b bool
			if v, ok := d.opt.globalArgs.Get(name); ok {
				b = isEnabledForStage(d.stageName, v)
			}
			for _, kv := range d.buildArgs {
				if kv.Key == name && kv.Value != nil {
					b = isEnabledForStage(d.stageName, *kv.Value)
				}
			}
			if b {
				if name == sbomScanContext {
					d.scanContext = true
				} else {
					d.scanStage = true
				}
			}
		}
	}

	// Ensure the entirety of the target state is marked as used.
	// This is done after we've already evaluated every stage to ensure
	// the paths attribute is set correctly.
	target.paths["/"] = struct{}{}

	if len(opt.Labels) != 0 && target.image.Config.Labels == nil {
		target.image.Config.Labels = make(map[string]string, len(opt.Labels))
	}
	maps.Copy(target.image.Config.Labels, opt.Labels)

	// If lint.Error() returns an error, it means that
	// there were warnings, and that our linter has been
	// configured to return an error on warnings,
	// so we appropriately return that error here.
	if err := lint.Error(); err != nil {
		return nil, err
	}

	opts := filterPaths(ctxPaths)
	bctx := opt.MainContext
	if opt.Client != nil {
		bctx, err = opt.Client.MainContext(ctx, opts...)
		if err != nil {
			return nil, err
		}
	} else if bctx == nil {
		bctx = dockerui.DefaultMainContext(opts...)
	}
	buildContext.Output = bctx.Output()

	defaults := []llb.ConstraintsOpt{
		llb.Platform(platformOpt.targetPlatform),
	}
	if opt.LLBCaps != nil {
		defaults = append(defaults, llb.WithCaps(*opt.LLBCaps))
	}
	target.state = target.state.SetMarshalDefaults(defaults...)

	if !platformOpt.implicitTarget {
		target.image.OS = platformOpt.targetPlatform.OS
		target.image.Architecture = platformOpt.targetPlatform.Architecture
		target.image.Variant = platformOpt.targetPlatform.Variant
		target.image.OSVersion = platformOpt.targetPlatform.OSVersion
		if platformOpt.targetPlatform.OSFeatures != nil {
			target.image.OSFeatures = append([]string{}, platformOpt.targetPlatform.OSFeatures...)
		}
	}

	return target, nil
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
						stage:        instructions.Stage{BaseName: c.From, Location: c.Location()},
						deps:         make(map[*dispatchState]instructions.Command),
						paths:        make(map[string]struct{}),
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
	allDispatchStates   *dispatchStates
	globalArgs          shell.EnvGetter
	buildArgValues      map[string]string
	shlex               *shell.Lex
	buildContext        llb.State
	proxyEnv            *llb.ProxyEnv
	cacheIDNamespace    string
	targetPlatform      ocispecs.Platform
	buildPlatforms      []ocispecs.Platform
	extraHosts          []llb.HostIP
	shmSize             int64
	ulimit              []*pb.Ulimit
	cgroupParent        string
	llbCaps             *apicaps.CapSet
	sourceMap           *llb.SourceMap
	lint                *linter.Linter
	dockerIgnoreMatcher *patternmatcher.PatternMatcher
}

func getEnv(state llb.State) shell.EnvGetter {
	return &envsFromState{state: &state}
}

type envsFromState struct {
	state *llb.State
	once  sync.Once
	env   shell.EnvGetter
}

func (e *envsFromState) init() {
	env, err := e.state.Env(context.TODO())
	if err != nil {
		return
	}
	e.env = env
}

func (e *envsFromState) Get(key string) (string, bool) {
	e.once.Do(e.init)
	return e.env.Get(key)
}

func (e *envsFromState) Keys() []string {
	e.once.Do(e.init)
	return e.env.Keys()
}

func dispatch(d *dispatchState, cmd command, opt dispatchOpt) error {
	d.cmdIsOnBuild = cmd.isOnBuild
	var err error
	// ARG command value could be ignored, so defer handling the expansion error
	_, isArg := cmd.Command.(*instructions.ArgCommand)
	if ex, ok := cmd.Command.(instructions.SupportsSingleWordExpansion); ok && !isArg {
		err := ex.Expand(func(word string) (string, error) {
			env := getEnv(d.state)
			newword, unmatched, err := opt.shlex.ProcessWord(word, env)
			reportUnmatchedVariables(cmd, d.buildArgs, env, unmatched, &opt)
			return newword, err
		})
		if err != nil {
			return err
		}
	}
	if ex, ok := cmd.Command.(instructions.SupportsSingleWordExpansionRaw); ok {
		err := ex.ExpandRaw(func(word string) (string, error) {
			lex := shell.NewLex('\\')
			lex.SkipProcessQuotes = true
			env := getEnv(d.state)
			newword, unmatched, err := lex.ProcessWord(word, env)
			reportUnmatchedVariables(cmd, d.buildArgs, env, unmatched, &opt)
			return newword, err
		})
		if err != nil {
			return err
		}
	}

	switch c := cmd.Command.(type) {
	case *instructions.MaintainerCommand:
		err = dispatchMaintainer(d, c)
	case *instructions.EnvCommand:
		err = dispatchEnv(d, c, opt.lint)
	case *instructions.RunCommand:
		err = dispatchRun(d, c, opt.proxyEnv, cmd.sources, opt)
	case *instructions.WorkdirCommand:
		err = dispatchWorkdir(d, c, true, &opt)
	case *instructions.AddCommand:
		var checksum digest.Digest
		if c.Checksum != "" {
			checksum, err = digest.Parse(c.Checksum)
		}
		if err == nil {
			err = dispatchCopy(d, copyConfig{
				params:          c.SourcesAndDest,
				excludePatterns: c.ExcludePatterns,
				source:          opt.buildContext,
				isAddCommand:    true,
				cmdToPrint:      c,
				chown:           c.Chown,
				chmod:           c.Chmod,
				link:            c.Link,
				keepGitDir:      c.KeepGitDir,
				checksum:        checksum,
				location:        c.Location(),
				ignoreMatcher:   opt.dockerIgnoreMatcher,
				opt:             opt,
			})
		}
		if err == nil {
			for _, src := range c.SourcePaths {
				if !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://") {
					d.ctxPaths[path.Join("/", filepath.ToSlash(src))] = struct{}{}
				}
			}
		}
	case *instructions.LabelCommand:
		err = dispatchLabel(d, c, opt.lint)
	case *instructions.OnbuildCommand:
		err = dispatchOnbuild(d, c)
	case *instructions.CmdCommand:
		err = dispatchCmd(d, c, opt.lint)
	case *instructions.EntrypointCommand:
		err = dispatchEntrypoint(d, c, opt.lint)
	case *instructions.HealthCheckCommand:
		err = dispatchHealthcheck(d, c, opt.lint)
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
		err = dispatchArg(d, c, &opt)
	case *instructions.CopyCommand:
		l := opt.buildContext
		var ignoreMatcher *patternmatcher.PatternMatcher
		if len(cmd.sources) != 0 {
			src := cmd.sources[0]
			if !src.dispatched {
				return errors.Errorf("cannot copy from stage %q, it needs to be defined before current stage %q", c.From, d.stageName)
			}
			l = src.state
		} else {
			ignoreMatcher = opt.dockerIgnoreMatcher
		}
		err = dispatchCopy(d, copyConfig{
			params:          c.SourcesAndDest,
			excludePatterns: c.ExcludePatterns,
			source:          l,
			isAddCommand:    false,
			cmdToPrint:      c,
			chown:           c.Chown,
			chmod:           c.Chmod,
			link:            c.Link,
			parents:         c.Parents,
			location:        c.Location(),
			ignoreMatcher:   ignoreMatcher,
			opt:             opt,
		})
		if err == nil {
			if len(cmd.sources) == 0 {
				for _, src := range c.SourcePaths {
					d.ctxPaths[path.Join("/", filepath.ToSlash(src))] = struct{}{}
				}
			} else {
				source := cmd.sources[0]
				if source.paths == nil {
					source.paths = make(map[string]struct{})
				}
				for _, src := range c.SourcePaths {
					source.paths[path.Join("/", filepath.ToSlash(src))] = struct{}{}
				}
			}
		}
	default:
	}
	return err
}

type dispatchState struct {
	opt         dispatchOpt
	state       llb.State
	image       dockerspec.DockerOCIImage
	platform    *ocispecs.Platform
	stage       instructions.Stage
	base        *dispatchState
	baseImg     *dockerspec.DockerOCIImage // immutable, unlike image
	dispatched  bool
	resolved    bool // resolved is set to true if base image has been resolved
	onBuildInit bool
	deps        map[*dispatchState]instructions.Command
	buildArgs   []instructions.KeyValuePairOptional
	commands    []command
	// ctxPaths marks the paths this dispatchState uses from the build context.
	ctxPaths map[string]struct{}
	// paths marks the paths that are used by this dispatchState.
	paths          map[string]struct{}
	ignoreCache    bool
	unregistered   bool
	stageName      string
	cmdIndex       int
	cmdIsOnBuild   bool
	cmdTotal       int
	prefixPlatform bool
	outline        outlineCapture
	epoch          *time.Time
	scanStage      bool
	scanContext    bool
	// workdirSet is set to true if a workdir has been set
	// within the current dockerfile.
	workdirSet bool

	entrypoint  instructionTracker
	cmd         instructionTracker
	healthcheck instructionTracker
}

func (ds *dispatchState) asyncLocalOpts() []llb.LocalOption {
	return filterPaths(ds.paths)
}

// init is invoked when the dispatch state inherits its attributes
// from the base image.
func (ds *dispatchState) init() {
	// mark as initialized, used to determine states that have not been dispatched yet
	if ds.base == nil {
		return
	}

	ds.state = ds.base.state
	ds.platform = ds.base.platform
	ds.image = clone(ds.base.image)
	ds.baseImg = cloneX(ds.base.baseImg)
	// Utilize the same path index as our base image so we propagate
	// the paths we use back to the base image.
	ds.paths = ds.base.paths
	ds.workdirSet = ds.base.workdirSet
	ds.buildArgs = append(ds.buildArgs, ds.base.buildArgs...)
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
		ds.outline = d.outline.clone()
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
	sources   []*dispatchState
	isOnBuild bool
}

// initOnBuildTriggers initializes the onbuild triggers and creates the commands and dependecies for them.
// It returns true if there were any new dependencies added that need to be resolved.
func initOnBuildTriggers(d *dispatchState, triggers []string, allDispatchStates *dispatchStates) (bool, error) {
	hasNewDeps := false
	commands := make([]command, 0, len(triggers))

	for _, trigger := range triggers {
		ast, err := parser.Parse(strings.NewReader(trigger))
		if err != nil {
			return false, err
		}
		if len(ast.AST.Children) != 1 {
			return false, errors.New("onbuild trigger should be a single expression")
		}
		node := ast.AST.Children[0]
		// reset the location to the onbuild trigger
		node.StartLine, node.EndLine = rangeStartEnd(d.stage.Location)
		ic, err := instructions.ParseCommand(ast.AST.Children[0])
		if err != nil {
			return false, err
		}
		cmd, err := toCommand(ic, allDispatchStates)
		if err != nil {
			return false, err
		}
		cmd.isOnBuild = true
		if len(cmd.sources) > 0 {
			hasNewDeps = true
		}

		commands = append(commands, cmd)

		for _, src := range cmd.sources {
			if src != nil {
				d.deps[src] = cmd
				if src.unregistered {
					allDispatchStates.addState(src)
				}
			}
		}
	}
	d.commands = append(commands, d.commands...)
	d.cmdTotal += len(commands)

	return hasNewDeps, nil
}

func dispatchEnv(d *dispatchState, c *instructions.EnvCommand, lint *linter.Linter) error {
	commitMessage := bytes.NewBufferString("ENV")
	for _, e := range c.Env {
		if e.NoDelim {
			msg := linter.RuleLegacyKeyValueFormat.Format(c.Name())
			lint.Run(&linter.RuleLegacyKeyValueFormat, c.Location(), msg)
		}
		validateNoSecretKey("ENV", e.Key, c.Location(), lint)
		commitMessage.WriteString(" " + e.String())
		d.state = d.state.AddEnv(e.Key, e.Value)
		d.image.Config.Env = addEnv(d.image.Config.Env, e.Key, e.Value)
	}
	return commitToHistory(&d.image, commitMessage.String(), false, nil, d.epoch)
}

func dispatchRun(d *dispatchState, c *instructions.RunCommand, proxy *llb.ProxyEnv, sources []*dispatchState, dopt dispatchOpt) error {
	var opt []llb.RunOption

	customname := c.String()

	// Run command can potentially access any file. Mark the full filesystem as used.
	d.paths["/"] = struct{}{}

	var args []string = c.CmdLine
	if len(c.Files) > 0 {
		if len(args) != 1 || !c.PrependShell {
			return errors.Errorf("parsing produced an invalid run command: %v", args)
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
					dockerui.WithInternalName("preparing inline document"),
					llb.Platform(*d.platform),
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
		// Don't pass the linter function because we do not report a warning for
		// shell usage on run commands.
		args = withShell(d.image, args)
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
	env := getEnv(d.state)
	opt = append(opt, llb.WithCustomName(prefixCommand(d, uppercaseCmd(processCmdEnv(&shlex, customname, withSecretEnvMask(c, env))), d.prefixPlatform, pl, env)))
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
	return commitToHistory(&d.image, "RUN "+runCommandString(args, d.buildArgs, env), true, &d.state, d.epoch)
}

func dispatchWorkdir(d *dispatchState, c *instructions.WorkdirCommand, commit bool, opt *dispatchOpt) error {
	if commit {
		// This linter rule checks if workdir has been set to an absolute value locally
		// within the current dockerfile. Absolute paths in base images are ignored
		// because they might change and it is not advised to rely on them.
		//
		// We only run this check when commit is true. Commit is true when we are performing
		// this operation on a local call to workdir rather than one coming from
		// the base image. We only check the first instance of workdir being set
		// so successive relative paths are ignored because every instance is fixed
		// by fixing the first one.
		if !d.workdirSet && !system.IsAbs(c.Path, d.platform.OS) {
			msg := linter.RuleWorkdirRelativePath.Format(c.Path)
			opt.lint.Run(&linter.RuleWorkdirRelativePath, c.Location(), msg)
		}
		d.workdirSet = true
	}

	wd, err := system.NormalizeWorkdir(d.image.Config.WorkingDir, c.Path, d.platform.OS)
	if err != nil {
		return errors.Wrap(err, "normalizing workdir")
	}

	// NormalizeWorkdir returns paths with platform specific separators. For Windows
	// this will be of the form: \some\path, which is needed later when we pass it to
	// HCS.
	d.image.Config.WorkingDir = wd

	// From this point forward, we can use UNIX style paths.
	wd = system.ToSlash(wd, d.platform.OS)
	d.state = d.state.Dir(wd)

	if commit {
		withLayer := false
		if wd != "/" {
			mkdirOpt := []llb.MkdirOption{llb.WithParents(true)}
			if user := d.image.Config.User; user != "" {
				mkdirOpt = append(mkdirOpt, llb.WithUser(user))
			}
			platform := opt.targetPlatform
			if d.platform != nil {
				platform = *d.platform
			}
			env := getEnv(d.state)
			d.state = d.state.File(llb.Mkdir(wd, 0755, mkdirOpt...),
				llb.WithCustomName(prefixCommand(d, uppercaseCmd(processCmdEnv(opt.shlex, c.String(), env)), d.prefixPlatform, &platform, env)),
				location(opt.sourceMap, c.Location()),
				llb.Platform(*d.platform),
			)
			withLayer = true
		}
		return commitToHistory(&d.image, "WORKDIR "+wd, withLayer, nil, d.epoch)
	}
	return nil
}

func dispatchCopy(d *dispatchState, cfg copyConfig) error {
	dest, err := pathRelativeToWorkingDir(d.state, cfg.params.DestPath, *d.platform)
	if err != nil {
		return err
	}

	var copyOpt []llb.CopyOption

	if cfg.chown != "" {
		copyOpt = append(copyOpt, llb.WithUser(cfg.chown))
	}

	if len(cfg.excludePatterns) > 0 {
		// in theory we don't need to check whether there are any exclude patterns,
		// as an empty list is a no-op. However, performing the check makes
		// the code easier to understand and costs virtually nothing.
		copyOpt = append(copyOpt, llb.WithExcludePatterns(cfg.excludePatterns))
	}

	var mode *llb.ChmodOpt
	if cfg.chmod != "" {
		mode = &llb.ChmodOpt{}
		p, err := strconv.ParseUint(cfg.chmod, 8, 32)
		nonOctalErr := errors.Errorf("invalid chmod parameter: '%v'. it should be octal string and between 0 and 07777", cfg.chmod)
		if err == nil {
			if p > 0o7777 {
				return nonOctalErr
			}
			mode.Mode = os.FileMode(p)
		} else {
			if featureCopyChmodNonOctalEnabled {
				mode.ModeStr = cfg.chmod
			} else {
				return nonOctalErr
			}
		}
	}

	if cfg.checksum != "" {
		if !cfg.isAddCommand {
			return errors.New("checksum can't be specified for COPY")
		}
		if len(cfg.params.SourcePaths) != 1 {
			return errors.New("checksum can't be specified for multiple sources")
		}
		if !isHTTPSource(cfg.params.SourcePaths[0]) {
			return errors.New("checksum can't be specified for non-HTTP(S) sources")
		}
	}

	commitMessage := bytes.NewBufferString("")
	if cfg.isAddCommand {
		commitMessage.WriteString("ADD")
	} else {
		commitMessage.WriteString("COPY")
	}

	if cfg.parents {
		commitMessage.WriteString(" " + "--parents")
	}
	if cfg.chown != "" {
		commitMessage.WriteString(" " + "--chown=" + cfg.chown)
	}
	if cfg.chmod != "" {
		commitMessage.WriteString(" " + "--chmod=" + cfg.chmod)
	}

	platform := cfg.opt.targetPlatform
	if d.platform != nil {
		platform = *d.platform
	}

	env := getEnv(d.state)
	name := uppercaseCmd(processCmdEnv(cfg.opt.shlex, cfg.cmdToPrint.String(), env))
	pgName := prefixCommand(d, name, d.prefixPlatform, &platform, env)

	var a *llb.FileAction

	for _, src := range cfg.params.SourcePaths {
		commitMessage.WriteString(" " + src)
		gitRef, gitRefErr := gitutil.ParseGitRef(src)
		if gitRefErr == nil && !gitRef.IndistinguishableFromLocal {
			if !cfg.isAddCommand {
				return errors.New("source can't be a git ref for COPY")
			}
			// TODO: print a warning (not an error) if gitRef.UnencryptedTCP is true
			commit := gitRef.Commit
			if gitRef.SubDir != "" {
				commit += ":" + gitRef.SubDir
			}
			gitOptions := []llb.GitOption{llb.WithCustomName(pgName)}
			if cfg.keepGitDir {
				gitOptions = append(gitOptions, llb.KeepGitDir())
			}
			st := llb.Git(gitRef.Remote, commit, gitOptions...)
			opts := append([]llb.CopyOption{&llb.CopyInfo{
				Mode:           mode,
				CreateDestPath: true,
			}}, copyOpt...)
			if a == nil {
				a = llb.Copy(st, "/", dest, opts...)
			} else {
				a = a.Copy(st, "/", dest, opts...)
			}
		} else if isHTTPSource(src) {
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

			st := llb.HTTP(src, llb.Filename(f), llb.WithCustomName(pgName), llb.Checksum(cfg.checksum), dfCmd(cfg.params))

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
			validateCopySourcePath(src, &cfg)
			var patterns []string
			if cfg.parents {
				// detect optional pivot point
				parent, pattern, ok := strings.Cut(src, "/./")
				if !ok {
					pattern = src
					src = "/"
				} else {
					src = parent
				}

				pattern, err = system.NormalizePath("/", pattern, d.platform.OS, false)
				if err != nil {
					return errors.Wrap(err, "removing drive letter")
				}

				patterns = []string{strings.TrimPrefix(pattern, "/")}
			}

			src, err = system.NormalizePath("/", src, d.platform.OS, false)
			if err != nil {
				return errors.Wrap(err, "removing drive letter")
			}

			opts := append([]llb.CopyOption{&llb.CopyInfo{
				Mode:                mode,
				FollowSymlinks:      true,
				CopyDirContentsOnly: true,
				IncludePatterns:     patterns,
				AttemptUnpack:       cfg.isAddCommand,
				CreateDestPath:      true,
				AllowWildcard:       true,
				AllowEmptyWildcard:  true,
			}}, copyOpt...)

			if a == nil {
				a = llb.Copy(cfg.source, src, dest, opts...)
			} else {
				a = a.Copy(cfg.source, src, dest, opts...)
			}
		}
	}

	for _, src := range cfg.params.SourceContents {
		commitMessage.WriteString(" <<" + src.Path)

		data := src.Data
		f, err := system.CheckSystemDriveAndRemoveDriveLetter(src.Path, d.platform.OS)
		if err != nil {
			return errors.Wrap(err, "removing drive letter")
		}
		st := llb.Scratch().File(
			llb.Mkfile(f, 0644, []byte(data)),
			dockerui.WithInternalName("preparing inline document"),
			llb.Platform(*d.platform),
		)

		opts := append([]llb.CopyOption{&llb.CopyInfo{
			Mode:           mode,
			CreateDestPath: true,
		}}, copyOpt...)

		if a == nil {
			a = llb.Copy(st, system.ToSlash(f, d.platform.OS), dest, opts...)
		} else {
			a = a.Copy(st, filepath.ToSlash(f), dest, opts...)
		}
	}

	commitMessage.WriteString(" " + cfg.params.DestPath)

	fileOpt := []llb.ConstraintsOpt{
		llb.WithCustomName(pgName),
		location(cfg.opt.sourceMap, cfg.location),
	}
	if d.ignoreCache {
		fileOpt = append(fileOpt, llb.IgnoreCache)
	}

	// cfg.opt.llbCaps can be nil in unit tests
	if cfg.opt.llbCaps != nil && cfg.opt.llbCaps.Supports(pb.CapMergeOp) == nil && cfg.link && cfg.chmod == "" {
		pgID := identity.NewID()
		d.cmdIndex-- // prefixCommand increases it
		pgName := prefixCommand(d, name, d.prefixPlatform, &platform, env)

		copyOpts := []llb.ConstraintsOpt{
			llb.Platform(*d.platform),
		}
		copyOpts = append(copyOpts, fileOpt...)
		copyOpts = append(copyOpts, llb.ProgressGroup(pgID, pgName, true))

		mergeOpts := append([]llb.ConstraintsOpt{}, fileOpt...)
		d.cmdIndex--
		mergeOpts = append(mergeOpts, llb.ProgressGroup(pgID, pgName, false), llb.WithCustomName(prefixCommand(d, "LINK "+name, d.prefixPlatform, &platform, env)))

		d.state = d.state.WithOutput(llb.Merge([]llb.State{d.state, llb.Scratch().File(a, copyOpts...)}, mergeOpts...).Output())
	} else {
		d.state = d.state.File(a, fileOpt...)
	}

	return commitToHistory(&d.image, commitMessage.String(), true, &d.state, d.epoch)
}

type copyConfig struct {
	params          instructions.SourcesAndDest
	excludePatterns []string
	source          llb.State
	isAddCommand    bool
	cmdToPrint      fmt.Stringer
	chown           string
	chmod           string
	link            bool
	keepGitDir      bool
	checksum        digest.Digest
	parents         bool
	location        []parser.Range
	ignoreMatcher   *patternmatcher.PatternMatcher
	opt             dispatchOpt
}

func dispatchMaintainer(d *dispatchState, c *instructions.MaintainerCommand) error {
	d.image.Author = c.Maintainer
	return commitToHistory(&d.image, fmt.Sprintf("MAINTAINER %v", c.Maintainer), false, nil, d.epoch)
}

func dispatchLabel(d *dispatchState, c *instructions.LabelCommand, lint *linter.Linter) error {
	commitMessage := bytes.NewBufferString("LABEL")
	if d.image.Config.Labels == nil {
		d.image.Config.Labels = make(map[string]string, len(c.Labels))
	}
	for _, v := range c.Labels {
		if v.NoDelim {
			msg := linter.RuleLegacyKeyValueFormat.Format(c.Name())
			lint.Run(&linter.RuleLegacyKeyValueFormat, c.Location(), msg)
		}
		d.image.Config.Labels[v.Key] = v.Value
		commitMessage.WriteString(" " + v.String())
	}
	return commitToHistory(&d.image, commitMessage.String(), false, nil, d.epoch)
}

func dispatchOnbuild(d *dispatchState, c *instructions.OnbuildCommand) error {
	d.image.Config.OnBuild = append(d.image.Config.OnBuild, c.Expression)
	return nil
}

func dispatchCmd(d *dispatchState, c *instructions.CmdCommand, lint *linter.Linter) error {
	validateUsedOnce(c, &d.cmd, lint)

	var args []string = c.CmdLine
	if c.PrependShell {
		if len(d.image.Config.Shell) == 0 {
			msg := linter.RuleJSONArgsRecommended.Format(c.Name())
			lint.Run(&linter.RuleJSONArgsRecommended, c.Location(), msg)
		}
		args = withShell(d.image, args)
	}
	d.image.Config.Cmd = args
	d.image.Config.ArgsEscaped = true //nolint:staticcheck // ignore SA1019: field is deprecated in OCI Image spec, but used for backward-compatibility with Docker image spec.
	return commitToHistory(&d.image, fmt.Sprintf("CMD %q", args), false, nil, d.epoch)
}

func dispatchEntrypoint(d *dispatchState, c *instructions.EntrypointCommand, lint *linter.Linter) error {
	validateUsedOnce(c, &d.entrypoint, lint)

	var args []string = c.CmdLine
	if c.PrependShell {
		if len(d.image.Config.Shell) == 0 {
			msg := linter.RuleJSONArgsRecommended.Format(c.Name())
			lint.Run(&linter.RuleJSONArgsRecommended, c.Location(), msg)
		}
		args = withShell(d.image, args)
	}
	d.image.Config.Entrypoint = args
	if !d.cmd.IsSet {
		d.image.Config.Cmd = nil
	}
	return commitToHistory(&d.image, fmt.Sprintf("ENTRYPOINT %q", args), false, nil, d.epoch)
}

func dispatchHealthcheck(d *dispatchState, c *instructions.HealthCheckCommand, lint *linter.Linter) error {
	validateUsedOnce(c, &d.healthcheck, lint)
	d.image.Config.Healthcheck = &dockerspec.HealthcheckConfig{
		Test:          c.Health.Test,
		Interval:      c.Health.Interval,
		Timeout:       c.Health.Timeout,
		StartPeriod:   c.Health.StartPeriod,
		StartInterval: c.Health.StartInterval,
		Retries:       c.Health.Retries,
	}
	return commitToHistory(&d.image, fmt.Sprintf("HEALTHCHECK %q", d.image.Config.Healthcheck), false, nil, d.epoch)
}

func dispatchExpose(d *dispatchState, c *instructions.ExposeCommand, shlex *shell.Lex) error {
	ports := []string{}
	env := getEnv(d.state)
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

	return commitToHistory(&d.image, fmt.Sprintf("EXPOSE %v", ps), false, nil, d.epoch)
}

func dispatchUser(d *dispatchState, c *instructions.UserCommand, commit bool) error {
	d.state = d.state.User(c.User)
	d.image.Config.User = c.User
	if commit {
		return commitToHistory(&d.image, fmt.Sprintf("USER %v", c.User), false, nil, d.epoch)
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
	return commitToHistory(&d.image, fmt.Sprintf("VOLUME %v", c.Volumes), false, nil, d.epoch)
}

func dispatchStopSignal(d *dispatchState, c *instructions.StopSignalCommand) error {
	if _, err := signal.ParseSignal(c.Signal); err != nil {
		return err
	}
	d.image.Config.StopSignal = c.Signal
	return commitToHistory(&d.image, fmt.Sprintf("STOPSIGNAL %v", c.Signal), false, nil, d.epoch)
}

func dispatchShell(d *dispatchState, c *instructions.ShellCommand) error {
	d.image.Config.Shell = c.Shell
	return commitToHistory(&d.image, fmt.Sprintf("SHELL %v", c.Shell), false, nil, d.epoch)
}

func dispatchArg(d *dispatchState, c *instructions.ArgCommand, opt *dispatchOpt) error {
	commitStrs := make([]string, 0, len(c.Args))
	for _, arg := range c.Args {
		validateNoSecretKey("ARG", arg.Key, c.Location(), opt.lint)
		_, hasValue := opt.buildArgValues[arg.Key]
		hasDefault := arg.Value != nil

		skipArgInfo := false // skip the arg info if the arg is inherited from global scope
		if !hasDefault && !hasValue {
			if v, ok := opt.globalArgs.Get(arg.Key); ok {
				arg.Value = &v
				skipArgInfo = true
				hasDefault = false
			}
		}

		if hasValue {
			v := opt.buildArgValues[arg.Key]
			arg.Value = &v
		} else if hasDefault {
			env := getEnv(d.state)
			v, unmatched, err := opt.shlex.ProcessWord(*arg.Value, env)
			reportUnmatchedVariables(c, d.buildArgs, env, unmatched, opt)
			if err != nil {
				return err
			}
			arg.Value = &v
		}

		ai := argInfo{definition: arg, location: c.Location()}

		if arg.Value != nil {
			if _, ok := nonEnvArgs[arg.Key]; !ok {
				d.state = d.state.AddEnv(arg.Key, *arg.Value)
			}
			ai.value = *arg.Value
		}

		if !skipArgInfo {
			d.outline.allArgs[arg.Key] = ai
		}
		d.outline.usedArgs[arg.Key] = struct{}{}

		d.buildArgs = append(d.buildArgs, arg)

		commitStr := arg.Key
		if arg.Value != nil {
			commitStr += "=" + *arg.Value
		}
		commitStrs = append(commitStrs, commitStr)
	}
	return commitToHistory(&d.image, "ARG "+strings.Join(commitStrs, " "), false, nil, d.epoch)
}

func pathRelativeToWorkingDir(s llb.State, p string, platform ocispecs.Platform) (string, error) {
	dir, err := s.GetDir(context.TODO(), llb.Platform(platform))
	if err != nil {
		return "", err
	}

	p, err = system.CheckSystemDriveAndRemoveDriveLetter(p, platform.OS)
	if err != nil {
		return "", errors.Wrap(err, "removing drive letter")
	}

	if system.IsAbs(p, platform.OS) {
		return system.NormalizePath("/", p, platform.OS, true)
	}

	// add slashes for "" and "." paths
	// "" is treated as current directory and not necessariy root
	if p == "." || p == "" {
		p = "./"
	}
	return system.NormalizePath(dir, p, platform.OS, true)
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

func runCommandString(args []string, buildArgs []instructions.KeyValuePairOptional, env shell.EnvGetter) string {
	var tmpBuildEnv []string
	tmpIdx := map[string]int{}
	for _, arg := range buildArgs {
		v, ok := env.Get(arg.Key)
		if !ok {
			v = arg.ValueString()
		}
		if idx, ok := tmpIdx[arg.Key]; ok {
			tmpBuildEnv[idx] = arg.Key + "=" + v
		} else {
			tmpIdx[arg.Key] = len(tmpBuildEnv)
			tmpBuildEnv = append(tmpBuildEnv, arg.Key+"="+v)
		}
	}
	if len(tmpBuildEnv) > 0 {
		tmpBuildEnv = append([]string{fmt.Sprintf("|%d", len(tmpBuildEnv))}, tmpBuildEnv...)
	}

	return strings.Join(append(tmpBuildEnv, args...), " ")
}

func commitToHistory(img *dockerspec.DockerOCIImage, msg string, withLayer bool, st *llb.State, tm *time.Time) error {
	if st != nil {
		msg += " # buildkit"
	}

	img.History = append(img.History, ocispecs.History{
		CreatedBy:  msg,
		Comment:    historyComment,
		EmptyLayer: !withLayer,
		Created:    tm,
	})
	return nil
}

func allReachableStages(s *dispatchState) map[*dispatchState]struct{} {
	stages := make(map[*dispatchState]struct{})
	addReachableStages(s, stages)
	return stages
}

func addReachableStages(s *dispatchState, stages map[*dispatchState]struct{}) {
	if _, ok := stages[s]; ok {
		return
	}
	stages[s] = struct{}{}
	if s.base != nil {
		addReachableStages(s.base, stages)
	}
	for d := range s.deps {
		addReachableStages(d, stages)
	}
}

func validateCopySourcePath(src string, cfg *copyConfig) error {
	if cfg.ignoreMatcher == nil {
		return nil
	}
	cmd := "Copy"
	if cfg.isAddCommand {
		cmd = "Add"
	}

	ok, err := cfg.ignoreMatcher.MatchesOrParentMatches(src)
	if err != nil {
		return err
	}
	if ok {
		msg := linter.RuleCopyIgnoredFile.Format(cmd, src)
		cfg.opt.lint.Run(&linter.RuleCopyIgnoredFile, cfg.location, msg)
	}

	return nil
}

func validateCircularDependency(states []*dispatchState) error {
	var visit func(*dispatchState, []instructions.Command) []instructions.Command
	if states == nil {
		return nil
	}
	visited := make(map[*dispatchState]struct{})
	path := make(map[*dispatchState]struct{})

	visit = func(state *dispatchState, current []instructions.Command) []instructions.Command {
		_, ok := visited[state]
		if ok {
			return nil
		}
		visited[state] = struct{}{}
		path[state] = struct{}{}
		for dep, c := range state.deps {
			next := append(current, c)
			if _, ok := path[dep]; ok {
				return next
			}
			if c := visit(dep, next); c != nil {
				return c
			}
		}
		delete(path, state)
		return nil
	}
	for _, state := range states {
		if cmds := visit(state, nil); cmds != nil {
			err := errors.Errorf("circular dependency detected on stage: %s", state.stageName)
			for _, c := range cmds {
				err = parser.WithLocation(err, c.Location())
			}
			return err
		}
	}
	return nil
}

func normalizeContextPaths(paths map[string]struct{}) []string {
	// Avoid a useless allocation if the set of paths is empty.
	if len(paths) == 0 {
		return nil
	}

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

// filterPaths returns the local options required to filter an llb.Local
// to only the required paths.
func filterPaths(paths map[string]struct{}) []llb.LocalOption {
	if includePaths := normalizeContextPaths(paths); len(includePaths) > 0 {
		return []llb.LocalOption{llb.FollowPaths(includePaths)}
	}
	return nil
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

func withShell(img dockerspec.DockerOCIImage, args []string) []string {
	var shell []string
	if len(img.Config.Shell) > 0 {
		shell = append([]string{}, img.Config.Shell...)
	} else {
		shell = defaultShell(img.OS)
	}
	return append(shell, strings.Join(args, " "))
}

func autoDetectPlatform(img dockerspec.DockerOCIImage, target ocispecs.Platform, supported []ocispecs.Platform) ocispecs.Platform {
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

func uppercaseCmd(str string) string {
	p := strings.SplitN(str, " ", 2)
	p[0] = strings.ToUpper(p[0])
	return strings.Join(p, " ")
}

func processCmdEnv(shlex *shell.Lex, cmd string, env shell.EnvGetter) string {
	w, _, err := shlex.ProcessWord(cmd, env)
	if err != nil {
		return cmd
	}
	return w
}

func prefixCommand(ds *dispatchState, str string, prefixPlatform bool, platform *ocispecs.Platform, env shell.EnvGetter) string {
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
	if ds.cmdIsOnBuild {
		out += "ONBUILD "
	}
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
func platformFromEnv(env shell.EnvGetter) *ocispecs.Platform {
	var p ocispecs.Platform
	var set bool
	for _, key := range env.Keys() {
		switch key {
		case "TARGETPLATFORM":
			v, _ := env.Get(key)
			p, err := platforms.Parse(v)
			if err != nil {
				continue
			}
			return &p
		case "TARGETOS":
			p.OS, _ = env.Get(key)
			set = true
		case "TARGETARCH":
			p.Architecture, _ = env.Get(key)
			set = true
		case "TARGETVARIANT":
			p.Variant, _ = env.Get(key)
			set = true
		}
	}
	if !set {
		return nil
	}
	return &p
}

func location(sm *llb.SourceMap, locations []parser.Range) llb.ConstraintsOpt {
	loc := make([]*pb.Range, 0, len(locations))
	for _, l := range locations {
		loc = append(loc, &pb.Range{
			Start: &pb.Position{
				Line:      int32(l.Start.Line),
				Character: int32(l.Start.Character),
			},
			End: &pb.Position{
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

func isHTTPSource(src string) bool {
	if !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://") {
		return false
	}
	// https://github.com/ORG/REPO.git is a git source, not an http source
	if gitRef, gitErr := gitutil.ParseGitRef(src); gitRef != nil && gitErr == nil {
		return false
	}
	return true
}

func isEnabledForStage(stage string, value string) bool {
	if enabled, err := strconv.ParseBool(value); err == nil {
		return enabled
	}

	vv := strings.Split(value, ",")
	for _, v := range vv {
		if v == stage {
			return true
		}
	}
	return false
}

func isSelfConsistentCasing(s string) bool {
	return s == strings.ToLower(s) || s == strings.ToUpper(s)
}

func validateCaseMatch(name string, isMajorityLower bool, location []parser.Range, lint *linter.Linter) {
	var correctCasing string
	if isMajorityLower && strings.ToLower(name) != name {
		correctCasing = "lowercase"
	} else if !isMajorityLower && strings.ToUpper(name) != name {
		correctCasing = "uppercase"
	}
	if correctCasing != "" {
		msg := linter.RuleConsistentInstructionCasing.Format(name, correctCasing)
		lint.Run(&linter.RuleConsistentInstructionCasing, location, msg)
	}
}

func validateCommandCasing(stages []instructions.Stage, lint *linter.Linter) {
	var lowerCount, upperCount int
	for _, stage := range stages {
		if isSelfConsistentCasing(stage.OrigCmd) {
			if strings.ToLower(stage.OrigCmd) == stage.OrigCmd {
				lowerCount++
			} else {
				upperCount++
			}
		}
		for _, cmd := range stage.Commands {
			cmdName := cmd.Name()
			if isSelfConsistentCasing(cmdName) {
				if strings.ToLower(cmdName) == cmdName {
					lowerCount++
				} else {
					upperCount++
				}
			}
		}
	}

	isMajorityLower := lowerCount > upperCount
	for _, stage := range stages {
		// Here, we check both if the command is consistent per command (ie, "CMD" or "cmd", not "Cmd")
		// as well as ensuring that the casing is consistent throughout the dockerfile by comparing the
		// command to the casing of the majority of commands.
		validateCaseMatch(stage.OrigCmd, isMajorityLower, stage.Location, lint)
		for _, cmd := range stage.Commands {
			validateCaseMatch(cmd.Name(), isMajorityLower, cmd.Location(), lint)
		}
	}
}

var reservedStageNames = map[string]struct{}{
	"context": {},
	"scratch": {},
}

func validateStageNames(stages []instructions.Stage, lint *linter.Linter) {
	stageNames := make(map[string]struct{})
	for _, stage := range stages {
		if stage.Name != "" {
			if _, ok := reservedStageNames[stage.Name]; ok {
				msg := linter.RuleReservedStageName.Format(stage.Name)
				lint.Run(&linter.RuleReservedStageName, stage.Location, msg)
			}

			if _, ok := stageNames[stage.Name]; ok {
				msg := linter.RuleDuplicateStageName.Format(stage.Name)
				lint.Run(&linter.RuleDuplicateStageName, stage.Location, msg)
			}
			stageNames[stage.Name] = struct{}{}
		}
	}
}

func reportUnmatchedVariables(cmd instructions.Command, buildArgs []instructions.KeyValuePairOptional, env shell.EnvGetter, unmatched map[string]struct{}, opt *dispatchOpt) {
	if len(unmatched) == 0 {
		return
	}
	for _, buildArg := range buildArgs {
		delete(unmatched, buildArg.Key)
	}
	if len(unmatched) == 0 {
		return
	}
	options := env.Keys()
	for cmdVar := range unmatched {
		if _, nonEnvOk := nonEnvArgs[cmdVar]; nonEnvOk {
			continue
		}
		match, _ := suggest.Search(cmdVar, options, runtime.GOOS != "windows")
		msg := linter.RuleUndefinedVar.Format(cmdVar, match)
		opt.lint.Run(&linter.RuleUndefinedVar, cmd.Location(), msg)
	}
}

func mergeLocations(locations ...[]parser.Range) []parser.Range {
	allRanges := []parser.Range{}
	for _, ranges := range locations {
		allRanges = append(allRanges, ranges...)
	}
	if len(allRanges) == 0 {
		return []parser.Range{}
	}
	if len(allRanges) == 1 {
		return allRanges
	}

	sort.Slice(allRanges, func(i, j int) bool {
		return allRanges[i].Start.Line < allRanges[j].Start.Line
	})

	location := []parser.Range{}
	currentRange := allRanges[0]
	for _, r := range allRanges[1:] {
		if r.Start.Line <= currentRange.End.Line {
			currentRange.End.Line = max(currentRange.End.Line, r.End.Line)
		} else {
			location = append(location, currentRange)
			currentRange = r
		}
	}
	location = append(location, currentRange)
	return location
}

func toPBLocation(sourceIndex int, location []parser.Range) pb.Location {
	loc := make([]*pb.Range, 0, len(location))
	for _, l := range location {
		loc = append(loc, &pb.Range{
			Start: &pb.Position{
				Line:      int32(l.Start.Line),
				Character: int32(l.Start.Character),
			},
			End: &pb.Position{
				Line:      int32(l.End.Line),
				Character: int32(l.End.Character),
			},
		})
	}
	return pb.Location{
		SourceIndex: int32(sourceIndex),
		Ranges:      loc,
	}
}

func unusedFromArgsCheckKeys(env shell.EnvGetter, args map[string]argInfo) map[string]struct{} {
	matched := make(map[string]struct{})
	for _, arg := range args {
		matched[arg.definition.Key] = struct{}{}
	}
	for _, k := range env.Keys() {
		matched[k] = struct{}{}
	}
	return matched
}

func reportUnusedFromArgs(testArgKeys map[string]struct{}, unmatched map[string]struct{}, location []parser.Range, lint *linter.Linter) {
	var argKeys []string
	for arg := range testArgKeys {
		argKeys = append(argKeys, arg)
	}
	for arg := range unmatched {
		if _, ok := testArgKeys[arg]; ok {
			continue
		}
		suggest, _ := suggest.Search(arg, argKeys, true)
		msg := linter.RuleUndefinedArgInFrom.Format(arg, suggest)
		lint.Run(&linter.RuleUndefinedArgInFrom, location, msg)
	}
}

func reportRedundantTargetPlatform(platformVar string, nameMatch shell.ProcessWordResult, location []parser.Range, env shell.EnvGetter, lint *linter.Linter) {
	// Only match this rule if there was only one matched name.
	// It's psosible there were multiple args and that one of them expanded to an empty
	// string and we don't want to report a warning when that happens.
	if len(nameMatch.Matched) == 1 && len(nameMatch.Unmatched) == 0 {
		const targetPlatform = "TARGETPLATFORM"
		// If target platform is the only environment variable that was substituted and the result
		// matches the target platform exactly, we can infer that the input was ${TARGETPLATFORM} or
		// $TARGETPLATFORM.
		if _, ok := nameMatch.Matched[targetPlatform]; !ok {
			return
		}

		if result, _ := env.Get(targetPlatform); nameMatch.Result == result {
			msg := linter.RuleRedundantTargetPlatform.Format(platformVar)
			lint.Run(&linter.RuleRedundantTargetPlatform, location, msg)
		}
	}
}

func reportConstPlatformDisallowed(stageName string, nameMatch shell.ProcessWordResult, location []parser.Range, lint *linter.Linter) {
	if len(nameMatch.Matched) > 0 || len(nameMatch.Unmatched) > 0 {
		// Some substitution happened so the platform was not a constant.
		// Disable checking for this warning.
		return
	}

	// Attempt to parse the platform result. If this fails, then it will fail
	// later so just ignore.
	p, err := platforms.Parse(nameMatch.Result)
	if err != nil {
		return
	}

	// Check if the platform os or architecture is used in the stage name
	// at all. If it is, then disable this warning.
	if strings.Contains(stageName, p.OS) || strings.Contains(stageName, p.Architecture) {
		return
	}

	// Report the linter warning.
	msg := linter.RuleFromPlatformFlagConstDisallowed.Format(nameMatch.Result)
	lint.Run(&linter.RuleFromPlatformFlagConstDisallowed, location, msg)
}

type instructionTracker struct {
	Loc   []parser.Range
	IsSet bool
}

func (v *instructionTracker) MarkUsed(loc []parser.Range) {
	v.Loc = loc
	v.IsSet = true
}

func validateUsedOnce(c instructions.Command, loc *instructionTracker, lint *linter.Linter) {
	if loc.IsSet {
		msg := linter.RuleMultipleInstructionsDisallowed.Format(c.Name())
		// Report the location of the previous invocation because it is the one
		// that will be ignored.
		lint.Run(&linter.RuleMultipleInstructionsDisallowed, loc.Loc, msg)
	}
	loc.MarkUsed(c.Location())
}

func wrapSuggestAny(err error, keys map[string]struct{}, options []string) error {
	for k := range keys {
		var ok bool
		ok, err = suggest.WrapErrorMaybe(err, k, options, true)
		if ok {
			break
		}
	}
	return err
}

func validateBaseImagePlatform(name string, expected, actual ocispecs.Platform, location []parser.Range, lint *linter.Linter) {
	if expected.OS != actual.OS || expected.Architecture != actual.Architecture {
		expectedStr := platforms.Format(platforms.Normalize(expected))
		actualStr := platforms.Format(platforms.Normalize(actual))
		msg := linter.RuleInvalidBaseImagePlatform.Format(name, expectedStr, actualStr)
		lint.Run(&linter.RuleInvalidBaseImagePlatform, location, msg)
	}
}

func getSecretsRegex() (*regexp.Regexp, *regexp.Regexp) {
	// Check for either full value or first/last word.
	// Examples: api_key, DATABASE_PASSWORD, GITHUB_TOKEN, secret_MESSAGE, AUTH
	// Case insensitive.
	secretsRegexpOnce.Do(func() {
		secretTokens := []string{
			"apikey",
			"auth",
			"credential",
			"credentials",
			"key",
			"password",
			"pword",
			"passwd",
			"secret",
			"token",
		}
		pattern := `(?i)(?:_|^)(?:` + strings.Join(secretTokens, "|") + `)(?:_|$)`
		secretsRegexp = regexp.MustCompile(pattern)

		allowTokens := []string{
			"public",
		}
		allowPattern := `(?i)(?:_|^)(?:` + strings.Join(allowTokens, "|") + `)(?:_|$)`
		secretsAllowRegexp = regexp.MustCompile(allowPattern)
	})
	return secretsRegexp, secretsAllowRegexp
}

func validateNoSecretKey(instruction, key string, location []parser.Range, lint *linter.Linter) {
	deny, allow := getSecretsRegex()
	if deny.MatchString(key) && !allow.MatchString(key) {
		msg := linter.RuleSecretsUsedInArgOrEnv.Format(instruction, key)
		lint.Run(&linter.RuleSecretsUsedInArgOrEnv, location, msg)
	}
}

func validateBaseImagesWithDefaultArgs(stages []instructions.Stage, shlex *shell.Lex, env *llb.EnvList, argCmds []instructions.ArgCommand, lint *linter.Linter) {
	// Build the arguments as if no build options were given
	// and using only defaults.
	args, _, err := buildMetaArgs(env, shlex, argCmds, nil)
	if err != nil {
		// Abandon running the linter. We'll likely fail after this point
		// with the same error but we shouldn't error here inside
		// of the linting check.
		return
	}

	for _, st := range stages {
		nameMatch, err := shlex.ProcessWordWithMatches(st.BaseName, args)
		if err != nil {
			return
		}

		// Verify the image spec is potentially valid.
		if _, err := reference.ParseNormalizedNamed(nameMatch.Result); err != nil {
			msg := linter.RuleInvalidDefaultArgInFrom.Format(st.BaseName)
			lint.Run(&linter.RuleInvalidDefaultArgInFrom, st.Location, msg)
		}
	}
}

func buildMetaArgs(args *llb.EnvList, shlex *shell.Lex, argCommands []instructions.ArgCommand, buildArgs map[string]string) (*llb.EnvList, map[string]argInfo, error) {
	allArgs := make(map[string]argInfo)

	for _, cmd := range argCommands {
		for _, kp := range cmd.Args {
			info := argInfo{definition: kp, location: cmd.Location()}
			if v, ok := buildArgs[kp.Key]; !ok {
				if kp.Value != nil {
					result, err := shlex.ProcessWordWithMatches(*kp.Value, args)
					if err != nil {
						return nil, nil, parser.WithLocation(err, cmd.Location())
					}
					kp.Value = &result.Result
					info.deps = result.Matched
				}
			} else {
				kp.Value = &v
			}
			if kp.Value != nil {
				args = args.AddOrReplace(kp.Key, *kp.Value)
				info.value = *kp.Value
			}
			allArgs[kp.Key] = info
		}
	}
	return args, allArgs, nil
}

func rangeStartEnd(r []parser.Range) (int, int) {
	if len(r) == 0 {
		return 0, 0
	}
	start := math.MaxInt32
	end := 0
	for _, rng := range r {
		if rng.Start.Line < start {
			start = rng.Start.Line
		}
		if rng.End.Line > end {
			end = rng.End.Line
		}
	}
	return start, end
}

type emptyEnvs struct{}

func (emptyEnvs) Get(string) (string, bool) {
	return "", false
}

func (emptyEnvs) Keys() []string {
	return nil
}
