package dockerfile

// This file contains the dispatchers for each command. Note that
// `nullDispatch` is not actually a command, but support for commands we parse
// but do nothing with.
//
// See evaluator.go for a higher level discussion of the whole evaluator
// package.

import (
	"fmt"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"bytes"
	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
)

// ENV foo bar
//
// Sets the environment variable foo to bar, also makes interpolation
// in the dockerfile available from the next statement on via ${foo}.
//
func env(req dispatchRequest) error {
	if len(req.args) == 0 {
		return errAtLeastOneArgument("ENV")
	}

	if len(req.args)%2 != 0 {
		// should never get here, but just in case
		return errTooManyArguments("ENV")
	}

	if err := req.flags.Parse(); err != nil {
		return err
	}

	commitMessage := bytes.NewBufferString("ENV")

	for j := 0; j < len(req.args); j += 2 {
		if len(req.args[j]) == 0 {
			return errBlankCommandNames("ENV")
		}
		name := req.args[j]
		value := req.args[j+1]
		newVar := name + "=" + value
		commitMessage.WriteString(" " + newVar)

		gotOne := false
		for i, envVar := range req.runConfig.Env {
			envParts := strings.SplitN(envVar, "=", 2)
			compareFrom := envParts[0]
			if equalEnvKeys(compareFrom, name) {
				req.runConfig.Env[i] = newVar
				gotOne = true
				break
			}
		}
		if !gotOne {
			req.runConfig.Env = append(req.runConfig.Env, newVar)
		}
	}

	return req.builder.commit(commitMessage.String())
}

// MAINTAINER some text <maybe@an.email.address>
//
// Sets the maintainer metadata.
func maintainer(req dispatchRequest) error {
	if len(req.args) != 1 {
		return errExactlyOneArgument("MAINTAINER")
	}

	if err := req.flags.Parse(); err != nil {
		return err
	}

	maintainer := req.args[0]
	req.builder.maintainer = maintainer
	return req.builder.commit("MAINTAINER " + maintainer)
}

// LABEL some json data describing the image
//
// Sets the Label variable foo to bar,
//
func label(req dispatchRequest) error {
	if len(req.args) == 0 {
		return errAtLeastOneArgument("LABEL")
	}
	if len(req.args)%2 != 0 {
		// should never get here, but just in case
		return errTooManyArguments("LABEL")
	}

	if err := req.flags.Parse(); err != nil {
		return err
	}

	commitStr := "LABEL"

	if req.runConfig.Labels == nil {
		req.runConfig.Labels = map[string]string{}
	}

	for j := 0; j < len(req.args); j++ {
		// name  ==> req.args[j]
		// value ==> req.args[j+1]

		if len(req.args[j]) == 0 {
			return errBlankCommandNames("LABEL")
		}

		newVar := req.args[j] + "=" + req.args[j+1] + ""
		commitStr += " " + newVar

		req.runConfig.Labels[req.args[j]] = req.args[j+1]
		j++
	}
	return req.builder.commit(commitStr)
}

// ADD foo /path
//
// Add the file 'foo' to '/path'. Tarball and Remote URL (git, http) handling
// exist here. If you do not wish to have this automatic handling, use COPY.
//
func add(req dispatchRequest) error {
	if len(req.args) < 2 {
		return errAtLeastTwoArguments("ADD")
	}

	if err := req.flags.Parse(); err != nil {
		return err
	}

	return req.builder.runContextCommand(req.args, true, true, "ADD", nil)
}

// COPY foo /path
//
// Same as 'ADD' but without the tar and remote url handling.
//
func dispatchCopy(req dispatchRequest) error {
	if len(req.args) < 2 {
		return errAtLeastTwoArguments("COPY")
	}

	flFrom := req.flags.AddString("from", "")

	if err := req.flags.Parse(); err != nil {
		return err
	}

	var im *imageMount
	if flFrom.IsUsed() {
		var err error
		im, err = req.builder.imageContexts.get(flFrom.Value)
		if err != nil {
			return err
		}
	}

	return req.builder.runContextCommand(req.args, false, false, "COPY", im)
}

