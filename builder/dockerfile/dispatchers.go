package dockerfile // import "github.com/docker/docker/builder/dockerfile"

// This file contains the dispatchers for each command. Note that
// `nullDispatch` is not actually a command, but support for commands we parse
// but do nothing with.
//
// See evaluator.go for a higher level discussion of the whole evaluator
// package.

import (
	"bytes"
	"fmt"
	"runtime"
	"sort"
	"strings"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/go-connections/nat"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/moby/buildkit/frontend/dockerfile/shell"
	"github.com/pkg/errors"
)

// ENV foo bar
//
// Sets the environment variable foo to bar, also makes interpolation
// in the dockerfile available from the next statement on via ${foo}.
//
func dispatchEnv(d dispatchRequest, c *instructions.EnvCommand) error {
	runConfig := d.state.runConfig
	commitMessage := bytes.NewBufferString("ENV")
	for _, e := range c.Env {
		name := e.Key
		newVar := e.String()

		commitMessage.WriteString(" " + newVar)
		gotOne := false
		for i, envVar := range runConfig.Env {
			envParts := strings.SplitN(envVar, "=", 2)
			compareFrom := envParts[0]
			if shell.EqualEnvKeys(compareFrom, name) {
				runConfig.Env[i] = newVar
				gotOne = true
				break
			}
		}
		if !gotOne {
			runConfig.Env = append(runConfig.Env, newVar)
		}
	}
	return d.builder.commit(d.state, commitMessage.String())
}

// MAINTAINER some text <maybe@an.email.address>
//
// Sets the maintainer metadata.
func dispatchMaintainer(d dispatchRequest, c *instructions.MaintainerCommand) error {

	d.state.maintainer = c.Maintainer
	return d.builder.commit(d.state, "MAINTAINER "+c.Maintainer)
}

// LABEL some json data describing the image
//
// Sets the Label variable foo to bar,
//
func dispatchLabel(d dispatchRequest, c *instructions.LabelCommand) error {
	if d.state.runConfig.Labels == nil {
		d.state.runConfig.Labels = make(map[string]string)
	}
	commitStr := "LABEL"
	for _, v := range c.Labels {
		d.state.runConfig.Labels[v.Key] = v.Value
		commitStr += " " + v.String()
	}
	return d.builder.commit(d.state, commitStr)
}

// ADD foo /path
//
// Add the file 'foo' to '/path'. Tarball and Remote URL (git, http) handling
// exist here. If you do not wish to have this automatic handling, use COPY.
//
func dispatchAdd(d dispatchRequest, c *instructions.AddCommand) error {
	downloader := newRemoteSourceDownloader(d.builder.Output, d.builder.Stdout)
	copier := copierFromDispatchRequest(d, downloader, nil)
	defer copier.Cleanup()

	copyInstruction, err := copier.createCopyInstruction(c.SourcesAndDest, "ADD")
	if err != nil {
		return err
	}
	copyInstruction.chownStr = c.Chown
	copyInstruction.allowLocalDecompression = true

	return d.builder.performCopy(d.state, copyInstruction)
}

// COPY foo /path
//
// Same as 'ADD' but without the tar and remote url handling.
//
func dispatchCopy(d dispatchRequest, c *instructions.CopyCommand) error {
	var im *imageMount
	var err error
	if c.From != "" {
		im, err = d.getImageMount(c.From)
		if err != nil {
			return errors.Wrapf(err, "invalid from flag value %s", c.From)
		}
	}
	copier := copierFromDispatchRequest(d, errOnSourceDownload, im)
	defer copier.Cleanup()
	copyInstruction, err := copier.createCopyInstruction(c.SourcesAndDest, "COPY")
	if err != nil {
		return err
	}
	copyInstruction.chownStr = c.Chown

	return d.builder.performCopy(d.state, copyInstruction)
}

