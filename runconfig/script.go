package runconfig

import (
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/nat"
	"github.com/dotcloud/docker/pkg/dockerfile"
	"path/filepath"
	"strings"
)

// Handle implements the dockerfile.Handler interface to allow scripting.
// FIXME:... and we could also output the contents of a config as a Dockerfile :-)
func (cfg *Config) Handle(stepname, cmd, arg string) error {
	return dockerfile.ReflectorHandler(cfg, nil).Handle(stepname, cmd, arg)
}

// The ONBUILD command declares a build instruction to be executed in any future build
// using the current image as a base.
func (cfg *Config) CmdOnbuild(trigger string) error {
	splitTrigger := strings.Split(trigger, " ")
	triggerInstruction := strings.ToUpper(strings.Trim(splitTrigger[0], " "))
	switch triggerInstruction {
	case "ONBUILD":
		return fmt.Errorf("Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed")
	case "MAINTAINER", "FROM":
		return fmt.Errorf("%s isn't allowed as an ONBUILD trigger", triggerInstruction)
	}
	cfg.OnBuild = append(cfg.OnBuild, trigger)
	return nil
}

func (cfg *Config) CmdEnv(args string) error {
	tmp := strings.SplitN(args, " ", 2)
	if len(tmp) != 2 {
		return fmt.Errorf("Invalid ENV format")
	}
	key := strings.Trim(tmp[0], " \t")
	value := strings.Trim(tmp[1], " \t")
	env := (*engine.Env)(&cfg.Env)
	env.Add(key, env.Expand(value))
	return nil
}

func (cfg *Config) FindEnvKey(key string) int {
	for k, envVar := range cfg.Env {
		envParts := strings.SplitN(envVar, "=", 2)
		if key == envParts[0] {
			return k
		}
	}
	return -1
}

func (cfg *Config) CmdCmd(args string) error {
	cfg.Cmd = parseArgCommand(args)
	return nil
}

func (cfg *Config) CmdEntrypoint(args string) error {
	cfg.Entrypoint = parseArgCommand(args)
	return nil
}

// parseArgCommand parses a command from a single string. The syntax is as follows:
// 1) If the string is a valid json-encoded array of strings, the decoded array is returned.
// 2) Otherwise, the 3-part array {"/bin/sh", "-c", <input>} is returned.
//
// Historical note: command parsing was implemented in this way as a stopgap while waiting for
// the correct solution: parsing a shell-like syntax for word separation (single quotes,
// double quotes and backquotes).
//
// FIXME: the aforementioned shell syntax is still not implemented. The additional difficulty
// is that we must now support the current syntax, which in certain cases conflicts with shell.
func parseArgCommand(input string) []string {
	var cmd []string
	if err := json.Unmarshal([]byte(input), &cmd); err != nil {
		cmd = []string{"/bin/sh", "-c", input}
	}
	return cmd
}

func (cfg *Config) CmdExpose(args string) error {
	portsTab := strings.Split(args, " ")

	if cfg.ExposedPorts == nil {
		cfg.ExposedPorts = make(nat.PortSet)
	}
	ports, _, err := nat.ParsePortSpecs(append(portsTab, cfg.PortSpecs...))
	if err != nil {
		return err
	}
	for port := range ports {
		if _, exists := cfg.ExposedPorts[port]; !exists {
			cfg.ExposedPorts[port] = struct{}{}
		}
	}
	cfg.PortSpecs = nil // Unset deprecated field
	return nil
}

func (cfg *Config) CmdUser(args string) error {
	cfg.User = args
	return nil
}

func (cfg *Config) CmdWorkdir(workdir string) error {
	if workdir[0] == '/' {
		cfg.WorkingDir = workdir
	} else {
		if cfg.WorkingDir == "" {
			cfg.WorkingDir = "/"
		}
		cfg.WorkingDir = filepath.Join(cfg.WorkingDir, workdir)
	}
	return nil
}

func (cfg *Config) CmdVolume(args string) error {
	if args == "" {
		return fmt.Errorf("Volume cannot be empty")
	}

	var volume []string
	if err := json.Unmarshal([]byte(args), &volume); err != nil {
		volume = []string{args}
	}
	if cfg.Volumes == nil {
		cfg.Volumes = map[string]struct{}{}
	}
	for _, v := range volume {
		cfg.Volumes[v] = struct{}{}
	}
	return nil
}

// Print a config back as a script

func (cfg *Config) AsScript() string {
	var ops []string
	if cfg.User != "" {
		ops = append(ops, fmt.Sprintf("user %s", cfg.User))
	}
	for port := range cfg.ExposedPorts {
		ops = append(ops, fmt.Sprintf("expose %s", port))
	}
	if len(cfg.Env) != 0 {
		ops = append(ops, (*engine.Env)(&cfg.Env).ToScript())
	}
	if len(cfg.Cmd) != 0 {
		if j, err := json.Marshal(cfg.Cmd); err == nil {
			ops = append(ops, fmt.Sprintf("cmd %s", j))
		}
	}
	for volume := range cfg.Volumes {
		ops = append(ops, fmt.Sprintf("volume %s", volume))
	}
	if cfg.WorkingDir != "" {
		ops = append(ops, fmt.Sprintf("workdir %s", cfg.WorkingDir))
	}
	if len(cfg.Entrypoint) != 0 {
		if j, err := json.Marshal(cfg.Entrypoint); err == nil {
			ops = append(ops, fmt.Sprintf("entrypoint %s", j))
		}
	}
	for _, trigger := range cfg.OnBuild {
		ops = append(ops, fmt.Sprintf("onbuild %s", trigger))
	}
	return strings.Join(ops, "\n")
}
