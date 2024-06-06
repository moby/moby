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
	"time"

	"github.com/containerd/containerd/platforms"
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

func Dockefile2Outline(ctx context.Context, dt []byte, opt ConvertOpt) (*outline.Outline, error) {
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

func parseLintOptions(checkStr string) (*linter.Config, error) {
	checkStr = strings.TrimSpace(checkStr)
	if checkStr == "" {
		return &linter.Config{}, nil
	}

	parts := strings.SplitN(checkStr, ";", 2)
	var skipSet []string
	var errorOnWarn, skipAll bool
	for _, p := range parts {
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			return nil, errors.Errorf("invalid check option %q", p)
		}
		k = strings.TrimSpace(k)
		switch k {
		case "skip":
			v = strings.TrimSpace(v)
			if v == "all" {
				skipAll = true
			} else {
				skipSet = strings.Split(v, ",")
				for i, rule := range skipSet {
					skipSet[i] = strings.TrimSpace(rule)
				}
			}
		case "error":
			v, err := strconv.ParseBool(strings.TrimSpace(v))
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse check option %q", p)
			}
			errorOnWarn = v
		default:
			return nil, errors.Errorf("invalid check option %q", k)
		}
	}
	return &linter.Config{
		SkipRules:     skipSet,
		SkipAll:       skipAll,
		ReturnAsError: errorOnWarn,
	}, nil
}

