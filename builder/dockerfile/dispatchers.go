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

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/pkg/signal"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/docker/go-connections/nat"
)

// ENV foo bar
//
// Sets the environment variable foo to bar, also makes interpolation
// in the dockerfile available from the next statement on via ${foo}.
//
func env(b *Builder, args []string, attributes map[string]bool, original string) error {
	if len(args) == 0 {
		return errAtLeastOneArgument("ENV")
	}

	if len(args)%2 != 0 {
		// should never get here, but just in case
		return errTooManyArguments("ENV")
	}

	if err := b.flags.Parse(); err != nil {
		return err
	}

	// TODO/FIXME/NOT USED
	// Just here to show how to use the builder flags stuff within the
	// context of a builder command. Will remove once we actually add
	// a builder command to something!
	/*
		flBool1 := b.flags.AddBool("bool1", false)
		flStr1 := b.flags.AddString("str1", "HI")

		if err := b.flags.Parse(); err != nil {
			return err
		}

		fmt.Printf("Bool1:%v\n", flBool1)
		fmt.Printf("Str1:%v\n", flStr1)
	*/

	commitStr := "ENV"

	for j := 0; j < len(args); j++ {
		// name  ==> args[j]
		// value ==> args[j+1]

		if len(args[j]) == 0 {
			return errBlankCommandNames("ENV")
		}
		newVar := args[j] + "=" + args[j+1] + ""
		commitStr += " " + newVar

		gotOne := false
		for i, envVar := range b.runConfig.Env {
			envParts := strings.SplitN(envVar, "=", 2)
			compareFrom := envParts[0]
			compareTo := args[j]
			if runtime.GOOS == "windows" {
				// Case insensitive environment variables on Windows
				compareFrom = strings.ToUpper(compareFrom)
				compareTo = strings.ToUpper(compareTo)
			}
			if compareFrom == compareTo {
				b.runConfig.Env[i] = newVar
				gotOne = true
				break
			}
		}
		if !gotOne {
			b.runConfig.Env = append(b.runConfig.Env, newVar)
		}
		j++
	}

	return b.commit("", b.runConfig.Cmd, commitStr)
}

// MAINTAINER some text <maybe@an.email.address>
//
// Sets the maintainer metadata.
func maintainer(b *Builder, args []string, attributes map[string]bool, original string) error {
	if len(args) != 1 {
		return errExactlyOneArgument("MAINTAINER")
	}

	if err := b.flags.Parse(); err != nil {
		return err
	}

	b.maintainer = args[0]
	return b.commit("", b.runConfig.Cmd, fmt.Sprintf("MAINTAINER %s", b.maintainer))
}

// LABEL some json data describing the image
//
// Sets the Label variable foo to bar,
//
func label(b *Builder, args []string, attributes map[string]bool, original string) error {
	if len(args) == 0 {
		return errAtLeastOneArgument("LABEL")
	}
	if len(args)%2 != 0 {
		// should never get here, but just in case
		return errTooManyArguments("LABEL")
	}

	if err := b.flags.Parse(); err != nil {
		return err
	}

	commitStr := "LABEL"

	if b.runConfig.Labels == nil {
		b.runConfig.Labels = map[string]string{}
	}

	for j := 0; j < len(args); j++ {
		// name  ==> args[j]
		// value ==> args[j+1]

		if len(args[j]) == 0 {
			return errBlankCommandNames("LABEL")
		}

		newVar := args[j] + "=" + args[j+1] + ""
		commitStr += " " + newVar

		b.runConfig.Labels[args[j]] = args[j+1]
		j++
	}
	return b.commit("", b.runConfig.Cmd, commitStr)
}

// ADD foo /path
//
// Add the file 'foo' to '/path'. Tarball and Remote URL (git, http) handling
// exist here. If you do not wish to have this automatic handling, use COPY.
//
func add(b *Builder, args []string, attributes map[string]bool, original string) error {
	if len(args) < 2 {
		return errAtLeastTwoArguments("ADD")
	}

	if err := b.flags.Parse(); err != nil {
		return err
	}

	return b.runContextCommand(args, true, true, "ADD")
}

