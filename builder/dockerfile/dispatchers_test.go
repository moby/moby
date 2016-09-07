package dockerfile

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	"github.com/docker/engine-api/types/strslice"
	"github.com/docker/go-connections/nat"
	"github.com/go-check/check"
)

type commandWithFunction struct {
	name     string
	function func(args []string) error
}

func (s *DockerSuite) TestCommandsExactlyOneArgument(c *check.C) {
	commands := []commandWithFunction{
		{"MAINTAINER", func(args []string) error { return maintainer(nil, args, nil, "") }},
		{"FROM", func(args []string) error { return from(nil, args, nil, "") }},
		{"WORKDIR", func(args []string) error { return workdir(nil, args, nil, "") }},
		{"USER", func(args []string) error { return user(nil, args, nil, "") }},
		{"STOPSIGNAL", func(args []string) error { return stopSignal(nil, args, nil, "") }}}

	for _, command := range commands {
		err := command.function([]string{})

		if err == nil {
			c.Fatalf("Error should be present for %s command", command.name)
		}

		expectedError := errExactlyOneArgument(command.name)

		if err.Error() != expectedError.Error() {
			c.Fatalf("Wrong error message for %s. Got: %s. Should be: %s", command.name, err.Error(), expectedError)
		}
	}
}

func (s *DockerSuite) TestCommandsAtLeastOneArgument(c *check.C) {
	commands := []commandWithFunction{
		{"ENV", func(args []string) error { return env(nil, args, nil, "") }},
		{"LABEL", func(args []string) error { return label(nil, args, nil, "") }},
		{"ONBUILD", func(args []string) error { return onbuild(nil, args, nil, "") }},
		{"HEALTHCHECK", func(args []string) error { return healthcheck(nil, args, nil, "") }},
		{"EXPOSE", func(args []string) error { return expose(nil, args, nil, "") }},
		{"VOLUME", func(args []string) error { return volume(nil, args, nil, "") }}}

	for _, command := range commands {
		err := command.function([]string{})

		if err == nil {
			c.Fatalf("Error should be present for %s command", command.name)
		}

		expectedError := errAtLeastOneArgument(command.name)

		if err.Error() != expectedError.Error() {
			c.Fatalf("Wrong error message for %s. Got: %s. Should be: %s", command.name, err.Error(), expectedError)
		}
	}
}

func (s *DockerSuite) TestCommandsAtLeastTwoArguments(c *check.C) {
	commands := []commandWithFunction{
		{"ADD", func(args []string) error { return add(nil, args, nil, "") }},
		{"COPY", func(args []string) error { return dispatchCopy(nil, args, nil, "") }}}

	for _, command := range commands {
		err := command.function([]string{"arg1"})

		if err == nil {
			c.Fatalf("Error should be present for %s command", command.name)
		}

		expectedError := errAtLeastTwoArguments(command.name)

		if err.Error() != expectedError.Error() {
			c.Fatalf("Wrong error message for %s. Got: %s. Should be: %s", command.name, err.Error(), expectedError)
		}
	}
}

func (s *DockerSuite) TestCommandsTooManyArguments(c *check.C) {
	commands := []commandWithFunction{
		{"ENV", func(args []string) error { return env(nil, args, nil, "") }},
		{"LABEL", func(args []string) error { return label(nil, args, nil, "") }}}

	for _, command := range commands {
		err := command.function([]string{"arg1", "arg2", "arg3"})

		if err == nil {
			c.Fatalf("Error should be present for %s command", command.name)
		}

		expectedError := errTooManyArguments(command.name)

		if err.Error() != expectedError.Error() {
			c.Fatalf("Wrong error message for %s. Got: %s. Should be: %s", command.name, err.Error(), expectedError)
		}
	}
}

func (s *DockerSuite) TestCommandseBlankNames(c *check.C) {
	bflags := &BFlags{}
	config := &container.Config{}

	b := &Builder{flags: bflags, runConfig: config, disableCommit: true}

	commands := []commandWithFunction{
		{"ENV", func(args []string) error { return env(b, args, nil, "") }},
		{"LABEL", func(args []string) error { return label(b, args, nil, "") }},
	}

	for _, command := range commands {
		err := command.function([]string{"", ""})

		if err == nil {
			c.Fatalf("Error should be present for %s command", command.name)
		}

		expectedError := errBlankCommandNames(command.name)

		if err.Error() != expectedError.Error() {
			c.Fatalf("Wrong error message for %s. Got: %s. Should be: %s", command.name, err.Error(), expectedError)
		}
	}
}