// FROM imagename[:tag | @digest] [AS build-stage-name]
//
func from(req dispatchRequest) error {
	ctxName, err := parseBuildStageName(req.args)
	if err != nil {
		return err
	}

	if err := req.flags.Parse(); err != nil {
		return err
	}

	req.builder.resetImageCache()
	if _, err := req.builder.imageContexts.add(ctxName); err != nil {
		return err
	}

	image, err := req.builder.getFromImage(req.shlex, req.args[0])
	if err != nil {
		return err
	}
	if image != nil {
		req.builder.imageContexts.update(image.ImageID(), image.RunConfig())
	}
	req.builder.from = image

	req.builder.buildArgs.ResetAllowed()
	return req.builder.processImageFrom(image)
}

func parseBuildStageName(args []string) (string, error) {
	stageName := ""
	switch {
	case len(args) == 3 && strings.EqualFold(args[1], "as"):
		stageName = strings.ToLower(args[2])
		if ok, _ := regexp.MatchString("^[a-z][a-z0-9-_\\.]*$", stageName); !ok {
			return "", errors.Errorf("invalid name for build stage: %q, name can't start with a number or contain symbols", stageName)
		}
	case len(args) != 1:
		return "", errors.New("FROM requires either one or three arguments")
	}

	return stageName, nil
}

func (b *Builder) getFromImage(shlex *ShellLex, name string) (builder.Image, error) {
	substitutionArgs := []string{}
	for key, value := range b.buildArgs.GetAllMeta() {
		substitutionArgs = append(substitutionArgs, key+"="+value)
	}

	name, err := shlex.ProcessWord(name, substitutionArgs)
	if err != nil {
		return nil, err
	}

	if im, ok := b.imageContexts.byName[name]; ok {
		if len(im.ImageID()) > 0 {
			return im, nil
		}
		// FROM scratch does not have an ImageID
		return nil, nil
	}

	// Windows cannot support a container with no base image.
	if name == api.NoBaseImageSpecifier {
		if runtime.GOOS == "windows" {
			return nil, errors.New("Windows does not support FROM scratch")
		}
		b.image = ""
		b.noBaseImage = true
		return nil, nil
	}
	return pullOrGetImage(b, name)
}

// ONBUILD RUN echo yo
//
// ONBUILD triggers run when the image is used in a FROM statement.
//
// ONBUILD handling has a lot of special-case functionality, the heading in
// evaluator.go and comments around dispatch() in the same file explain the
// special cases. search for 'OnBuild' in internals.go for additional special
// cases.
//
func onbuild(req dispatchRequest) error {
	if len(req.args) == 0 {
		return errAtLeastOneArgument("ONBUILD")
	}

	if err := req.flags.Parse(); err != nil {
		return err
	}

	triggerInstruction := strings.ToUpper(strings.TrimSpace(req.args[0]))
	switch triggerInstruction {
	case "ONBUILD":
		return errors.New("Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed")
	case "MAINTAINER", "FROM":
		return fmt.Errorf("%s isn't allowed as an ONBUILD trigger", triggerInstruction)
	}

	original := regexp.MustCompile(`(?i)^\s*ONBUILD\s*`).ReplaceAllString(req.original, "")
	req.runConfig.OnBuild = append(req.runConfig.OnBuild, original)
	return req.builder.commit("ONBUILD " + original)
}

// WORKDIR /tmp
//
// Set the working directory for future RUN/CMD/etc statements.
//
func workdir(req dispatchRequest) error {
	if len(req.args) != 1 {
		return errExactlyOneArgument("WORKDIR")
	}

	err := req.flags.Parse()
	if err != nil {
		return err
	}

	// This is from the Dockerfile and will not necessarily be in platform
	// specific semantics, hence ensure it is converted.
	req.runConfig.WorkingDir, err = normaliseWorkdir(req.runConfig.WorkingDir, req.args[0])
	if err != nil {
		return err
	}

	// For performance reasons, we explicitly do a create/mkdir now
	// This avoids having an unnecessary expensive mount/unmount calls
	// (on Windows in particular) during each container create.
	// Prior to 1.13, the mkdir was deferred and not executed at this step.
	if req.builder.disableCommit {
		// Don't call back into the daemon if we're going through docker commit --change "WORKDIR /foo".
		// We've already updated the runConfig and that's enough.
		return nil
	}

	cmd := req.runConfig.Cmd
	comment := "WORKDIR " + req.runConfig.WorkingDir
	// reset the command for cache detection
	req.runConfig.Cmd = strslice.StrSlice(append(getShell(req.runConfig), "#(nop) "+comment))
	defer func(cmd strslice.StrSlice) { req.runConfig.Cmd = cmd }(cmd)

	// TODO: this should pass a copy of runConfig
	if hit, err := req.builder.probeCache(req.builder.image, req.runConfig); err != nil || hit {
		return err
	}

	req.runConfig.Image = req.builder.image
	container, err := req.builder.docker.ContainerCreate(types.ContainerCreateConfig{
		Config: req.runConfig,
		// Set a log config to override any default value set on the daemon
		HostConfig: &container.HostConfig{LogConfig: defaultLogConfig},
	})
	if err != nil {
		return err
	}
	req.builder.tmpContainers[container.ID] = struct{}{}
	if err := req.builder.docker.ContainerCreateWorkdir(container.ID); err != nil {
		return err
	}

	return req.builder.commitContainer(container.ID, copyRunConfig(req.runConfig, withCmd(cmd)))
}