// COPY foo /path
//
// Same as 'ADD' but without the tar and remote url handling.
//
func dispatchCopy(b *Builder, args []string, attributes map[string]bool, original string) error {
	if len(args) < 2 {
		return errAtLeastTwoArguments("COPY")
	}

	if err := b.flags.Parse(); err != nil {
		return err
	}

	return b.runContextCommand(args, false, false, "COPY")
}

// FROM imagename
//
// This sets the image the dockerfile will build on top of.
//
func from(b *Builder, args []string, attributes map[string]bool, original string) error {
	if len(args) != 1 {
		return errExactlyOneArgument("FROM")
	}

	if err := b.flags.Parse(); err != nil {
		return err
	}

	name := args[0]

	var (
		image builder.Image
		err   error
	)

	// Windows cannot support a container with no base image.
	if name == api.NoBaseImageSpecifier {
		if runtime.GOOS == "windows" {
			return fmt.Errorf("Windows does not support FROM scratch")
		}
		b.image = ""
		b.noBaseImage = true
	} else {
		// TODO: don't use `name`, instead resolve it to a digest
		if !b.options.PullParent {
			image, err = b.docker.GetImageOnBuild(name)
			// TODO: shouldn't we error out if error is different from "not found" ?
		}
		if image == nil {
			image, err = b.docker.PullOnBuild(b.clientCtx, name, b.options.AuthConfigs, b.Output)
			if err != nil {
				return err
			}
		}
	}
	b.from = image

	return b.processImageFrom(image)
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
func onbuild(b *Builder, args []string, attributes map[string]bool, original string) error {
	if len(args) == 0 {
		return errAtLeastOneArgument("ONBUILD")
	}

	if err := b.flags.Parse(); err != nil {
		return err
	}

	triggerInstruction := strings.ToUpper(strings.TrimSpace(args[0]))
	switch triggerInstruction {
	case "ONBUILD":
		return fmt.Errorf("Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed")
	case "MAINTAINER", "FROM":
		return fmt.Errorf("%s isn't allowed as an ONBUILD trigger", triggerInstruction)
	}

	original = regexp.MustCompile(`(?i)^\s*ONBUILD\s*`).ReplaceAllString(original, "")

	b.runConfig.OnBuild = append(b.runConfig.OnBuild, original)
	return b.commit("", b.runConfig.Cmd, fmt.Sprintf("ONBUILD %s", original))
}

// WORKDIR /tmp
//
// Set the working directory for future RUN/CMD/etc statements.
//
func workdir(b *Builder, args []string, attributes map[string]bool, original string) error {
	if len(args) != 1 {
		return errExactlyOneArgument("WORKDIR")
	}

	err := b.flags.Parse()
	if err != nil {
		return err
	}

	// This is from the Dockerfile and will not necessarily be in platform
	// specific semantics, hence ensure it is converted.
	b.runConfig.WorkingDir, err = normaliseWorkdir(b.runConfig.WorkingDir, args[0])
	if err != nil {
		return err
	}

	// For performance reasons, we explicitly do a create/mkdir now
	// This avoids having an unnecessary expensive mount/unmount calls
	// (on Windows in particular) during each container create.
	// Prior to 1.13, the mkdir was deferred and not executed at this step.
	if b.disableCommit {
		// Don't call back into the daemon if we're going through docker commit --change "WORKDIR /foo".
		// We've already updated the runConfig and that's enough.
		return nil
	}
	b.runConfig.Image = b.image

	cmd := b.runConfig.Cmd
	comment := "WORKDIR " + b.runConfig.WorkingDir
	// reset the command for cache detection
	b.runConfig.Cmd = strslice.StrSlice(append(getShell(b.runConfig), "#(nop) "+comment))
	defer func(cmd strslice.StrSlice) { b.runConfig.Cmd = cmd }(cmd)

	if hit, err := b.probeCache(); err != nil {
		return err
	} else if hit {
		return nil
	}

	container, err := b.docker.ContainerCreate(types.ContainerCreateConfig{Config: b.runConfig})
	if err != nil {
		return err
	}
	b.tmpContainers[container.ID] = struct{}{}
	if err := b.docker.ContainerCreateWorkdir(container.ID); err != nil {
		return err
	}

	return b.commit(container.ID, cmd, comment)
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
func run(b *Builder, args []string, attributes map[string]bool, original string) error {
	if b.image == "" && !b.noBaseImage {
		return fmt.Errorf("Please provide a source image with `from` prior to run")
	}

	if err := b.flags.Parse(); err != nil {
		return err
	}

	args = handleJSONArgs(args, attributes)

	if !attributes["json"] {
		args = append(getShell(b.runConfig), args...)
	}
	config := &container.Config{
		Cmd:   strslice.StrSlice(args),
		Image: b.image,
	}

	// stash the cmd
	cmd := b.runConfig.Cmd
	if len(b.runConfig.Entrypoint) == 0 && len(b.runConfig.Cmd) == 0 {
		b.runConfig.Cmd = config.Cmd
	}

	// stash the config environment
	env := b.runConfig.Env

	defer func(cmd strslice.StrSlice) { b.runConfig.Cmd = cmd }(cmd)
	defer func(env []string) { b.runConfig.Env = env }(env)

	// derive the net build-time environment for this run. We let config
	// environment override the build time environment.
	// This means that we take the b.buildArgs list of env vars and remove
	// any of those variables that are defined as part of the container. In other
	// words, anything in b.Config.Env. What's left is the list of build-time env
	// vars that we need to add to each RUN command - note the list could be empty.
	//
	// We don't persist the build time environment with container's config
	// environment, but just sort and prepend it to the command string at time
	// of commit.
	// This helps with tracing back the image's actual environment at the time
	// of RUN, without leaking it to the final image. It also aids cache
	// lookup for same image built with same build time environment.
	cmdBuildEnv := []string{}
	configEnv := runconfigopts.ConvertKVStringsToMap(b.runConfig.Env)
	for key, val := range b.options.BuildArgs {
		if !b.isBuildArgAllowed(key) {
			// skip build-args that are not in allowed list, meaning they have
			// not been defined by an "ARG" Dockerfile command yet.
			// This is an error condition but only if there is no "ARG" in the entire
			// Dockerfile, so we'll generate any necessary errors after we parsed
			// the entire file (see 'leftoverArgs' processing in evaluator.go )
			continue
		}
		if _, ok := configEnv[key]; !ok && val != nil {
			cmdBuildEnv = append(cmdBuildEnv, fmt.Sprintf("%s=%s", key, *val))
		}
	}

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

	b.runConfig.Cmd = saveCmd
	hit, err := b.probeCache()
	if err != nil {
		return err
	}
	if hit {
		return nil
	}

	// set Cmd manually, this is special case only for Dockerfiles
	b.runConfig.Cmd = config.Cmd
	// set build-time environment for 'run'.
	b.runConfig.Env = append(b.runConfig.Env, cmdBuildEnv...)
	// set config as already being escaped, this prevents double escaping on windows
	b.runConfig.ArgsEscaped = true

	logrus.Debugf("[BUILDER] Command to be executed: %v", b.runConfig.Cmd)

	cID, err := b.create()
	if err != nil {
		return err
	}

	if err := b.run(cID); err != nil {
		return err
	}

	// revert to original config environment and set the command string to
	// have the build-time env vars in it (if any) so that future cache look-ups
	// properly match it.
	b.runConfig.Env = env
	b.runConfig.Cmd = saveCmd
	return b.commit(cID, cmd, "run")
}

// CMD foo
//
// Set the default command to run in the container (which may be empty).
// Argument handling is the same as RUN.
//
func cmd(b *Builder, args []string, attributes map[string]bool, original string) error {
	if err := b.flags.Parse(); err != nil {
		return err
	}

	cmdSlice := handleJSONArgs(args, attributes)

	if !attributes["json"] {
		cmdSlice = append(getShell(b.runConfig), cmdSlice...)
	}

	b.runConfig.Cmd = strslice.StrSlice(cmdSlice)
	// set config as already being escaped, this prevents double escaping on windows
	b.runConfig.ArgsEscaped = true

	if err := b.commit("", b.runConfig.Cmd, fmt.Sprintf("CMD %q", cmdSlice)); err != nil {
		return err
	}

	if len(args) != 0 {
		b.cmdSet = true
	}

	return nil
}

// parseOptInterval(flag) is the duration of flag.Value, or 0 if
// empty. An error is reported if the value is given and is not positive.
func parseOptInterval(f *Flag) (time.Duration, error) {
	s := f.Value
	if s == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return 0, fmt.Errorf("Interval %#v must be positive", f.name)
	}
	return d, nil
}

// HEALTHCHECK foo
//
// Set the default healthcheck command to run in the container (which may be empty).
// Argument handling is the same as RUN.
//
func healthcheck(b *Builder, args []string, attributes map[string]bool, original string) error {
	if len(args) == 0 {
		return errAtLeastOneArgument("HEALTHCHECK")
	}
	typ := strings.ToUpper(args[0])
	args = args[1:]
	if typ == "NONE" {
		if len(args) != 0 {
			return fmt.Errorf("HEALTHCHECK NONE takes no arguments")
		}
		test := strslice.StrSlice{typ}
		b.runConfig.Healthcheck = &container.HealthConfig{
			Test: test,
		}
	} else {
		if b.runConfig.Healthcheck != nil {
			oldCmd := b.runConfig.Healthcheck.Test
			if len(oldCmd) > 0 && oldCmd[0] != "NONE" {
				fmt.Fprintf(b.Stdout, "Note: overriding previous HEALTHCHECK: %v\n", oldCmd)
			}
		}

		healthcheck := container.HealthConfig{}

		flInterval := b.flags.AddString("interval", "")
		flTimeout := b.flags.AddString("timeout", "")
		flRetries := b.flags.AddString("retries", "")

		if err := b.flags.Parse(); err != nil {
			return err
		}

		switch typ {
		case "CMD":
			cmdSlice := handleJSONArgs(args, attributes)
			if len(cmdSlice) == 0 {
				return fmt.Errorf("Missing command after HEALTHCHECK CMD")
			}

			if !attributes["json"] {
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

		b.runConfig.Healthcheck = &healthcheck
	}

	return b.commit("", b.runConfig.Cmd, fmt.Sprintf("HEALTHCHECK %q", b.runConfig.Healthcheck))
}

// ENTRYPOINT /usr/sbin/nginx
//
// Set the entrypoint to /usr/sbin/nginx. Will accept the CMD as the arguments
// to /usr/sbin/nginx. Uses the default shell if not in JSON format.
//
// Handles command processing similar to CMD and RUN, only b.runConfig.Entrypoint
// is initialized at NewBuilder time instead of through argument parsing.
//
func entrypoint(b *Builder, args []string, attributes map[string]bool, original string) error {
	if err := b.flags.Parse(); err != nil {
		return err
	}

	parsed := handleJSONArgs(args, attributes)

	switch {
	case attributes["json"]:
		// ENTRYPOINT ["echo", "hi"]
		b.runConfig.Entrypoint = strslice.StrSlice(parsed)
	case len(parsed) == 0:
		// ENTRYPOINT []
		b.runConfig.Entrypoint = nil
	default:
		// ENTRYPOINT echo hi
		b.runConfig.Entrypoint = strslice.StrSlice(append(getShell(b.runConfig), parsed[0]))
	}

	// when setting the entrypoint if a CMD was not explicitly set then
	// set the command to nil
	if !b.cmdSet {
		b.runConfig.Cmd = nil
	}

	if err := b.commit("", b.runConfig.Cmd, fmt.Sprintf("ENTRYPOINT %q", b.runConfig.Entrypoint)); err != nil {
		return err
	}

	return nil
}

// EXPOSE 6667/tcp 7000/tcp
//
// Expose ports for links and port mappings. This all ends up in
// b.runConfig.ExposedPorts for runconfig.
//
func expose(b *Builder, args []string, attributes map[string]bool, original string) error {
	portsTab := args

	if len(args) == 0 {
		return errAtLeastOneArgument("EXPOSE")
	}

	if err := b.flags.Parse(); err != nil {
		return err
	}

	if b.runConfig.ExposedPorts == nil {
		b.runConfig.ExposedPorts = make(nat.PortSet)
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
		if _, exists := b.runConfig.ExposedPorts[port]; !exists {
			b.runConfig.ExposedPorts[port] = struct{}{}
		}
		portList[i] = string(port)
		i++
	}
	sort.Strings(portList)
	return b.commit("", b.runConfig.Cmd, fmt.Sprintf("EXPOSE %s", strings.Join(portList, " ")))
}

// USER foo
//
// Set the user to 'foo' for future commands and when running the
// ENTRYPOINT/CMD at container run time.
//
func user(b *Builder, args []string, attributes map[string]bool, original string) error {
	if len(args) != 1 {
		return errExactlyOneArgument("USER")
	}

	if err := b.flags.Parse(); err != nil {
		return err
	}

	b.runConfig.User = args[0]
	return b.commit("", b.runConfig.Cmd, fmt.Sprintf("USER %v", args))
}

// VOLUME /foo
//
// Expose the volume /foo for use. Will also accept the JSON array form.
//
func volume(b *Builder, args []string, attributes map[string]bool, original string) error {
	if len(args) == 0 {
		return errAtLeastOneArgument("VOLUME")
	}

	if err := b.flags.Parse(); err != nil {
		return err
	}

	if b.runConfig.Volumes == nil {
		b.runConfig.Volumes = map[string]struct{}{}
	}
	for _, v := range args {
		v = strings.TrimSpace(v)
		if v == "" {
			return fmt.Errorf("VOLUME specified can not be an empty string")
		}
		b.runConfig.Volumes[v] = struct{}{}
	}
	if err := b.commit("", b.runConfig.Cmd, fmt.Sprintf("VOLUME %v", args)); err != nil {
		return err
	}
	return nil
}

// STOPSIGNAL signal
//
// Set the signal that will be used to kill the container.
func stopSignal(b *Builder, args []string, attributes map[string]bool, original string) error {
	if len(args) != 1 {
		return errExactlyOneArgument("STOPSIGNAL")
	}

	sig := args[0]
	_, err := signal.ParseSignal(sig)
	if err != nil {
		return err
	}

	b.runConfig.StopSignal = sig
	return b.commit("", b.runConfig.Cmd, fmt.Sprintf("STOPSIGNAL %v", args))
}

// ARG name[=value]
//
// Adds the variable foo to the trusted list of variables that can be passed
// to builder using the --build-arg flag for expansion/subsitution or passing to 'run'.
// Dockerfile author may optionally set a default value of this variable.
func arg(b *Builder, args []string, attributes map[string]bool, original string) error {
	if len(args) != 1 {
		return errExactlyOneArgument("ARG")
	}

	var (
		name       string
		newValue   string
		hasDefault bool
	)

	arg := args[0]
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
	// add the arg to allowed list of build-time args from this step on.
	b.allowedBuildArgs[name] = true

	// If there is a default value associated with this arg then add it to the
	// b.buildArgs if one is not already passed to the builder. The args passed
	// to builder override the default value of 'arg'. Note that a 'nil' for
	// a value means that the user specified "--build-arg FOO" and "FOO" wasn't
	// defined as an env var - and in that case we DO want to use the default
	// value specified in the ARG cmd.
	if baValue, ok := b.options.BuildArgs[name]; (!ok || baValue == nil) && hasDefault {
		b.options.BuildArgs[name] = &newValue
	}

	return b.commit("", b.runConfig.Cmd, fmt.Sprintf("ARG %s", arg))
}

// SHELL powershell -command
//
// Set the non-default shell to use.
func shell(b *Builder, args []string, attributes map[string]bool, original string) error {
	if err := b.flags.Parse(); err != nil {
		return err
	}
	shellSlice := handleJSONArgs(args, attributes)
	switch {
	case len(shellSlice) == 0:
		// SHELL []
		return errAtLeastOneArgument("SHELL")
	case attributes["json"]:
		// SHELL ["powershell", "-command"]
		b.runConfig.Shell = strslice.StrSlice(shellSlice)
	default:
		// SHELL powershell -command - not JSON
		return errNotJSON("SHELL", original)
	}
	return b.commit("", b.runConfig.Cmd, fmt.Sprintf("SHELL %v", shellSlice))
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

// getShell is a helper function which gets the right shell for prefixing the
// shell-form of RUN, ENTRYPOINT and CMD instructions
func getShell(c *container.Config) []string {
	if 0 == len(c.Shell) {
		return defaultShell[:]
	}
	return c.Shell[:]
}