func (s *DockerSuite) TestEnv2Variables(c *check.C) {
	variables := []string{"var1", "val1", "var2", "val2"}

	bflags := &BFlags{}
	config := &container.Config{}

	b := &Builder{flags: bflags, runConfig: config, disableCommit: true}

	if err := env(b, variables, nil, ""); err != nil {
		c.Fatalf("Error when executing env: %s", err.Error())
	}

	expectedVar1 := fmt.Sprintf("%s=%s", variables[0], variables[1])
	expectedVar2 := fmt.Sprintf("%s=%s", variables[2], variables[3])

	if b.runConfig.Env[0] != expectedVar1 {
		c.Fatalf("Wrong env output for first variable. Got: %s. Should be: %s", b.runConfig.Env[0], expectedVar1)
	}

	if b.runConfig.Env[1] != expectedVar2 {
		c.Fatalf("Wrong env output for second variable. Got: %s, Should be: %s", b.runConfig.Env[1], expectedVar2)
	}
}

func (s *DockerSuite) TestMaintainer(c *check.C) {
	maintainerEntry := "Some Maintainer <maintainer@example.com>"

	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	if err := maintainer(b, []string{maintainerEntry}, nil, ""); err != nil {
		c.Fatalf("Error when executing maintainer: %s", err.Error())
	}

	if b.maintainer != maintainerEntry {
		c.Fatalf("Maintainer in builder should be set to %s. Got: %s", maintainerEntry, b.maintainer)
	}
}

func (s *DockerSuite) TestLabel(c *check.C) {
	labelName := "label"
	labelValue := "value"

	labelEntry := []string{labelName, labelValue}

	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	if err := label(b, labelEntry, nil, ""); err != nil {
		c.Fatalf("Error when executing label: %s", err.Error())
	}

	if val, ok := b.runConfig.Labels[labelName]; ok {
		if val != labelValue {
			c.Fatalf("Label %s should have value %s, had %s instead", labelName, labelValue, val)
		}
	} else {
		c.Fatalf("Label %s should be present but it is not", labelName)
	}
}

func (s *DockerSuite) TestFrom(c *check.C) {
	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	err := from(b, []string{"scratch"}, nil, "")

	if runtime.GOOS == "windows" {
		if err == nil {
			c.Fatalf("Error not set on Windows")
		}

		expectedError := "Windows does not support FROM scratch"

		if !strings.Contains(err.Error(), expectedError) {
			c.Fatalf("Error message not correct on Windows. Should be: %s, got: %s", expectedError, err.Error())
		}
	} else {
		if err != nil {
			c.Fatalf("Error when executing from: %s", err.Error())
		}

		if b.image != "" {
			c.Fatalf("Image shoule be empty, got: %s", b.image)
		}

		if b.noBaseImage != true {
			c.Fatalf("Image should not have any base image, got: %v", b.noBaseImage)
		}
	}
}

func (s *DockerSuite) TestOnbuildIllegalTriggers(c *check.C) {
	triggers := []struct{ command, expectedError string }{
		{"ONBUILD", "Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed"},
		{"MAINTAINER", "MAINTAINER isn't allowed as an ONBUILD trigger"},
		{"FROM", "FROM isn't allowed as an ONBUILD trigger"}}

	for _, trigger := range triggers {
		b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

		err := onbuild(b, []string{trigger.command}, nil, "")

		if err == nil {
			c.Fatalf("Error should not be nil")
		}

		if !strings.Contains(err.Error(), trigger.expectedError) {
			c.Fatalf("Error message not correct. Should be: %s, got: %s", trigger.expectedError, err.Error())
		}
	}
}

func (s *DockerSuite) TestOnbuild(c *check.C) {
	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	err := onbuild(b, []string{"ADD", ".", "/app/src"}, nil, "ONBUILD ADD . /app/src")

	if err != nil {
		c.Fatalf("Error should be empty, got: %s", err.Error())
	}

	expectedOnbuild := "ADD . /app/src"

	if b.runConfig.OnBuild[0] != expectedOnbuild {
		c.Fatalf("Wrong ONBUILD command. Expected: %s, got: %s", expectedOnbuild, b.runConfig.OnBuild[0])
	}
}