// RUN some command yo
//
// run a command and commit the image. Args are automatically prepended with
// the current SHELL which defaults to 'sh -c' under linux or 'cmd /S /C' under
// Windows, in the event there is only one argument The difference in processing:
//
// RUN echo hi          # sh -c echo hi       (Linux)
// RUN echo hi          # cmd /S /C echo hi   (Windows)
// RUN [ "echo", "hi" ] # echo hi
//
func run(req dispatchRequest) error {
	if !req.builder.hasFromImage() {
		return errors.New("Please provide a source image with `from` prior to run")
	}

	if err := req.flags.Parse(); err != nil {
		return err
	}

	args := handleJSONArgs(req.args, req.attributes)

	if !req.attributes["json"] {
		args = append(getShell(req.runConfig), args...)
	}
	config := &container.Config{
		Cmd:   strslice.StrSlice(args),
		Image: req.builder.image,
	}

	// stash the cmd
	cmd := req.runConfig.Cmd
	if len(req.runConfig.Entrypoint) == 0 && len(req.runConfig.Cmd) == 0 {
		req.runConfig.Cmd = config.Cmd
	}

	// stash the config environment
	env := req.runConfig.Env

	defer func(cmd strslice.StrSlice) { req.runConfig.Cmd = cmd }(cmd)
	defer func(env []string) { req.runConfig.Env = env }(env)

	cmdBuildEnv := req.builder.buildArgsWithoutConfigEnv()

	// derive the command to use for probeCache() and to commit in this container.
	// Note that we only do this if there are any build-time env vars.  Also, we
	// use the special argument "|#" at the start of the args array. This will
	// avoid conflicts with any RUN command since commands can not
	// start with | (vertical bar). The "#" (number of build envs) is there to
	// help ensure proper cache matches. We don't want a RUN command
	// that starts with "foo=abc" to be considered part of a build-time env var.
	saveCmd := config.Cmd
	if len(cmdBuildEnv) > 0 {
		sort.Strings(cmdBuildEnv)
		tmpEnv := append([]string{fmt.Sprintf("|%d", len(cmdBuildEnv))}, cmdBuildEnv...)
		saveCmd = strslice.StrSlice(append(tmpEnv, saveCmd...))
	}

	req.runConfig.Cmd = saveCmd
	hit, err := req.builder.probeCache(req.builder.image, req.runConfig)
	if err != nil || hit {
		return err
	}

	// set Cmd manually, this is special case only for Dockerfiles
	req.runConfig.Cmd = config.Cmd
	// set build-time environment for 'run'.
	req.runConfig.Env = append(req.runConfig.Env, cmdBuildEnv...)
	// set config as already being escaped, this prevents double escaping on windows
	req.runConfig.ArgsEscaped = true

	logrus.Debugf("[BUILDER] Command to be executed: %v", req.runConfig.Cmd)

	// TODO: this was previously in b.create(), why is it necessary?
	req.builder.runConfig.Image = req.builder.image

	// TODO: should pass a copy of runConfig
	cID, err := req.builder.create(req.runConfig)
	if err != nil {
		return err
	}

	if err := req.builder.run(cID); err != nil {
		return err
	}

	// FIXME: this is duplicated with the defer above in this function (i think?)
	// revert to original config environment and set the command string to
	// have the build-time env vars in it (if any) so that future cache look-ups
	// properly match it.
	req.runConfig.Env = env

	// remove builtinAllowedBuildArgs (see: builder.go)  from the saveCmd
	// these args are transparent so resulting image should be the same regardless of the value
	if len(cmdBuildEnv) > 0 {
		saveCmd = config.Cmd
		var tmpBuildEnv []string
		for _, env := range cmdBuildEnv {
			key := strings.SplitN(env, "=", 2)[0]
			if !req.builder.buildArgs.IsUnreferencedBuiltin(key) {
				tmpBuildEnv = append(tmpBuildEnv, env)
			}
		}
		sort.Strings(tmpBuildEnv)
		tmpEnv := append([]string{fmt.Sprintf("|%d", len(tmpBuildEnv))}, tmpBuildEnv...)
		saveCmd = strslice.StrSlice(append(tmpEnv, saveCmd...))
	}
	req.runConfig.Cmd = saveCmd
	return req.builder.commitContainer(cID, copyRunConfig(req.runConfig, withCmd(cmd)))
}