func (d *dispatchRequest) getImageMount(imageRefOrID string) (*imageMount, error) {
	if imageRefOrID == "" {
		// TODO: this could return the source in the default case as well?
		return nil, nil
	}

	var localOnly bool
	stage, err := d.stages.get(imageRefOrID)
	if err != nil {
		return nil, err
	}
	if stage != nil {
		imageRefOrID = stage.Image
		localOnly = true
	}
	return d.builder.imageSources.Get(imageRefOrID, localOnly, d.state.operatingSystem)
}

// FROM [--platform=platform] imagename[:tag | @digest] [AS build-stage-name]
//
func initializeStage(d dispatchRequest, cmd *instructions.Stage) error {
	d.builder.imageProber.Reset()
	if err := system.ValidatePlatform(&cmd.Platform); err != nil {
		return err
	}
	image, err := d.getFromImage(d.shlex, cmd.BaseName, cmd.Platform.OS)
	if err != nil {
		return err
	}
	state := d.state
	if err := state.beginStage(cmd.Name, image); err != nil {
		return err
	}
	if len(state.runConfig.OnBuild) > 0 {
		triggers := state.runConfig.OnBuild
		state.runConfig.OnBuild = nil
		return dispatchTriggeredOnBuild(d, triggers)
	}
	return nil
}