func (s *DockerSuite) TestWorkdir(c *check.C) {
	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	workingDir := "/app"

	if runtime.GOOS == "windows" {
		workingDir = "C:\app"
	}

	err := workdir(b, []string{workingDir}, nil, "")

	if err != nil {
		c.Fatalf("Error should be empty, got: %s", err.Error())
	}

	if b.runConfig.WorkingDir != workingDir {
		c.Fatalf("WorkingDir should be set to %s, got %s", workingDir, b.runConfig.WorkingDir)
	}

}

func (s *DockerSuite) TestCmd(c *check.C) {
	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	command := "./executable"

	err := cmd(b, []string{command}, nil, "")

	if err != nil {
		c.Fatalf("Error should be empty, got: %s", err.Error())
	}

	var expectedCommand strslice.StrSlice

	if runtime.GOOS == "windows" {
		expectedCommand = strslice.StrSlice(append([]string{"cmd"}, "/S", "/C", command))
	} else {
		expectedCommand = strslice.StrSlice(append([]string{"/bin/sh"}, "-c", command))
	}

	if !compareStrSlice(b.runConfig.Cmd, expectedCommand) {
		c.Fatalf("Command should be set to %s, got %s", command, b.runConfig.Cmd)
	}

	if !b.cmdSet {
		c.Fatalf("Command should be marked as set")
	}
}

func compareStrSlice(slice1, slice2 strslice.StrSlice) bool {
	if len(slice1) != len(slice2) {
		return false
	}

	for i := range slice1 {
		if slice1[i] != slice2[i] {
			return false
		}
	}

	return true
}

func (s *DockerSuite) TestHealthcheckNone(c *check.C) {
	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	if err := healthcheck(b, []string{"NONE"}, nil, ""); err != nil {
		c.Fatalf("Error should be empty, got: %s", err.Error())
	}

	if b.runConfig.Healthcheck == nil {
		c.Fatal("Healthcheck should be set, got nil")
	}

	expectedTest := strslice.StrSlice(append([]string{"NONE"}))

	if !compareStrSlice(expectedTest, b.runConfig.Healthcheck.Test) {
		c.Fatalf("Command should be set to %s, got %s", expectedTest, b.runConfig.Healthcheck.Test)
	}
}

func (s *DockerSuite) TestHealthcheckCmd(c *check.C) {
	b := &Builder{flags: &BFlags{flags: make(map[string]*Flag)}, runConfig: &container.Config{}, disableCommit: true}

	if err := healthcheck(b, []string{"CMD", "curl", "-f", "http://localhost/", "||", "exit", "1"}, nil, ""); err != nil {
		c.Fatalf("Error should be empty, got: %s", err.Error())
	}

	if b.runConfig.Healthcheck == nil {
		c.Fatal("Healthcheck should be set, got nil")
	}

	expectedTest := strslice.StrSlice(append([]string{"CMD-SHELL"}, "curl -f http://localhost/ || exit 1"))

	if !compareStrSlice(expectedTest, b.runConfig.Healthcheck.Test) {
		c.Fatalf("Command should be set to %s, got %s", expectedTest, b.runConfig.Healthcheck.Test)
	}
}

func (s *DockerSuite) TestEntrypoint(c *check.C) {
	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	entrypointCmd := "/usr/sbin/nginx"

	if err := entrypoint(b, []string{entrypointCmd}, nil, ""); err != nil {
		c.Fatalf("Error should be empty, got: %s", err.Error())
	}

	if b.runConfig.Entrypoint == nil {
		c.Fatalf("Entrypoint should be set")
	}

	var expectedEntrypoint strslice.StrSlice

	if runtime.GOOS == "windows" {
		expectedEntrypoint = strslice.StrSlice(append([]string{"cmd"}, "/S", "/C", entrypointCmd))
	} else {
		expectedEntrypoint = strslice.StrSlice(append([]string{"/bin/sh"}, "-c", entrypointCmd))
	}

	if !compareStrSlice(expectedEntrypoint, b.runConfig.Entrypoint) {
		c.Fatalf("Entrypoint command should be set to %s, got %s", expectedEntrypoint, b.runConfig.Entrypoint)
	}
}

