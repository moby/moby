package evaluator

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/docker/docker/nat"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

// dispatch with no layer / parsing.
func nullDispatch(b *buildFile, args []string) error {
	return nil
}

func env(b *buildFile, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("ENV accepts two arguments")
	}

	// the duplication here is intended to ease the replaceEnv() call's env
	// handling. This routine gets much shorter with the denormalization here.
	key := args[0]
	b.env[key] = args[1]
	b.config.Env = append(b.config.Env, strings.Join([]string{key, b.env[key]}, "="))

	return b.commit("", b.config.Cmd, fmt.Sprintf("ENV %s=%s", key, b.env[key]))
}

func maintainer(b *buildFile, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("MAINTAINER requires only one argument")
	}

	b.maintainer = args[0]
	return b.commit("", b.config.Cmd, fmt.Sprintf("MAINTAINER %s", b.maintainer))
}

func add(b *buildFile, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("ADD requires two arguments")
	}

	return b.runContextCommand(args, true, true, "ADD")
}

func dispatchCopy(b *buildFile, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("COPY requires two arguments")
	}

	return b.runContextCommand(args, false, false, "COPY")
}

func from(b *buildFile, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("FROM requires one argument")
	}

	name := args[0]

	image, err := b.options.Daemon.Repositories().LookupImage(name)
	if err != nil {
		if b.options.Daemon.Graph().IsNotExist(err) {
			image, err = b.pullImage(name)
		}

		// note that the top level err will still be !nil here if IsNotExist is
		// not the error. This approach just simplifies hte logic a bit.
		if err != nil {
			return err
		}
	}

	return b.processImageFrom(image)
}

func onbuild(b *buildFile, args []string) error {
	triggerInstruction := strings.ToUpper(strings.TrimSpace(args[0]))
	switch triggerInstruction {
	case "ONBUILD":
		return fmt.Errorf("Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed")
	case "MAINTAINER", "FROM":
		return fmt.Errorf("%s isn't allowed as an ONBUILD trigger", triggerInstruction)
	}

	trigger := strings.Join(args, " ")

	b.config.OnBuild = append(b.config.OnBuild, trigger)
	return b.commit("", b.config.Cmd, fmt.Sprintf("ONBUILD %s", trigger))
}

func workdir(b *buildFile, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("WORKDIR requires exactly one argument")
	}

	workdir := args[0]

	if workdir[0] == '/' {
		b.config.WorkingDir = workdir
	} else {
		if b.config.WorkingDir == "" {
			b.config.WorkingDir = "/"
		}
		b.config.WorkingDir = filepath.Join(b.config.WorkingDir, workdir)
	}

	return b.commit("", b.config.Cmd, fmt.Sprintf("WORKDIR %v", workdir))
}

func run(b *buildFile, args []string) error {
	if len(args) == 1 { // literal string command, not an exec array
		args = append([]string{"/bin/sh", "-c"}, args[0])
	}

	if b.image == "" {
		return fmt.Errorf("Please provide a source image with `from` prior to run")
	}

	config, _, _, err := runconfig.Parse(append([]string{b.image}, args...), nil)
	if err != nil {
		return err
	}

	cmd := b.config.Cmd
	// set Cmd manually, this is special case only for Dockerfiles
	b.config.Cmd = config.Cmd
	runconfig.Merge(b.config, config)

	defer func(cmd []string) { b.config.Cmd = cmd }(cmd)

	utils.Debugf("Command to be executed: %v", b.config.Cmd)

	hit, err := b.probeCache()
	if err != nil {
		return err
	}
	if hit {
		return nil
	}

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
	if err := b.commit(c.ID, cmd, "run"); err != nil {
		return err
	}

	return nil
}

func cmd(b *buildFile, args []string) error {
	if len(args) < 2 {
		args = append([]string{"/bin/sh", "-c"}, args...)
	}

	b.config.Cmd = args
	if err := b.commit("", b.config.Cmd, fmt.Sprintf("CMD %v", cmd)); err != nil {
		return err
	}

	b.cmdSet = true
	return nil
}

func entrypoint(b *buildFile, args []string) error {
	b.config.Entrypoint = args

	// if there is no cmd in current Dockerfile - cleanup cmd
	if !b.cmdSet {
		b.config.Cmd = nil
	}
	if err := b.commit("", b.config.Cmd, fmt.Sprintf("ENTRYPOINT %v", entrypoint)); err != nil {
		return err
	}
	return nil
}

func expose(b *buildFile, args []string) error {
	portsTab := args

	if b.config.ExposedPorts == nil {
		b.config.ExposedPorts = make(nat.PortSet)
	}

	ports, _, err := nat.ParsePortSpecs(append(portsTab, b.config.PortSpecs...))
	if err != nil {
		return err
	}

	for port := range ports {
		if _, exists := b.config.ExposedPorts[port]; !exists {
			b.config.ExposedPorts[port] = struct{}{}
		}
	}
	b.config.PortSpecs = nil

	return b.commit("", b.config.Cmd, fmt.Sprintf("EXPOSE %v", ports))
}

func user(b *buildFile, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("USER requires exactly one argument")
	}

	b.config.User = args[0]
	return b.commit("", b.config.Cmd, fmt.Sprintf("USER %v", args))
}

func volume(b *buildFile, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("Volume cannot be empty")
	}

	volume := args

	if b.config.Volumes == nil {
		b.config.Volumes = map[string]struct{}{}
	}
	for _, v := range volume {
		b.config.Volumes[v] = struct{}{}
	}
	if err := b.commit("", b.config.Cmd, fmt.Sprintf("VOLUME %s", args)); err != nil {
		return err
	}
	return nil
}

func insert(b *buildFile, args []string) error {
	return fmt.Errorf("INSERT has been deprecated. Please use ADD instead")
}
