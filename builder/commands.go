package builder

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/docker/docker/daemon"
	"github.com/docker/docker/nat"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

func (b *BuildFile) CmdFrom(name string) error {
	image, err := b.daemon.Repositories().LookupImage(name)
	if err != nil {
		if b.daemon.Graph().IsNotExist(err) {
			remote, tag := parsers.ParseRepositoryTag(name)
			pullRegistryAuth := b.authConfig
			if len(b.configFile.Configs) > 0 {
				// The request came with a full auth config file, we prefer to use that
				endpoint, _, err := registry.ResolveRepositoryName(remote)
				if err != nil {
					return err
				}
				resolvedAuth := b.configFile.ResolveAuthConfig(endpoint)
				pullRegistryAuth = &resolvedAuth
			}
			job := b.eng.Job("pull", remote, tag)
			job.SetenvBool("json", b.sf.Json())
			job.SetenvBool("parallel", true)
			job.SetenvJson("authConfig", pullRegistryAuth)
			job.Stdout.Add(b.outOld)
			if err := job.Run(); err != nil {
				return err
			}
			image, err = b.daemon.Repositories().LookupImage(name)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	b.image = image.ID
	b.config = &runconfig.Config{}
	if image.Config != nil {
		b.config = image.Config
	}
	if b.config.Env == nil || len(b.config.Env) == 0 {
		b.config.Env = append(b.config.Env, "HOME=/", "PATH="+daemon.DefaultPathEnv)
	}
	// Process ONBUILD triggers if they exist
	if nTriggers := len(b.config.OnBuild); nTriggers != 0 {
		fmt.Fprintf(b.errStream, "# Executing %d build triggers\n", nTriggers)
	}

	// Copy the ONBUILD triggers, and remove them from the config, since the config will be commited.
	onBuildTriggers := b.config.OnBuild
	b.config.OnBuild = []string{}

	for n, step := range onBuildTriggers {
		splitStep := strings.Split(step, " ")
		stepInstruction := strings.ToUpper(strings.Trim(splitStep[0], " "))
		switch stepInstruction {
		case "ONBUILD":
			return fmt.Errorf("Source image contains forbidden chained `ONBUILD ONBUILD` trigger: %s", step)
		case "MAINTAINER", "FROM":
			return fmt.Errorf("Source image contains forbidden %s trigger: %s", stepInstruction, step)
		}
		if err := b.BuildStep(fmt.Sprintf("onbuild-%d", n), step); err != nil {
			return err
		}
	}
	return nil
}

// The ONBUILD command declares a build instruction to be executed in any future build
// using the current image as a base.
func (b *BuildFile) CmdOnbuild(trigger string) error {
	splitTrigger := strings.Split(trigger, " ")
	triggerInstruction := strings.ToUpper(strings.Trim(splitTrigger[0], " "))
	switch triggerInstruction {
	case "ONBUILD":
		return fmt.Errorf("Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed")
	case "MAINTAINER", "FROM":
		return fmt.Errorf("%s isn't allowed as an ONBUILD trigger", triggerInstruction)
	}
	b.config.OnBuild = append(b.config.OnBuild, trigger)
	return b.commit("", b.config.Cmd, fmt.Sprintf("ONBUILD %s", trigger))
}

func (b *BuildFile) CmdMaintainer(name string) error {
	b.maintainer = name
	return b.commit("", b.config.Cmd, fmt.Sprintf("MAINTAINER %s", name))
}

func (b *BuildFile) CmdRun(args string) error {
	if b.image == "" {
		return fmt.Errorf("Please provide a source image with `from` prior to run")
	}
	config, _, _, err := runconfig.Parse(append([]string{b.image}, b.buildCmdFromJson(args)...), nil)
	if err != nil {
		return err
	}

	cmd := b.config.Cmd
	b.config.Cmd = nil
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

func (b *BuildFile) CmdEnv(args string) error {
	tmp := strings.SplitN(args, " ", 2)
	if len(tmp) != 2 {
		return fmt.Errorf("Invalid ENV format")
	}
	key := strings.Trim(tmp[0], " \t")
	value := strings.Trim(tmp[1], " \t")

	envKey := b.FindEnvKey(key)
	replacedValue, err := b.ReplaceEnvMatches(value)
	if err != nil {
		return err
	}
	replacedVar := fmt.Sprintf("%s=%s", key, replacedValue)

	if envKey >= 0 {
		b.config.Env[envKey] = replacedVar
	} else {
		b.config.Env = append(b.config.Env, replacedVar)
	}
	return b.commit("", b.config.Cmd, fmt.Sprintf("ENV %s", replacedVar))
}

func (b *BuildFile) CmdCmd(args string) error {
	cmd := b.buildCmdFromJson(args)
	b.config.Cmd = cmd
	if err := b.commit("", b.config.Cmd, fmt.Sprintf("CMD %v", cmd)); err != nil {
		return err
	}
	return nil
}

func (b *BuildFile) CmdEntrypoint(args string) error {
	entrypoint := b.buildCmdFromJson(args)
	b.config.Entrypoint = entrypoint
	if err := b.commit("", b.config.Cmd, fmt.Sprintf("ENTRYPOINT %v", entrypoint)); err != nil {
		return err
	}
	return nil
}

func (b *BuildFile) CmdExpose(args string) error {
	portsTab := strings.Split(args, " ")

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

func (b *BuildFile) CmdUser(args string) error {
	b.config.User = args
	return b.commit("", b.config.Cmd, fmt.Sprintf("USER %v", args))
}

func (b *BuildFile) CmdInsert(args string) error {
	return fmt.Errorf("INSERT has been deprecated. Please use ADD instead")
}

func (b *BuildFile) CmdCopy(args string) error {
	return b.runContextCommand(args, false, false, "COPY")
}

func (b *BuildFile) CmdWorkdir(workdir string) error {
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

func (b *BuildFile) CmdVolume(args string) error {
	if args == "" {
		return fmt.Errorf("Volume cannot be empty")
	}

	var volume []string
	if err := json.Unmarshal([]byte(args), &volume); err != nil {
		volume = []string{args}
	}
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

func (b *BuildFile) CmdAdd(args string) error {
	return b.runContextCommand(args, true, true, "ADD")
}