// CMD foo
//
// Set the default command to run in the container (which may be empty).
// Argument handling is the same as RUN.
//
func cmd(req dispatchRequest) error {
	if err := req.flags.Parse(); err != nil {
		return err
	}

	cmdSlice := handleJSONArgs(req.args, req.attributes)

	if !req.attributes["json"] {
		cmdSlice = append(getShell(req.runConfig), cmdSlice...)
	}

	req.runConfig.Cmd = strslice.StrSlice(cmdSlice)
	// set config as already being escaped, this prevents double escaping on windows
	req.runConfig.ArgsEscaped = true

	if err := req.builder.commit(fmt.Sprintf("CMD %q", cmdSlice)); err != nil {
		return err
	}

	if len(req.args) != 0 {
		req.builder.cmdSet = true
	}

	return nil
}

// parseOptInterval(flag) is the duration of flag.Value, or 0 if
// empty. An error is reported if the value is given and less than minimum duration.
func parseOptInterval(f *Flag) (time.Duration, error) {
	s := f.Value
	if s == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	if d < time.Duration(container.MinimumDuration) {
		return 0, fmt.Errorf("Interval %#v cannot be less than %s", f.name, container.MinimumDuration)
	}
	return d, nil
}

// HEALTHCHECK foo
//
// Set the default healthcheck command to run in the container (which may be empty).
// Argument handling is the same as RUN.
//
func healthcheck(req dispatchRequest) error {
	if len(req.args) == 0 {
		return errAtLeastOneArgument("HEALTHCHECK")
	}
	typ := strings.ToUpper(req.args[0])
	args := req.args[1:]
	if typ == "NONE" {
		if len(args) != 0 {
			return errors.New("HEALTHCHECK NONE takes no arguments")
		}
		test := strslice.StrSlice{typ}
		req.runConfig.Healthcheck = &container.HealthConfig{
			Test: test,
		}
	} else {
		if req.runConfig.Healthcheck != nil {
			oldCmd := req.runConfig.Healthcheck.Test
			if len(oldCmd) > 0 && oldCmd[0] != "NONE" {
				fmt.Fprintf(req.builder.Stdout, "Note: overriding previous HEALTHCHECK: %v\n", oldCmd)
			}
		}

		healthcheck := container.HealthConfig{}

		flInterval := req.flags.AddString("interval", "")
		flTimeout := req.flags.AddString("timeout", "")
		flStartPeriod := req.flags.AddString("start-period", "")
		flRetries := req.flags.AddString("retries", "")

		if err := req.flags.Parse(); err != nil {
			return err
		}

		switch typ {
		case "CMD":
			cmdSlice := handleJSONArgs(args, req.attributes)
			if len(cmdSlice) == 0 {
				return errors.New("Missing command after HEALTHCHECK CMD")
			}

			if !req.attributes["json"] {
				typ = "CMD-SHELL"
			}

			healthcheck.Test = strslice.StrSlice(append([]string{typ}, cmdSlice...))
		default:
			return fmt.Errorf("Unknown type %#v in HEALTHCHECK (try CMD)", typ)
		}

		interval, err := parseOptInterval(flInterval)
		if err != nil {
			return err
		}
		healthcheck.Interval = interval

		timeout, err := parseOptInterval(flTimeout)
		if err != nil {
			return err
		}
		healthcheck.Timeout = timeout

		startPeriod, err := parseOptInterval(flStartPeriod)
		if err != nil {
			return err
		}
		healthcheck.StartPeriod = startPeriod

		if flRetries.Value != "" {
			retries, err := strconv.ParseInt(flRetries.Value, 10, 32)
			if err != nil {
				return err
			}
			if retries < 1 {
				return fmt.Errorf("--retries must be at least 1 (not %d)", retries)
			}
			healthcheck.Retries = int(retries)
		} else {
			healthcheck.Retries = 0
		}

		req.runConfig.Healthcheck = &healthcheck
	}

	return req.builder.commit(fmt.Sprintf("HEALTHCHECK %q", req.runConfig.Healthcheck))
}

