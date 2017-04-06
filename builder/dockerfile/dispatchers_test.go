package dockerfile

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/go-connections/nat"
)

type commandWithFunction struct {
	name     string
	function func(args []string) error
}

func TestCommandsExactlyOneArgument(t *testing.T) {
	commands := []commandWithFunction{
		{"MAINTAINER", func(args []string) error { return maintainer(nil, args, nil, "") }},
		{"FROM", func(args []string) error { return from(nil, args, nil, "") }},
		{"WORKDIR", func(args []string) error { return workdir(nil, args, nil, "") }},
		{"USER", func(args []string) error { return user(nil, args, nil, "") }},
		{"STOPSIGNAL", func(args []string) error { return stopSignal(nil, args, nil, "") }}}

	for _, command := range commands {
		err := command.function([]string{})

		if err == nil {
			t.Fatalf("Error should be present for %s command", command.name)
		}

		expectedError := errExactlyOneArgument(command.name)

		if err.Error() != expectedError.Error() {
			t.Fatalf("Wrong error message for %s. Got: %s. Should be: %s", command.name, err.Error(), expectedError)
		}
	}
}

func TestCommandsAtLeastOneArgument(t *testing.T) {
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
			t.Fatalf("Error should be present for %s command", command.name)
		}

		expectedError := errAtLeastOneArgument(command.name)

		if err.Error() != expectedError.Error() {
			t.Fatalf("Wrong error message for %s. Got: %s. Should be: %s", command.name, err.Error(), expectedError)
		}
	}
}

func TestCommandsAtLeastTwoArguments(t *testing.T) {
	commands := []commandWithFunction{
		{"ADD", func(args []string) error { return add(nil, args, nil, "") }},
		{"COPY", func(args []string) error { return dispatchCopy(nil, args, nil, "") }}}

	for _, command := range commands {
		err := command.function([]string{"arg1"})

		if err == nil {
			t.Fatalf("Error should be present for %s command", command.name)
		}

		expectedError := errAtLeastTwoArguments(command.name)

		if err.Error() != expectedError.Error() {
			t.Fatalf("Wrong error message for %s. Got: %s. Should be: %s", command.name, err.Error(), expectedError)
		}
	}
}

func TestCommandsTooManyArguments(t *testing.T) {
	commands := []commandWithFunction{
		{"ENV", func(args []string) error { return env(nil, args, nil, "") }},
		{"LABEL", func(args []string) error { return label(nil, args, nil, "") }}}

	for _, command := range commands {
		err := command.function([]string{"arg1", "arg2", "arg3"})

		if err == nil {
			t.Fatalf("Error should be present for %s command", command.name)
		}

		expectedError := errTooManyArguments(command.name)

		if err.Error() != expectedError.Error() {
			t.Fatalf("Wrong error message for %s. Got: %s. Should be: %s", command.name, err.Error(), expectedError)
		}
	}
}

func TestCommandseBlankNames(t *testing.T) {
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
			t.Fatalf("Error should be present for %s command", command.name)
		}

		expectedError := errBlankCommandNames(command.name)

		if err.Error() != expectedError.Error() {
			t.Fatalf("Wrong error message for %s. Got: %s. Should be: %s", command.name, err.Error(), expectedError)
		}
	}
}

func TestEnv2Variables(t *testing.T) {
	variables := []string{"var1", "val1", "var2", "val2"}

	bflags := &BFlags{}
	config := &container.Config{}

	b := &Builder{flags: bflags, runConfig: config, disableCommit: true}

	if err := env(b, variables, nil, ""); err != nil {
		t.Fatalf("Error when executing env: %s", err.Error())
	}

	expectedVar1 := fmt.Sprintf("%s=%s", variables[0], variables[1])
	expectedVar2 := fmt.Sprintf("%s=%s", variables[2], variables[3])

	if b.runConfig.Env[0] != expectedVar1 {
		t.Fatalf("Wrong env output for first variable. Got: %s. Should be: %s", b.runConfig.Env[0], expectedVar1)
	}

	if b.runConfig.Env[1] != expectedVar2 {
		t.Fatalf("Wrong env output for second variable. Got: %s, Should be: %s", b.runConfig.Env[1], expectedVar2)
	}
}

func TestMaintainer(t *testing.T) {
	maintainerEntry := "Some Maintainer <maintainer@example.com>"

	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	if err := maintainer(b, []string{maintainerEntry}, nil, ""); err != nil {
		t.Fatalf("Error when executing maintainer: %s", err.Error())
	}

	if b.maintainer != maintainerEntry {
		t.Fatalf("Maintainer in builder should be set to %s. Got: %s", maintainerEntry, b.maintainer)
	}
}

func TestLabel(t *testing.T) {
	labelName := "label"
	labelValue := "value"

	labelEntry := []string{labelName, labelValue}

	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	if err := label(b, labelEntry, nil, ""); err != nil {
		t.Fatalf("Error when executing label: %s", err.Error())
	}

	if val, ok := b.runConfig.Labels[labelName]; ok {
		if val != labelValue {
			t.Fatalf("Label %s should have value %s, had %s instead", labelName, labelValue, val)
		}
	} else {
		t.Fatalf("Label %s should be present but it is not", labelName)
	}
}

