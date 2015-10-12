package dockerfile

// This file contains the dispatchers for each command. Note that
// `nullDispatch` is not actually a command, but support for commands we parse
// but do nothing with.
//
// See evaluator.go for a higher level discussion of the whole evaluator
// package.

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/Sirupsen/logrus"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/image"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/nat"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/runconfig"
)

const (
	// NoBaseImageSpecifier is the symbol used by the FROM
	// command to specify that no base image is to be used.
	NoBaseImageSpecifier string = "scratch"
)

// dispatch with no layer / parsing. This is effectively not a command.
func nullDispatch(b *Builder, args []string, attributes map[string]bool, original string) error {
	return nil
}

// ENV foo bar
//
// Sets the environment variable foo to bar, also makes interpolation
// in the dockerfile available from the next statement on via ${foo}.
//
func env(b *Builder, args []string, attributes map[string]bool, original string) error {
	if len(args) == 0 {
		return derr.ErrorCodeAtLeastOneArg.WithArgs("ENV")
	}

	if len(args)%2 != 0 {
		// should never get here, but just in case
		return derr.ErrorCodeTooManyArgs.WithArgs("ENV")
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
		newVar := args[j] + "=" + args[j+1] + ""
		commitStr += " " + newVar

		gotOne := false
		for i, envVar := range b.runConfig.Env {
			envParts := strings.SplitN(envVar, "=", 2)
			if envParts[0] == args[j] {
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
		return derr.ErrorCodeExactlyOneArg.WithArgs("MAINTAINER")
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
		return derr.ErrorCodeAtLeastOneArg.WithArgs("LABEL")
	}
	if len(args)%2 != 0 {
		// should never get here, but just in case
		return derr.ErrorCodeTooManyArgs.WithArgs("LABEL")
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
		return derr.ErrorCodeAtLeastTwoArgs.WithArgs("ADD")
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
		return derr.ErrorCodeAtLeastTwoArgs.WithArgs("COPY")
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
		return derr.ErrorCodeExactlyOneArg.WithArgs("FROM")
	}

	if err := b.flags.Parse(); err != nil {
		return err
	}

	name := args[0]

	// Windows cannot support a container with no base image.
	if name == NoBaseImageSpecifier {
		if runtime.GOOS == "windows" {
			return fmt.Errorf("Windows does not support FROM scratch")
		}
		b.image = ""
		b.noBaseImage = true
		return nil
	}

	var (
		image *image.Image
		err   error
	)
	// TODO: don't use `name`, instead resolve it to a digest
	if !b.Pull {
		image, err = b.docker.LookupImage(name)
		// TODO: shouldn't we error out if error is different from "not found" ?
	}
	if image == nil {
		image, err = b.docker.Pull(name)
		if err != nil {
			return err
		}
	}
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
		return derr.ErrorCodeAtLeastOneArg.WithArgs("ONBUILD")
	}

	if err := b.flags.Parse(); err != nil {
		return err
	}

	triggerInstruction := strings.ToUpper(strings.TrimSpace(args[0]))
	switch triggerInstruction {
	case "ONBUILD":
		return derr.ErrorCodeChainOnBuild
	case "MAINTAINER", "FROM":
		return derr.ErrorCodeBadOnBuildCmd.WithArgs(triggerInstruction)
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
		return derr.ErrorCodeExactlyOneArg.WithArgs("WORKDIR")
	}

	if err := b.flags.Parse(); err != nil {
		return err
	}

	// This is from the Dockerfile and will not necessarily be in platform
	// specific semantics, hence ensure it is converted.
	workdir := filepath.FromSlash(args[0])

	if !system.IsAbs(workdir) {
		current := filepath.FromSlash(b.runConfig.WorkingDir)
		workdir = filepath.Join(string(os.PathSeparator), current, workdir)
	}

	b.runConfig.WorkingDir = workdir

	return b.commit("", b.runConfig.Cmd, fmt.Sprintf("WORKDIR %v", workdir))
}

// RUN some command yo
//
// run a command and commit the image. Args are automatically prepended with
// 'sh -c' under linux or 'cmd /S /C' under Windows, in the event there is
// only one argument. The difference in processing:
//
// RUN echo hi          # sh -c echo hi       (Linux)
// RUN echo hi          # cmd /S /C echo hi   (Windows)
// RUN [ "echo", "hi" ] # echo hi
//
func run(b *Builder, args []string, attributes map[string]bool, original string) error {
	if b.image == "" && !b.noBaseImage {
		return derr.ErrorCodeMissingFrom
	}

	if err := b.flags.Parse(); err != nil {
		return err
	}

	args = handleJSONArgs(args, attributes)

	if !attributes["json"] {
		if runtime.GOOS != "windows" {
			args = append([]string{"/bin/sh", "-c"}, args...)
		} else {
			args = append([]string{"cmd", "/S", "/C"}, args...)
		}
	}

	runCmd := flag.NewFlagSet("run", flag.ContinueOnError)
	runCmd.SetOutput(ioutil.Discard)
	runCmd.Usage = nil

	config, _, _, err := runconfig.Parse(runCmd, append([]string{b.image}, args...))
	if err != nil {
		return err
	}

	// stash the cmd
	cmd := b.runConfig.Cmd
	runconfig.Merge(b.runConfig, config)
	// stash the config environment
	env := b.runConfig.Env

	defer func(cmd *stringutils.StrSlice) { b.runConfig.Cmd = cmd }(cmd)
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
	configEnv := runconfig.ConvertKVStringsToMap(b.runConfig.Env)
	for key, val := range b.BuildArgs {
		if !b.isBuildArgAllowed(key) {
			// skip build-args that are not in allowed list, meaning they have
			// not been defined by an "ARG" Dockerfile command yet.
			// This is an error condition but only if there is no "ARG" in the entire
			// Dockerfile, so we'll generate any necessary errors after we parsed
			// the entire file (see 'leftoverArgs' processing in evaluator.go )
			continue
		}
		if _, ok := configEnv[key]; !ok {
			cmdBuildEnv = append(cmdBuildEnv, fmt.Sprintf("%s=%s", key, val))
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
		saveCmd = stringutils.NewStrSlice(append(tmpEnv, saveCmd.Slice()...)...)
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

	logrus.Debugf("[BUILDER] Command to be executed: %v", b.runConfig.Cmd)

	c, err := b.create()
	if err != nil {
		return err
	}

	// Ensure that we keep the container mounted until the commit
	// to avoid unmounting and then mounting directly again
	c.Mount()
	defer c.Unmount()

	err = b.run(c)
	if err != nil {
		return err
	}

	// revert to original config environment and set the command string to
	// have the build-time env vars in it (if any) so that future cache look-ups
	// properly match it.
	b.runConfig.Env = env
	b.runConfig.Cmd = saveCmd
	if err := b.commit(c.ID, cmd, "run"); err != nil {
		return err
	}

	return nil
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
		if runtime.GOOS != "windows" {
			cmdSlice = append([]string{"/bin/sh", "-c"}, cmdSlice...)
		} else {
			cmdSlice = append([]string{"cmd", "/S", "/C"}, cmdSlice...)
		}
	}

	b.runConfig.Cmd = stringutils.NewStrSlice(cmdSlice...)

	if err := b.commit("", b.runConfig.Cmd, fmt.Sprintf("CMD %q", cmdSlice)); err != nil {
		return err
	}

	if len(args) != 0 {
		b.cmdSet = true
	}

	return nil
}

// ENTRYPOINT /usr/sbin/nginx
//
// Set the entrypoint (which defaults to sh -c on linux, or cmd /S /C on Windows) to
// /usr/sbin/nginx. Will accept the CMD as the arguments to /usr/sbin/nginx.
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
		b.runConfig.Entrypoint = stringutils.NewStrSlice(parsed...)
	case len(parsed) == 0:
		// ENTRYPOINT []
		b.runConfig.Entrypoint = nil
	default:
		// ENTRYPOINT echo hi
		if runtime.GOOS != "windows" {
			b.runConfig.Entrypoint = stringutils.NewStrSlice("/bin/sh", "-c", parsed[0])
		} else {
			b.runConfig.Entrypoint = stringutils.NewStrSlice("cmd", "/S /C", parsed[0])
		}
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
		return derr.ErrorCodeAtLeastOneArg.WithArgs("EXPOSE")
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
		return derr.ErrorCodeExactlyOneArg.WithArgs("USER")
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
		return derr.ErrorCodeAtLeastOneArg.WithArgs("VOLUME")
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
			return derr.ErrorCodeVolumeEmpty
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
		return fmt.Errorf("STOPSIGNAL requires exactly one argument")
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
		return fmt.Errorf("ARG requires exactly one argument definition")
	}

	var (
		name       string
		value      string
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
		name = parts[0]
		value = parts[1]
		hasDefault = true
	} else {
		name = arg
		hasDefault = false
	}
	// add the arg to allowed list of build-time args from this step on.
	b.allowedBuildArgs[name] = true

	// If there is a default value associated with this arg then add it to the
	// b.buildArgs if one is not already passed to the builder. The args passed
	// to builder override the defaut value of 'arg'.
	if _, ok := b.BuildArgs[name]; !ok && hasDefault {
		b.BuildArgs[name] = value
	}

	return b.commit("", b.runConfig.Cmd, fmt.Sprintf("ARG %s", arg))
}