// ENTRYPOINT /usr/sbin/nginx
//
// Set the entrypoint to /usr/sbin/nginx. Will accept the CMD as the arguments
// to /usr/sbin/nginx. Uses the default shell if not in JSON format.
//
// Handles command processing similar to CMD and RUN, only req.runConfig.Entrypoint
// is initialized at NewBuilder time instead of through argument parsing.
//
func entrypoint(req dispatchRequest) error {
	if err := req.flags.Parse(); err != nil {
		return err
	}

	parsed := handleJSONArgs(req.args, req.attributes)

	switch {
	case req.attributes["json"]:
		// ENTRYPOINT ["echo", "hi"]
		req.runConfig.Entrypoint = strslice.StrSlice(parsed)
	case len(parsed) == 0:
		// ENTRYPOINT []
		req.runConfig.Entrypoint = nil
	default:
		// ENTRYPOINT echo hi
		req.runConfig.Entrypoint = strslice.StrSlice(append(getShell(req.runConfig), parsed[0]))
	}

	// when setting the entrypoint if a CMD was not explicitly set then
	// set the command to nil
	if !req.builder.cmdSet {
		req.runConfig.Cmd = nil
	}

	return req.builder.commit(fmt.Sprintf("ENTRYPOINT %q", req.runConfig.Entrypoint))
}

// EXPOSE 6667/tcp 7000/tcp
//
// Expose ports for links and port mappings. This all ends up in
// req.runConfig.ExposedPorts for runconfig.
//
func expose(req dispatchRequest) error {
	portsTab := req.args

	if len(req.args) == 0 {
		return errAtLeastOneArgument("EXPOSE")
	}

	if err := req.flags.Parse(); err != nil {
		return err
	}

	if req.runConfig.ExposedPorts == nil {
		req.runConfig.ExposedPorts = make(nat.PortSet)
	}

	ports, _, err := nat.ParsePortSpecs(portsTab)
	if err != nil {
		return err
	}

	// instead of using ports directly, we build a list of ports and sort it so
	// the order is consistent. This prevents cache burst where map ordering
	// changes between builds
	portList := make([]string, len(ports))
	var i int
	for port := range ports {
		if _, exists := req.runConfig.ExposedPorts[port]; !exists {
			req.runConfig.ExposedPorts[port] = struct{}{}
		}
		portList[i] = string(port)
		i++
	}
	sort.Strings(portList)
	return req.builder.commit("EXPOSE " + strings.Join(portList, " "))
}

// USER foo
//
// Set the user to 'foo' for future commands and when running the
// ENTRYPOINT/CMD at container run time.
//
func user(req dispatchRequest) error {
	if len(req.args) != 1 {
		return errExactlyOneArgument("USER")
	}

	if err := req.flags.Parse(); err != nil {
		return err
	}

	req.runConfig.User = req.args[0]
	return req.builder.commit(fmt.Sprintf("USER %v", req.args))
}

// VOLUME /foo
//
// Expose the volume /foo for use. Will also accept the JSON array form.
//
func volume(req dispatchRequest) error {
	if len(req.args) == 0 {
		return errAtLeastOneArgument("VOLUME")
	}

	if err := req.flags.Parse(); err != nil {
		return err
	}

	if req.runConfig.Volumes == nil {
		req.runConfig.Volumes = map[string]struct{}{}
	}
	for _, v := range req.args {
		v = strings.TrimSpace(v)
		if v == "" {
			return errors.New("VOLUME specified can not be an empty string")
		}
		req.runConfig.Volumes[v] = struct{}{}
	}
	return req.builder.commit(fmt.Sprintf("VOLUME %v", req.args))
}