func TestFrom(t *testing.T) {
	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}
	b.imageContexts = &imageContexts{b: b}

	err := from(b, []string{"scratch"}, nil, "")

	if runtime.GOOS == "windows" {
		if err == nil {
			t.Fatal("Error not set on Windows")
		}

		expectedError := "Windows does not support FROM scratch"

		if !strings.Contains(err.Error(), expectedError) {
			t.Fatalf("Error message not correct on Windows. Should be: %s, got: %s", expectedError, err.Error())
		}
	} else {
		if err != nil {
			t.Fatalf("Error when executing from: %s", err.Error())
		}

		if b.image != "" {
			t.Fatalf("Image should be empty, got: %s", b.image)
		}

		if b.noBaseImage != true {
			t.Fatalf("Image should not have any base image, got: %v", b.noBaseImage)
		}
	}
}

func TestOnbuildIllegalTriggers(t *testing.T) {
	triggers := []struct{ command, expectedError string }{
		{"ONBUILD", "Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed"},
		{"MAINTAINER", "MAINTAINER isn't allowed as an ONBUILD trigger"},
		{"FROM", "FROM isn't allowed as an ONBUILD trigger"}}

	for _, trigger := range triggers {
		b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

		err := onbuild(b, []string{trigger.command}, nil, "")

		if err == nil {
			t.Fatal("Error should not be nil")
		}

		if !strings.Contains(err.Error(), trigger.expectedError) {
			t.Fatalf("Error message not correct. Should be: %s, got: %s", trigger.expectedError, err.Error())
		}
	}
}

func TestOnbuild(t *testing.T) {
	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	err := onbuild(b, []string{"ADD", ".", "/app/src"}, nil, "ONBUILD ADD . /app/src")

	if err != nil {
		t.Fatalf("Error should be empty, got: %s", err.Error())
	}

	expectedOnbuild := "ADD . /app/src"

	if b.runConfig.OnBuild[0] != expectedOnbuild {
		t.Fatalf("Wrong ONBUILD command. Expected: %s, got: %s", expectedOnbuild, b.runConfig.OnBuild[0])
	}
}

func TestWorkdir(t *testing.T) {
	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	workingDir := "/app"

	if runtime.GOOS == "windows" {
		workingDir = "C:\app"
	}

	err := workdir(b, []string{workingDir}, nil, "")

	if err != nil {
		t.Fatalf("Error should be empty, got: %s", err.Error())
	}

	if b.runConfig.WorkingDir != workingDir {
		t.Fatalf("WorkingDir should be set to %s, got %s", workingDir, b.runConfig.WorkingDir)
	}

}