func newRuleLinter(dt []byte, opt *ConvertOpt) (*linter.Linter, error) {
	var lintOptionStr string
	if opt.Client != nil && opt.Client.LinterConfig != nil {
		lintOptionStr = *opt.Client.LinterConfig
	} else {
		lintOptionStr, _, _, _ = parser.ParseDirective("check", dt)
	}
	lintConfig, err := parseLintOptions(lintOptionStr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse check options")
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

	platformOpt := buildPlatformOpt(&opt)

	optMetaArgs := getPlatformArgs(platformOpt)
	for i, arg := range optMetaArgs {
		optMetaArgs[i] = setKVValue(arg, opt.BuildArgs)
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
		if warning.URL == linter.RuleNoEmptyContinuations.URL {
			location := []parser.Range{*warning.Location}
			msg := linter.RuleNoEmptyContinuations.Format()
			lint.Run(&linter.RuleNoEmptyContinuations, location, msg)
		}
	}

	validateCommandCasing(dockerfile, lint)

	proxyEnv := proxyEnvFromBuildArgs(opt.BuildArgs)

	stages, metaArgs, err := instructions.Parse(dockerfile.AST, lint)
	if err != nil {
		return nil, err
	}
	validateStageNames(stages, lint)

	shlex := shell.NewLex(dockerfile.EscapeToken)
	outline := newOutlineCapture()

	for _, cmd := range metaArgs {
		for _, metaArg := range cmd.Args {
			info := argInfo{definition: metaArg, location: cmd.Location()}
			if v, ok := opt.BuildArgs[metaArg.Key]; !ok {
				if metaArg.Value != nil {
					result, err := shlex.ProcessWordWithMatches(*metaArg.Value, metaArgsToMap(optMetaArgs))
					if err != nil {
						return nil, parser.WithLocation(err, cmd.Location())
					}
					*metaArg.Value = result.Result
					info.deps = result.Matched
				}
			} else {
				metaArg.Value = &v
			}
			optMetaArgs = append(optMetaArgs, metaArg)
			if metaArg.Value != nil {
				info.value = *metaArg.Value
			}
			outline.allArgs[metaArg.Key] = info
		}
	}

	metaResolver := opt.MetaResolver
	if metaResolver == nil {
		metaResolver = imagemetaresolver.Default()
	}

	allDispatchStates := newDispatchStates()

	// set base state for every image
	for i, st := range stages {
		nameMatch, err := shlex.ProcessWordWithMatches(st.BaseName, metaArgsToMap(optMetaArgs))
		reportUnusedFromArgs(metaArgsKeys(optMetaArgs), nameMatch.Unmatched, st.Location, lint)
		used := nameMatch.Matched

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
			platMatch, err := shlex.ProcessWordWithMatches(v, metaArgsToMap(optMetaArgs))
			reportUnusedFromArgs(metaArgsKeys(optMetaArgs), platMatch.Unmatched, st.Location, lint)

			if err != nil {
				return nil, parser.WithLocation(errors.Wrapf(err, "failed to process arguments for platform %s", platMatch.Result), st.Location)
			}

			if platMatch.Result == "" {
				err := errors.Errorf("empty platform value from expression %s", v)
				err = parser.WithLocation(err, st.Location)
				err = wrapSuggestAny(err, platMatch.Unmatched, metaArgsKeys(optMetaArgs))
				return nil, err
			}

			p, err := platforms.Parse(platMatch.Result)
			if err != nil {
				err = parser.WithLocation(err, st.Location)
				err = wrapSuggestAny(err, platMatch.Unmatched, metaArgsKeys(optMetaArgs))
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
				ds.noinit = true
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
	allReachable := allReachableStages(target)

	baseCtx := ctx
	eg, ctx := errgroup.WithContext(ctx)
	for i, d := range allDispatchStates.states {
		_, reachable := allReachable[d]
		// resolve image config for every stage
		if d.base == nil && !d.noinit {
			if d.stage.BaseName == emptyImageName {
				d.state = llb.Scratch()
				d.image = emptyImage(platformOpt.targetPlatform)
				d.platform = &platformOpt.targetPlatform
				if d.unregistered {
					d.noinit = true
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
							d.noinit = true
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
							llb.WithCustomName(prefixCommand(d, "FROM "+d.stage.BaseName, opt.MultiPlatformRequested, platform, nil)),
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

	ctx = baseCtx
	buildContext := &mutableOutput{}
	ctxPaths := map[string]struct{}{}

	for _, d := range allDispatchStates.states {
		if _, ok := allReachable[d]; !ok || d.noinit {
			continue
		}
		d.init()

		// Ensure platform is set.
		if d.platform == nil {
			d.platform = &d.opt.targetPlatform
		}

		// make sure that PATH is always set
		if _, ok := shell.BuildEnvs(d.image.Config.Env)["PATH"]; !ok {
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
			allDispatchStates: allDispatchStates,
			metaArgs:          optMetaArgs,
			buildArgValues:    opt.BuildArgs,
			shlex:             shlex,
			buildContext:      llb.NewState(buildContext),
			proxyEnv:          proxyEnv,
			cacheIDNamespace:  opt.CacheIDNamespace,
			buildPlatforms:    platformOpt.buildPlatforms,
			targetPlatform:    platformOpt.targetPlatform,
			extraHosts:        opt.ExtraHosts,
			shmSize:           opt.ShmSize,
			ulimit:            opt.Ulimits,
			cgroupParent:      opt.CgroupParent,
			llbCaps:           opt.LLBCaps,
			sourceMap:         opt.SourceMap,
			lint:              lint,
		}

		if err = dispatchOnBuildTriggers(d, d.image.Config.OnBuild, opt); err != nil {
			return nil, parser.WithLocation(err, d.stage.Location)
		}
		d.image.Config.OnBuild = nil

		for _, cmd := range d.commands {
			if err := dispatch(d, cmd, opt); err != nil {
				return nil, parser.WithLocation(err, cmd.Location())
			}
		}
		d.opt = opt

		for p := range d.ctxPaths {
			ctxPaths[p] = struct{}{}
		}

		locals := []instructions.KeyValuePairOptional{}
		locals = append(locals, d.opt.metaArgs...)
		locals = append(locals, d.buildArgs...)
		for _, a := range locals {
			switch a.Key {
			case sbomScanStage:
				d.scanStage = isEnabledForStage(d.stageName, a.ValueString())
			case sbomScanContext:
				d.scanContext = isEnabledForStage(d.stageName, a.ValueString())
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
	for k, v := range opt.Labels {
		target.image.Config.Labels[k] = v
	}

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

func metaArgsToMap(metaArgs []instructions.KeyValuePairOptional) map[string]string {
	m := map[string]string{}

	for _, arg := range metaArgs {
		m[arg.Key] = arg.ValueString()
	}

	return m
}

func metaArgsKeys(metaArgs []instructions.KeyValuePairOptional) []string {
	s := make([]string, 0, len(metaArgs))
	for _, arg := range metaArgs {
		s = append(s, arg.Key)
	}
	return s
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
	allDispatchStates *dispatchStates
	metaArgs          []instructions.KeyValuePairOptional
	buildArgValues    map[string]string
	shlex             *shell.Lex
	buildContext      llb.State
	proxyEnv          *llb.ProxyEnv
	cacheIDNamespace  string
	targetPlatform    ocispecs.Platform
	buildPlatforms    []ocispecs.Platform
	extraHosts        []llb.HostIP
	shmSize           int64
	ulimit            []pb.Ulimit
	cgroupParent      string
	llbCaps           *apicaps.CapSet
	sourceMap         *llb.SourceMap
	lint              *linter.Linter
}

func dispatch(d *dispatchState, cmd command, opt dispatchOpt) error {
	var err error
	// ARG command value could be ignored, so defer handling the expansion error
	_, isArg := cmd.Command.(*instructions.ArgCommand)
	if ex, ok := cmd.Command.(instructions.SupportsSingleWordExpansion); ok && !isArg {
		err := ex.Expand(func(word string) (string, error) {
			env, err := d.state.Env(context.TODO())
			if err != nil {
				return "", err
			}

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
			env, err := d.state.Env(context.TODO())
			if err != nil {
				return "", err
			}
			lex := shell.NewLex('\\')
			lex.SkipProcessQuotes = true
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
		err = dispatchEnv(d, c)
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
		err = dispatchLabel(d, c)
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
		if len(cmd.sources) != 0 {
			src := cmd.sources[0]
			if !src.noinit {
				return errors.Errorf("cannot copy from stage %q, it needs to be defined before current stage %q", c.From, d.stageName)
			}
			l = src.state
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
	opt       dispatchOpt
	state     llb.State
	image     dockerspec.DockerOCIImage
	platform  *ocispecs.Platform
	stage     instructions.Stage
	base      *dispatchState
	baseImg   *dockerspec.DockerOCIImage // immutable, unlike image
	noinit    bool
	deps      map[*dispatchState]instructions.Command
	buildArgs []instructions.KeyValuePairOptional
	commands  []command
	// ctxPaths marks the paths this dispatchState uses from the build context.
	ctxPaths map[string]struct{}
	// paths marks the paths that are used by this dispatchState.
	paths          map[string]struct{}
	ignoreCache    bool
	unregistered   bool
	stageName      string
	cmdIndex       int
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
	ds.noinit = true

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
	return commitToHistory(&d.image, "RUN "+runCommandString(args, d.buildArgs, shell.BuildEnvs(env)), true, &d.state, d.epoch)
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
			env, err := d.state.Env(context.TODO())
			if err != nil {
				return err
			}
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

	var mode *os.FileMode
	if cfg.chmod != "" {
		p, err := strconv.ParseUint(cfg.chmod, 8, 32)
		if err == nil {
			perm := os.FileMode(p)
			mode = &perm
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
			return errors.New("checksum can't be specified for non-HTTP sources")
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

	env, err := d.state.Env(context.TODO())
	if err != nil {
		return err
	}

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
	opt             dispatchOpt
}

func dispatchMaintainer(d *dispatchState, c *instructions.MaintainerCommand) error {
	d.image.Author = c.Maintainer
	return commitToHistory(&d.image, fmt.Sprintf("MAINTAINER %v", c.Maintainer), false, nil, d.epoch)
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
		_, hasValue := opt.buildArgValues[arg.Key]
		hasDefault := arg.Value != nil

		skipArgInfo := false // skip the arg info if the arg is inherited from global scope
		if !hasDefault && !hasValue {
			for _, ma := range opt.metaArgs {
				if ma.Key == arg.Key {
					arg.Value = ma.Value
					skipArgInfo = true
					hasDefault = false
				}
			}
		}

		if hasValue {
			v := opt.buildArgValues[arg.Key]
			arg.Value = &v
		} else if hasDefault {
			env, err := d.state.Env(context.TODO())
			if err != nil {
				return err
			}
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

	if len(p) == 0 {
		return dir, nil
	}
	p, err = system.CheckSystemDriveAndRemoveDriveLetter(p, platform.OS)
	if err != nil {
		return "", errors.Wrap(err, "removing drive letter")
	}

	if system.IsAbs(p, platform.OS) {
		return system.NormalizePath("/", p, platform.OS, true)
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

func processCmdEnv(shlex *shell.Lex, cmd string, env []string) string {
	w, _, err := shlex.ProcessWord(cmd, env)
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

func isHTTPSource(src string) bool {
	return strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://")
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

func validateCommandCasing(dockerfile *parser.Result, lint *linter.Linter) {
	var lowerCount, upperCount int
	for _, node := range dockerfile.AST.Children {
		if isSelfConsistentCasing(node.Value) {
			if strings.ToLower(node.Value) == node.Value {
				lowerCount++
			} else {
				upperCount++
			}
		}
	}

	isMajorityLower := lowerCount > upperCount
	for _, node := range dockerfile.AST.Children {
		// Here, we check both if the command is consistent per command (ie, "CMD" or "cmd", not "Cmd")
		// as well as ensuring that the casing is consistent throughout the dockerfile by comparing the
		// command to the casing of the majority of commands.
		if !isSelfConsistentCasing(node.Value) {
			msg := linter.RuleSelfConsistentCommandCasing.Format(node.Value)
			lint.Run(&linter.RuleSelfConsistentCommandCasing, node.Location(), msg)
		} else {
			var msg string
			var needsLintWarn bool
			if isMajorityLower && strings.ToUpper(node.Value) == node.Value {
				msg = linter.RuleFileConsistentCommandCasing.Format(node.Value, "lowercase")
				needsLintWarn = true
			} else if !isMajorityLower && strings.ToLower(node.Value) == node.Value {
				msg = linter.RuleFileConsistentCommandCasing.Format(node.Value, "uppercase")
				needsLintWarn = true
			}
			if needsLintWarn {
				lint.Run(&linter.RuleFileConsistentCommandCasing, node.Location(), msg)
			}
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

func reportUnmatchedVariables(cmd instructions.Command, buildArgs []instructions.KeyValuePairOptional, env []string, unmatched map[string]struct{}, opt *dispatchOpt) {
	if len(unmatched) == 0 {
		return
	}
	for _, buildArg := range buildArgs {
		delete(unmatched, buildArg.Key)
	}
	if len(unmatched) == 0 {
		return
	}
	options := metaArgsKeys(opt.metaArgs)
	for _, envVar := range env {
		key, _ := parseKeyValue(envVar)
		options = append(options, key)
	}
	for cmdVar := range unmatched {
		if _, nonEnvOk := nonEnvArgs[cmdVar]; nonEnvOk {
			continue
		}
		match, _ := suggest.Search(cmdVar, options, true)
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
	return pb.Location{
		SourceIndex: int32(sourceIndex),
		Ranges:      loc,
	}
}

func reportUnusedFromArgs(values []string, unmatched map[string]struct{}, location []parser.Range, lint *linter.Linter) {
	for arg := range unmatched {
		suggest, _ := suggest.Search(arg, values, true)
		msg := linter.RuleUndeclaredArgInFrom.Format(arg, suggest)
		lint.Run(&linter.RuleUndeclaredArgInFrom, location, msg)
	}
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