// STOPSIGNAL signal
//
// Set the signal that will be used to kill the container.
func stopSignal(req dispatchRequest) error {
	if len(req.args) != 1 {
		return errExactlyOneArgument("STOPSIGNAL")
	}

	sig := req.args[0]
	_, err := signal.ParseSignal(sig)
	if err != nil {
		return err
	}

	req.runConfig.StopSignal = sig
	return req.builder.commit(fmt.Sprintf("STOPSIGNAL %v", req.args))
}

// ARG name[=value]
//
// Adds the variable foo to the trusted list of variables that can be passed
// to builder using the --build-arg flag for expansion/substitution or passing to 'run'.
// Dockerfile author may optionally set a default value of this variable.
func arg(req dispatchRequest) error {
	if len(req.args) != 1 {
		return errExactlyOneArgument("ARG")
	}

	var (
		name       string
		newValue   string
		hasDefault bool
	)

	arg := req.args[0]
	// 'arg' can just be a name or name-value pair. Note that this is different
	// from 'env' that handles the split of name and value at the parser level.
	// The reason for doing it differently for 'arg' is that we support just
	// defining an arg and not assign it a value (while 'env' always expects a
	// name-value pair). If possible, it will be good to harmonize the two.
	if strings.Contains(arg, "=") {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts[0]) == 0 {
			return errBlankCommandNames("ARG")
		}

		name = parts[0]
		newValue = parts[1]
		hasDefault = true
	} else {
		name = arg
		hasDefault = false
	}

	var value *string
	if hasDefault {
		value = &newValue
	}
	req.builder.buildArgs.AddArg(name, value)

	// Arg before FROM doesn't add a layer
	if !req.builder.hasFromImage() {
		req.builder.buildArgs.AddMetaArg(name, value)
		return nil
	}
	return req.builder.commit("ARG " + arg)
}

// SHELL powershell -command
//
// Set the non-default shell to use.
func shell(req dispatchRequest) error {
	if err := req.flags.Parse(); err != nil {
		return err
	}
	shellSlice := handleJSONArgs(req.args, req.attributes)
	switch {
	case len(shellSlice) == 0:
		// SHELL []
		return errAtLeastOneArgument("SHELL")
	case req.attributes["json"]:
		// SHELL ["powershell", "-command"]
		req.runConfig.Shell = strslice.StrSlice(shellSlice)
	default:
		// SHELL powershell -command - not JSON
		return errNotJSON("SHELL", req.original)
	}
	return req.builder.commit(fmt.Sprintf("SHELL %v", shellSlice))
}

func errAtLeastOneArgument(command string) error {
	return fmt.Errorf("%s requires at least one argument", command)
}

func errExactlyOneArgument(command string) error {
	return fmt.Errorf("%s requires exactly one argument", command)
}

func errAtLeastTwoArguments(command string) error {
	return fmt.Errorf("%s requires at least two arguments", command)
}

func errBlankCommandNames(command string) error {
	return fmt.Errorf("%s names can not be blank", command)
}

func errTooManyArguments(command string) error {
	return fmt.Errorf("Bad input to %s, too many arguments", command)
}

// mountByRef creates an imageMount from a reference. pulling the image if needed.
func mountByRef(b *Builder, name string) (*imageMount, error) {
	image, err := pullOrGetImage(b, name)
	if err != nil {
		return nil, err
	}
	im := b.imageContexts.newImageMount(image.ImageID())
	return im, nil
}

func pullOrGetImage(b *Builder, name string) (builder.Image, error) {
	var image builder.Image
	if !b.options.PullParent {
		image, _ = b.docker.GetImageOnBuild(name)
		// TODO: shouldn't we error out if error is different from "not found" ?
	}
	if image == nil {
		var err error
		image, err = b.docker.PullOnBuild(b.clientCtx, name, b.options.AuthConfigs, b.Output)
		if err != nil {
			return nil, err
		}
	}
	return image, nil
}