func TestCmd(t *testing.T) {
	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	command := "./executable"

	err := cmd(b, []string{command}, nil, "")

	if err != nil {
		t.Fatalf("Error should be empty, got: %s", err.Error())
	}

	var expectedCommand strslice.StrSlice

	if runtime.GOOS == "windows" {
		expectedCommand = strslice.StrSlice(append([]string{"cmd"}, "/S", "/C", command))
	} else {
		expectedCommand = strslice.StrSlice(append([]string{"/bin/sh"}, "-c", command))
	}

	if !compareStrSlice(b.runConfig.Cmd, expectedCommand) {
		t.Fatalf("Command should be set to %s, got %s", command, b.runConfig.Cmd)
	}

	if !b.cmdSet {
		t.Fatal("Command should be marked as set")
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

func TestHealthcheckNone(t *testing.T) {
	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	if err := healthcheck(b, []string{"NONE"}, nil, ""); err != nil {
		t.Fatalf("Error should be empty, got: %s", err.Error())
	}

	if b.runConfig.Healthcheck == nil {
		t.Fatal("Healthcheck should be set, got nil")
	}

	expectedTest := strslice.StrSlice(append([]string{"NONE"}))

	if !compareStrSlice(expectedTest, b.runConfig.Healthcheck.Test) {
		t.Fatalf("Command should be set to %s, got %s", expectedTest, b.runConfig.Healthcheck.Test)
	}
}

func TestHealthcheckCmd(t *testing.T) {
	b := &Builder{flags: &BFlags{flags: make(map[string]*Flag)}, runConfig: &container.Config{}, disableCommit: true}

	if err := healthcheck(b, []string{"CMD", "curl", "-f", "http://localhost/", "||", "exit", "1"}, nil, ""); err != nil {
		t.Fatalf("Error should be empty, got: %s", err.Error())
	}

	if b.runConfig.Healthcheck == nil {
		t.Fatal("Healthcheck should be set, got nil")
	}

	expectedTest := strslice.StrSlice(append([]string{"CMD-SHELL"}, "curl -f http://localhost/ || exit 1"))

	if !compareStrSlice(expectedTest, b.runConfig.Healthcheck.Test) {
		t.Fatalf("Command should be set to %s, got %s", expectedTest, b.runConfig.Healthcheck.Test)
	}
}

func TestEntrypoint(t *testing.T) {
	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	entrypointCmd := "/usr/sbin/nginx"

	if err := entrypoint(b, []string{entrypointCmd}, nil, ""); err != nil {
		t.Fatalf("Error should be empty, got: %s", err.Error())
	}

	if b.runConfig.Entrypoint == nil {
		t.Fatal("Entrypoint should be set")
	}

	var expectedEntrypoint strslice.StrSlice

	if runtime.GOOS == "windows" {
		expectedEntrypoint = strslice.StrSlice(append([]string{"cmd"}, "/S", "/C", entrypointCmd))
	} else {
		expectedEntrypoint = strslice.StrSlice(append([]string{"/bin/sh"}, "-c", entrypointCmd))
	}

	if !compareStrSlice(expectedEntrypoint, b.runConfig.Entrypoint) {
		t.Fatalf("Entrypoint command should be set to %s, got %s", expectedEntrypoint, b.runConfig.Entrypoint)
	}
}

func TestExpose(t *testing.T) {
	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	exposedPort := "80"

	if err := expose(b, []string{exposedPort}, nil, ""); err != nil {
		t.Fatalf("Error should be empty, got: %s", err.Error())
	}

	if b.runConfig.ExposedPorts == nil {
		t.Fatal("ExposedPorts should be set")
	}

	if len(b.runConfig.ExposedPorts) != 1 {
		t.Fatalf("ExposedPorts should contain only 1 element. Got %s", b.runConfig.ExposedPorts)
	}

	portsMapping, err := nat.ParsePortSpec(exposedPort)

	if err != nil {
		t.Fatalf("Error when parsing port spec: %s", err.Error())
	}

	if _, ok := b.runConfig.ExposedPorts[portsMapping[0].Port]; !ok {
		t.Fatalf("Port %s should be present. Got %s", exposedPort, b.runConfig.ExposedPorts)
	}
}

func TestUser(t *testing.T) {
	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	userCommand := "foo"

	if err := user(b, []string{userCommand}, nil, ""); err != nil {
		t.Fatalf("Error should be empty, got: %s", err.Error())
	}

	if b.runConfig.User != userCommand {
		t.Fatalf("User should be set to %s, got %s", userCommand, b.runConfig.User)
	}
}

func TestVolume(t *testing.T) {
	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	exposedVolume := "/foo"

	if err := volume(b, []string{exposedVolume}, nil, ""); err != nil {
		t.Fatalf("Error should be empty, got: %s", err.Error())
	}

	if b.runConfig.Volumes == nil {
		t.Fatal("Volumes should be set")
	}

	if len(b.runConfig.Volumes) != 1 {
		t.Fatalf("Volumes should contain only 1 element. Got %s", b.runConfig.Volumes)
	}

	if _, ok := b.runConfig.Volumes[exposedVolume]; !ok {
		t.Fatalf("Volume %s should be present. Got %s", exposedVolume, b.runConfig.Volumes)
	}
}

func TestStopSignal(t *testing.T) {
	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	signal := "SIGKILL"

	if err := stopSignal(b, []string{signal}, nil, ""); err != nil {
		t.Fatalf("Error should be empty, got: %s", err.Error())
	}

	if b.runConfig.StopSignal != signal {
		t.Fatalf("StopSignal should be set to %s, got %s", signal, b.runConfig.StopSignal)
	}
}

func TestArg(t *testing.T) {
	// This is a bad test that tests implementation details and not at
	// any features of the builder. Replace or remove.
	buildOptions := &types.ImageBuildOptions{BuildArgs: make(map[string]*string)}

	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true, allowedBuildArgs: make(map[string]*string), allBuildArgs: make(map[string]struct{}), options: buildOptions}

	argName := "foo"
	argVal := "bar"
	argDef := fmt.Sprintf("%s=%s", argName, argVal)

	if err := arg(b, []string{argDef}, nil, ""); err != nil {
		t.Fatalf("Error should be empty, got: %s", err.Error())
	}

	value, ok := b.getBuildArg(argName)

	if !ok {
		t.Fatalf("%s argument should be a build arg", argName)
	}

	if value != "bar" {
		t.Fatalf("%s argument should have default value 'bar', got %s", argName, value)
	}
}

func TestShell(t *testing.T) {
	b := &Builder{flags: &BFlags{}, runConfig: &container.Config{}, disableCommit: true}

	shellCmd := "powershell"

	attrs := make(map[string]bool)
	attrs["json"] = true

	if err := shell(b, []string{shellCmd}, attrs, ""); err != nil {
		t.Fatalf("Error should be empty, got: %s", err.Error())
	}

	if b.runConfig.Shell == nil {
		t.Fatal("Shell should be set")
	}

	expectedShell := strslice.StrSlice([]string{shellCmd})

	if !compareStrSlice(expectedShell, b.runConfig.Shell) {
		t.Fatalf("Shell should be set to %s, got %s", expectedShell, b.runConfig.Shell)
	}
}