func (s *DockerSuite) TestExpose(c *check.C) {
	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	exposedPort := "80"

	if err := expose(b, []string{exposedPort}, nil, ""); err != nil {
		c.Fatalf("Error should be empty, got: %s", err.Error())
	}

	if b.runConfig.ExposedPorts == nil {
		c.Fatalf("ExposedPorts should be set")
	}

	if len(b.runConfig.ExposedPorts) != 1 {
		c.Fatalf("ExposedPorts should contain only 1 element. Got %s", b.runConfig.ExposedPorts)
	}

	portsMapping, err := nat.ParsePortSpec(exposedPort)

	if err != nil {
		c.Fatalf("Error when parsing port spec: %s", err.Error())
	}

	if _, ok := b.runConfig.ExposedPorts[portsMapping[0].Port]; !ok {
		c.Fatalf("Port %s should be present. Got %s", exposedPort, b.runConfig.ExposedPorts)
	}
}

func (s *DockerSuite) TestUser(c *check.C) {
	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	userCommand := "foo"

	if err := user(b, []string{userCommand}, nil, ""); err != nil {
		c.Fatalf("Error should be empty, got: %s", err.Error())
	}

	if b.runConfig.User != userCommand {
		c.Fatalf("User should be set to %s, got %s", userCommand, b.runConfig.User)
	}
}

func (s *DockerSuite) TestVolume(c *check.C) {
	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	exposedVolume := "/foo"

	if err := volume(b, []string{exposedVolume}, nil, ""); err != nil {
		c.Fatalf("Error should be empty, got: %s", err.Error())
	}

	if b.runConfig.Volumes == nil {
		c.Fatalf("Volumes should be set")
	}

	if len(b.runConfig.Volumes) != 1 {
		c.Fatalf("Volumes should contain only 1 element. Got %s", b.runConfig.Volumes)
	}

	if _, ok := b.runConfig.Volumes[exposedVolume]; !ok {
		c.Fatalf("Volume %s should be present. Got %s", exposedVolume, b.runConfig.Volumes)
	}
}

func (s *DockerSuite) TestStopSignal(c *check.C) {
	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	signal := "SIGKILL"

	if err := stopSignal(b, []string{signal}, nil, ""); err != nil {
		c.Fatalf("Error should be empty, got: %s", err.Error())
	}

	if b.runConfig.StopSignal != signal {
		c.Fatalf("StopSignal should be set to %s, got %s", signal, b.runConfig.StopSignal)
	}
}

func (s *DockerSuite) TestArg(c *check.C) {
	buildOptions := &types.ImageBuildOptions{BuildArgs: make(map[string]string)}

	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true, allowedBuildArgs: make(map[string]bool), options: buildOptions}

	argName := "foo"
	argVal := "bar"
	argDef := fmt.Sprintf("%s=%s", argName, argVal)

	if err := arg(b, []string{argDef}, nil, ""); err != nil {
		c.Fatalf("Error should be empty, got: %s", err.Error())
	}

	allowed, ok := b.allowedBuildArgs[argName]

	if !ok {
		c.Fatalf("%s argument should be allowed as a build arg", argName)
	}

	if !allowed {
		c.Fatalf("%s argument was present in map but disallowed as a build arg", argName)
	}

	val, ok := b.options.BuildArgs[argName]

	if !ok {
		c.Fatalf("%s argument should be a build arg", argName)
	}

	if val != "bar" {
		c.Fatalf("%s argument should have default value 'bar', got %s", argName, val)
	}
}

func (s *DockerSuite) TestShell(c *check.C) {
	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	shellCmd := "powershell"

	attrs := make(map[string]bool)
	attrs["json"] = true

	if err := shell(b, []string{shellCmd}, attrs, ""); err != nil {
		c.Fatalf("Error should be empty, got: %s", err.Error())
	}

	if b.runConfig.Shell == nil {
		c.Fatalf("Shell should be set")
	}

	expectedShell := strslice.StrSlice([]string{shellCmd})

	if !compareStrSlice(expectedShell, b.runConfig.Shell) {
		c.Fatalf("Shell should be set to %s, got %s", expectedShell, b.runConfig.Shell)
	}
}