func dispatchTriggeredOnBuild(d dispatchRequest, triggers []string) error {
	fmt.Fprintf(d.builder.Stdout, "# Executing %d build trigger", len(triggers))
	if len(triggers) > 1 {
		fmt.Fprint(d.builder.Stdout, "s")
	}
	fmt.Fprintln(d.builder.Stdout)
	for _, trigger := range triggers {
		d.state.updateRunConfig()
		ast, err := parser.Parse(strings.NewReader(trigger))
		if err != nil {
			return err
		}
		if len(ast.AST.Children) != 1 {
			return errors.New("onbuild trigger should be a single expression")
		}
		cmd, err := instructions.ParseCommand(ast.AST.Children[0])
		if err != nil {
			if instructions.IsUnknownInstruction(err) {
				buildsFailed.WithValues(metricsUnknownInstructionError).Inc()
			}
			return err
		}
		err = dispatch(d, cmd)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *dispatchRequest) getExpandedImageName(shlex *shell.Lex, name string) (string, error) {
	substitutionArgs := []string{}
	for key, value := range d.state.buildArgs.GetAllMeta() {
		substitutionArgs = append(substitutionArgs, key+"="+value)
	}

	name, err := shlex.ProcessWord(name, substitutionArgs)
	if err != nil {
		return "", err
	}
	return name, nil
}

// getOsFromFlagsAndStage calculates the operating system if we need to pull an image.
// stagePlatform contains the value supplied by optional `--platform=` on
// a current FROM statement. b.builder.options.Platform contains the operating
// system part of the optional flag passed in the API call (or CLI flag
// through `docker build --platform=...`). Precedence is for an explicit
// platform indication in the FROM statement.
func (d *dispatchRequest) getOsFromFlagsAndStage(stageOS string) string {
	switch {
	case stageOS != "":
		return stageOS
	case d.builder.options.Platform != "":
		// Note this is API "platform", but by this point, as the daemon is not
		// multi-arch aware yet, it is guaranteed to only hold the OS part here.
		return d.builder.options.Platform
	default:
		return runtime.GOOS
	}
}

func (d *dispatchRequest) getImageOrStage(name string, stageOS string) (builder.Image, error) {
	var localOnly bool
	if im, ok := d.stages.getByName(name); ok {
		name = im.Image
		localOnly = true
	}

	os := d.getOsFromFlagsAndStage(stageOS)

	// Windows cannot support a container with no base image unless it is LCOW.
	if name == api.NoBaseImageSpecifier {
		imageImage := &image.Image{}
		imageImage.OS = runtime.GOOS
		if runtime.GOOS == "windows" {
			switch os {
			case "windows", "":
				return nil, errors.New("Windows does not support FROM scratch")
			case "linux":
				if !system.LCOWSupported() {
					return nil, errors.New("Linux containers are not supported on this system")
				}
				imageImage.OS = "linux"
			default:
				return nil, errors.Errorf("operating system %q is not supported", os)
			}
		}
		return builder.Image(imageImage), nil
	}
	imageMount, err := d.builder.imageSources.Get(name, localOnly, os)
	if err != nil {
		return nil, err
	}
	return imageMount.Image(), nil
}
func (d *dispatchRequest) getFromImage(shlex *shell.Lex, name string, stageOS string) (builder.Image, error) {
	name, err := d.getExpandedImageName(shlex, name)
	if err != nil {
		return nil, err
	}
	return d.getImageOrStage(name, stageOS)
}

func dispatchOnbuild(d dispatchRequest, c *instructions.OnbuildCommand) error {

	d.state.runConfig.OnBuild = append(d.state.runConfig.OnBuild, c.Expression)
	return d.builder.commit(d.state, "ONBUILD "+c.Expression)
}

// WORKDIR /tmp
//
// Set the working directory for future RUN/CMD/etc statements.
//
func dispatchWorkdir(d dispatchRequest, c *instructions.WorkdirCommand) error {
	runConfig := d.state.runConfig
	var err error
	runConfig.WorkingDir, err = normalizeWorkdir(d.state.operatingSystem, runConfig.WorkingDir, c.Path)
	if err != nil {
		return err
	}

	// For performance reasons, we explicitly do a create/mkdir now
	// This avoids having an unnecessary expensive mount/unmount calls
	// (on Windows in particular) during each container create.
	// Prior to 1.13, the mkdir was deferred and not executed at this step.
	if d.builder.disableCommit {
		// Don't call back into the daemon if we're going through docker commit --change "WORKDIR /foo".
		// We've already updated the runConfig and that's enough.
		return nil
	}

	comment := "WORKDIR " + runConfig.WorkingDir
	runConfigWithCommentCmd := copyRunConfig(runConfig, withCmdCommentString(comment, d.state.operatingSystem))

	containerID, err := d.builder.probeAndCreate(d.state, runConfigWithCommentCmd)
	if err != nil || containerID == "" {
		return err
	}

	if err := d.builder.docker.ContainerCreateWorkdir(containerID); err != nil {
		return err
	}

	return d.builder.commitContainer(d.state, containerID, runConfigWithCommentCmd)
}

func resolveCmdLine(cmd instructions.ShellDependantCmdLine, runConfig *container.Config, os string) []string {
	result := cmd.CmdLine
	if cmd.PrependShell && result != nil {
		result = append(getShell(runConfig, os), result...)
	}
	return result
}

// RUN some command yo
//
// run a command and commit the image. Args are automatically prepended with
// the current SHELL which defaults to 'sh -c' under linux or 'cmd /S /C' under
// Windows, in the event there is only one argument The difference in processing:
//
// RUN echo hi          # sh -c echo hi       (Linux and LCOW)
// RUN echo hi          # cmd /S /C echo hi   (Windows)
// RUN [ "echo", "hi" ] # echo hi
//
func dispatchRun(d dispatchRequest, c *instructions.RunCommand) error {
	if !system.IsOSSupported(d.state.operatingSystem) {
		return system.ErrNotSupportedOperatingSystem
	}
	stateRunConfig := d.state.runConfig
	cmdFromArgs := resolveCmdLine(c.ShellDependantCmdLine, stateRunConfig, d.state.operatingSystem)
	buildArgs := d.state.buildArgs.FilterAllowed(stateRunConfig.Env)

	saveCmd := cmdFromArgs
	if len(buildArgs) > 0 {
		saveCmd = prependEnvOnCmd(d.state.buildArgs, buildArgs, cmdFromArgs)
	}

	runConfigForCacheProbe := copyRunConfig(stateRunConfig,
		withCmd(saveCmd),
		withEntrypointOverride(saveCmd, nil))
	if hit, err := d.builder.probeCache(d.state, runConfigForCacheProbe); err != nil || hit {
		return err
	}

	runConfig := copyRunConfig(stateRunConfig,
		withCmd(cmdFromArgs),
		withEnv(append(stateRunConfig.Env, buildArgs...)),
		withEntrypointOverride(saveCmd, strslice.StrSlice{""}))

	// set config as already being escaped, this prevents double escaping on windows
	runConfig.ArgsEscaped = true

	cID, err := d.builder.create(runConfig)
	if err != nil {
		return err
	}

	if err := d.builder.containerManager.Run(d.builder.clientCtx, cID, d.builder.Stdout, d.builder.Stderr); err != nil {
		if err, ok := err.(*statusCodeError); ok {
			// TODO: change error type, because jsonmessage.JSONError assumes HTTP
			msg := fmt.Sprintf(
				"The command '%s' returned a non-zero code: %d",
				strings.Join(runConfig.Cmd, " "), err.StatusCode())
			if err.Error() != "" {
				msg = fmt.Sprintf("%s: %s", msg, err.Error())
			}
			return &jsonmessage.JSONError{
				Message: msg,
				Code:    err.StatusCode(),
			}
		}
		return err
	}

	return d.builder.commitContainer(d.state, cID, runConfigForCacheProbe)
}

// Derive the command to use for probeCache() and to commit in this container.
// Note that we only do this if there are any build-time env vars.  Also, we
// use the special argument "|#" at the start of the args array. This will
// avoid conflicts with any RUN command since commands can not
// start with | (vertical bar). The "#" (number of build envs) is there to
// help ensure proper cache matches. We don't want a RUN command
// that starts with "foo=abc" to be considered part of a build-time env var.
//
// remove any unreferenced built-in args from the environment variables.
// These args are transparent so resulting image should be the same regardless
// of the value.
func prependEnvOnCmd(buildArgs *BuildArgs, buildArgVars []string, cmd strslice.StrSlice) strslice.StrSlice {
	var tmpBuildEnv []string
	for _, env := range buildArgVars {
		key := strings.SplitN(env, "=", 2)[0]
		if buildArgs.IsReferencedOrNotBuiltin(key) {
			tmpBuildEnv = append(tmpBuildEnv, env)
		}
	}

	sort.Strings(tmpBuildEnv)
	tmpEnv := append([]string{fmt.Sprintf("|%d", len(tmpBuildEnv))}, tmpBuildEnv...)
	return strslice.StrSlice(append(tmpEnv, cmd...))
}

// CMD foo
//
// Set the default command to run in the container (which may be empty).
// Argument handling is the same as RUN.
//
func dispatchCmd(d dispatchRequest, c *instructions.CmdCommand) error {
	runConfig := d.state.runConfig
	cmd := resolveCmdLine(c.ShellDependantCmdLine, runConfig, d.state.operatingSystem)
	runConfig.Cmd = cmd
	// set config as already being escaped, this prevents double escaping on windows
	runConfig.ArgsEscaped = true

	if err := d.builder.commit(d.state, fmt.Sprintf("CMD %q", cmd)); err != nil {
		return err
	}

	if len(c.ShellDependantCmdLine.CmdLine) != 0 {
		d.state.cmdSet = true
	}

	return nil
}

// HEALTHCHECK foo
//
// Set the default healthcheck command to run in the container (which may be empty).
// Argument handling is the same as RUN.
//
func dispatchHealthcheck(d dispatchRequest, c *instructions.HealthCheckCommand) error {
	runConfig := d.state.runConfig
	if runConfig.Healthcheck != nil {
		oldCmd := runConfig.Healthcheck.Test
		if len(oldCmd) > 0 && oldCmd[0] != "NONE" {
			fmt.Fprintf(d.builder.Stdout, "Note: overriding previous HEALTHCHECK: %v\n", oldCmd)
		}
	}
	runConfig.Healthcheck = c.Health
	return d.builder.commit(d.state, fmt.Sprintf("HEALTHCHECK %q", runConfig.Healthcheck))
}

// ENTRYPOINT /usr/sbin/nginx
//
// Set the entrypoint to /usr/sbin/nginx. Will accept the CMD as the arguments
// to /usr/sbin/nginx. Uses the default shell if not in JSON format.
//
// Handles command processing similar to CMD and RUN, only req.runConfig.Entrypoint
// is initialized at newBuilder time instead of through argument parsing.
//
func dispatchEntrypoint(d dispatchRequest, c *instructions.EntrypointCommand) error {
	runConfig := d.state.runConfig
	cmd := resolveCmdLine(c.ShellDependantCmdLine, runConfig, d.state.operatingSystem)
	runConfig.Entrypoint = cmd
	if !d.state.cmdSet {
		runConfig.Cmd = nil
	}

	return d.builder.commit(d.state, fmt.Sprintf("ENTRYPOINT %q", runConfig.Entrypoint))
}

// EXPOSE 6667/tcp 7000/tcp
//
// Expose ports for links and port mappings. This all ends up in
// req.runConfig.ExposedPorts for runconfig.
//
func dispatchExpose(d dispatchRequest, c *instructions.ExposeCommand, envs []string) error {
	// custom multi word expansion
	// expose $FOO with FOO="80 443" is expanded as EXPOSE [80,443]. This is the only command supporting word to words expansion
	// so the word processing has been de-generalized
	ports := []string{}
	for _, p := range c.Ports {
		ps, err := d.shlex.ProcessWords(p, envs)
		if err != nil {
			return err
		}
		ports = append(ports, ps...)
	}
	c.Ports = ports

	ps, _, err := nat.ParsePortSpecs(ports)
	if err != nil {
		return err
	}

	if d.state.runConfig.ExposedPorts == nil {
		d.state.runConfig.ExposedPorts = make(nat.PortSet)
	}
	for p := range ps {
		d.state.runConfig.ExposedPorts[p] = struct{}{}
	}

	return d.builder.commit(d.state, "EXPOSE "+strings.Join(c.Ports, " "))
}

// USER foo
//
// Set the user to 'foo' for future commands and when running the
// ENTRYPOINT/CMD at container run time.
//
func dispatchUser(d dispatchRequest, c *instructions.UserCommand) error {
	d.state.runConfig.User = c.User
	return d.builder.commit(d.state, fmt.Sprintf("USER %v", c.User))
}

// VOLUME /foo
//
// Expose the volume /foo for use. Will also accept the JSON array form.
//
func dispatchVolume(d dispatchRequest, c *instructions.VolumeCommand) error {
	if d.state.runConfig.Volumes == nil {
		d.state.runConfig.Volumes = map[string]struct{}{}
	}
	for _, v := range c.Volumes {
		if v == "" {
			return errors.New("VOLUME specified can not be an empty string")
		}
		d.state.runConfig.Volumes[v] = struct{}{}
	}
	return d.builder.commit(d.state, fmt.Sprintf("VOLUME %v", c.Volumes))
}

// STOPSIGNAL signal
//
// Set the signal that will be used to kill the container.
func dispatchStopSignal(d dispatchRequest, c *instructions.StopSignalCommand) error {

	_, err := signal.ParseSignal(c.Signal)
	if err != nil {
		return errdefs.InvalidParameter(err)
	}
	d.state.runConfig.StopSignal = c.Signal
	return d.builder.commit(d.state, fmt.Sprintf("STOPSIGNAL %v", c.Signal))
}

// ARG name[=value]
//
// Adds the variable foo to the trusted list of variables that can be passed
// to builder using the --build-arg flag for expansion/substitution or passing to 'run'.
// Dockerfile author may optionally set a default value of this variable.
func dispatchArg(d dispatchRequest, c *instructions.ArgCommand) error {

	commitStr := "ARG " + c.Key
	if c.Value != nil {
		commitStr += "=" + *c.Value
	}

	d.state.buildArgs.AddArg(c.Key, c.Value)
	return d.builder.commit(d.state, commitStr)
}

// SHELL powershell -command
//
// Set the non-default shell to use.
func dispatchShell(d dispatchRequest, c *instructions.ShellCommand) error {
	d.state.runConfig.Shell = c.Shell
	return d.builder.commit(d.state, fmt.Sprintf("SHELL %v", d.state.runConfig.Shell))
}
