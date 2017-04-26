package dockerfile

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/docker/docker/pkg/testutil"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type commandWithFunction struct {
	name     string
	function func(args []string) error
}

func withArgs(f dispatcher) func([]string) error {
	return func(args []string) error {
		return f(dispatchRequest{args: args, runConfig: &container.Config{}})
	}
}

func withBuilderAndArgs(builder *Builder, f dispatcher) func([]string) error {
	return func(args []string) error {
		return f(defaultDispatchReq(builder, args...))
	}
}

func defaultDispatchReq(builder *Builder, args ...string) dispatchRequest {
	return dispatchRequest{
		builder:   builder,
		args:      args,
		flags:     NewBFlags(),
		runConfig: &container.Config{},
		shlex:     NewShellLex(parser.DefaultEscapeToken),
	}
}

func newBuilderWithMockBackend() *Builder {
	b := &Builder{
		runConfig:     &container.Config{},
		options:       &types.ImageBuildOptions{},
		docker:        &MockBackend{},
		buildArgs:     newBuildArgs(make(map[string]*string)),
		disableCommit: true,
	}
	b.imageContexts = &imageContexts{b: b}
	return b
}

func TestCommandsExactlyOneArgument(t *testing.T) {
	commands := []commandWithFunction{
		{"MAINTAINER", withArgs(maintainer)},
		{"WORKDIR", withArgs(workdir)},
		{"USER", withArgs(user)},
		{"STOPSIGNAL", withArgs(stopSignal)},
	}

	for _, command := range commands {
		err := command.function([]string{})
		assert.EqualError(t, err, errExactlyOneArgument(command.name).Error())
	}
}

func TestCommandsAtLeastOneArgument(t *testing.T) {
	commands := []commandWithFunction{
		{"ENV", withArgs(env)},
		{"LABEL", withArgs(label)},
		{"ONBUILD", withArgs(onbuild)},
		{"HEALTHCHECK", withArgs(healthcheck)},
		{"EXPOSE", withArgs(expose)},
		{"VOLUME", withArgs(volume)},
	}

	for _, command := range commands {
		err := command.function([]string{})
		assert.EqualError(t, err, errAtLeastOneArgument(command.name).Error())
	}
}

func TestCommandsAtLeastTwoArguments(t *testing.T) {
	commands := []commandWithFunction{
		{"ADD", withArgs(add)},
		{"COPY", withArgs(dispatchCopy)}}

	for _, command := range commands {
		err := command.function([]string{"arg1"})
		assert.EqualError(t, err, errAtLeastTwoArguments(command.name).Error())
	}
}

func TestCommandsTooManyArguments(t *testing.T) {
	commands := []commandWithFunction{
		{"ENV", withArgs(env)},
		{"LABEL", withArgs(label)}}

	for _, command := range commands {
		err := command.function([]string{"arg1", "arg2", "arg3"})
		assert.EqualError(t, err, errTooManyArguments(command.name).Error())
	}
}

func TestCommandsBlankNames(t *testing.T) {
	builder := newBuilderWithMockBackend()
	commands := []commandWithFunction{
		{"ENV", withBuilderAndArgs(builder, env)},
		{"LABEL", withBuilderAndArgs(builder, label)},
	}

	for _, command := range commands {
		err := command.function([]string{"", ""})
		assert.EqualError(t, err, errBlankCommandNames(command.name).Error())
	}
}

func TestEnv2Variables(t *testing.T) {
	b := newBuilderWithMockBackend()

	args := []string{"var1", "val1", "var2", "val2"}
	req := defaultDispatchReq(b, args...)
	err := env(req)
	require.NoError(t, err)

	expected := []string{
		fmt.Sprintf("%s=%s", args[0], args[1]),
		fmt.Sprintf("%s=%s", args[2], args[3]),
	}
	assert.Equal(t, expected, req.runConfig.Env)
}

func TestEnvValueWithExistingRunConfigEnv(t *testing.T) {
	b := newBuilderWithMockBackend()

	args := []string{"var1", "val1"}
	req := defaultDispatchReq(b, args...)
	req.runConfig.Env = []string{"var1=old", "var2=fromenv"}
	err := env(req)
	require.NoError(t, err)

	expected := []string{
		fmt.Sprintf("%s=%s", args[0], args[1]),
		"var2=fromenv",
	}
	assert.Equal(t, expected, req.runConfig.Env)
}

func TestMaintainer(t *testing.T) {
	maintainerEntry := "Some Maintainer <maintainer@example.com>"

	b := newBuilderWithMockBackend()
	err := maintainer(defaultDispatchReq(b, maintainerEntry))
	require.NoError(t, err)
	assert.Equal(t, maintainerEntry, b.maintainer)
}

func TestLabel(t *testing.T) {
	labelName := "label"
	labelValue := "value"

	labelEntry := []string{labelName, labelValue}
	b := newBuilderWithMockBackend()
	req := defaultDispatchReq(b, labelEntry...)
	err := label(req)
	require.NoError(t, err)

	require.Contains(t, req.runConfig.Labels, labelName)
	assert.Equal(t, req.runConfig.Labels[labelName], labelValue)
}

func TestFromScratch(t *testing.T) {
	b := newBuilderWithMockBackend()
	err := from(defaultDispatchReq(b, "scratch"))

	if runtime.GOOS == "windows" {
		assert.EqualError(t, err, "Windows does not support FROM scratch")
		return
	}

	require.NoError(t, err)
	assert.Equal(t, "", b.image)
	assert.Equal(t, true, b.noBaseImage)
}

func TestFromWithArg(t *testing.T) {
	tag, expected := ":sometag", "expectedthisid"

	getImage := func(name string) (builder.Image, error) {
		assert.Equal(t, "alpine"+tag, name)
		return &mockImage{id: "expectedthisid"}, nil
	}
	b := newBuilderWithMockBackend()
	b.docker.(*MockBackend).getImageOnBuildFunc = getImage

	require.NoError(t, arg(defaultDispatchReq(b, "THETAG="+tag)))
	err := from(defaultDispatchReq(b, "alpine${THETAG}"))

	require.NoError(t, err)
	assert.Equal(t, expected, b.image)
	assert.Equal(t, expected, b.from.ImageID())
	assert.Len(t, b.buildArgs.GetAllAllowed(), 0)
	assert.Len(t, b.buildArgs.GetAllMeta(), 1)
}

func TestFromWithUndefinedArg(t *testing.T) {
	tag, expected := "sometag", "expectedthisid"

	getImage := func(name string) (builder.Image, error) {
		assert.Equal(t, "alpine", name)
		return &mockImage{id: "expectedthisid"}, nil
	}
	b := newBuilderWithMockBackend()
	b.docker.(*MockBackend).getImageOnBuildFunc = getImage
	b.options.BuildArgs = map[string]*string{"THETAG": &tag}

	err := from(defaultDispatchReq(b, "alpine${THETAG}"))
	require.NoError(t, err)
	assert.Equal(t, expected, b.image)
}

func TestOnbuildIllegalTriggers(t *testing.T) {
	triggers := []struct{ command, expectedError string }{
		{"ONBUILD", "Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed"},
		{"MAINTAINER", "MAINTAINER isn't allowed as an ONBUILD trigger"},
		{"FROM", "FROM isn't allowed as an ONBUILD trigger"}}

	for _, trigger := range triggers {
		b := newBuilderWithMockBackend()

		err := onbuild(defaultDispatchReq(b, trigger.command))
		testutil.ErrorContains(t, err, trigger.expectedError)
	}
}

func TestOnbuild(t *testing.T) {
	b := newBuilderWithMockBackend()

	req := defaultDispatchReq(b, "ADD", ".", "/app/src")
	req.original = "ONBUILD ADD . /app/src"
	req.runConfig = &container.Config{}

	err := onbuild(req)
	require.NoError(t, err)
	assert.Equal(t, "ADD . /app/src", req.runConfig.OnBuild[0])
}

func TestWorkdir(t *testing.T) {
	b := newBuilderWithMockBackend()
	workingDir := "/app"
	if runtime.GOOS == "windows" {
		workingDir = "C:\app"
	}

	req := defaultDispatchReq(b, workingDir)
	err := workdir(req)
	require.NoError(t, err)
	assert.Equal(t, workingDir, req.runConfig.WorkingDir)
}

func TestCmd(t *testing.T) {
	b := newBuilderWithMockBackend()
	command := "./executable"

	req := defaultDispatchReq(b, command)
	err := cmd(req)
	require.NoError(t, err)

	var expectedCommand strslice.StrSlice
	if runtime.GOOS == "windows" {
		expectedCommand = strslice.StrSlice(append([]string{"cmd"}, "/S", "/C", command))
	} else {
		expectedCommand = strslice.StrSlice(append([]string{"/bin/sh"}, "-c", command))
	}

	assert.Equal(t, expectedCommand, req.runConfig.Cmd)
	assert.True(t, b.cmdSet)
}

func TestHealthcheckNone(t *testing.T) {
	b := newBuilderWithMockBackend()

	req := defaultDispatchReq(b, "NONE")
	err := healthcheck(req)
	require.NoError(t, err)

	require.NotNil(t, req.runConfig.Healthcheck)
	assert.Equal(t, []string{"NONE"}, req.runConfig.Healthcheck.Test)
}

func TestHealthcheckCmd(t *testing.T) {
	b := newBuilderWithMockBackend()

	args := []string{"CMD", "curl", "-f", "http://localhost/", "||", "exit", "1"}
	req := defaultDispatchReq(b, args...)
	err := healthcheck(req)
	require.NoError(t, err)

	require.NotNil(t, req.runConfig.Healthcheck)
	expectedTest := []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"}
	assert.Equal(t, expectedTest, req.runConfig.Healthcheck.Test)
}

func TestEntrypoint(t *testing.T) {
	b := newBuilderWithMockBackend()
	entrypointCmd := "/usr/sbin/nginx"

	req := defaultDispatchReq(b, entrypointCmd)
	err := entrypoint(req)
	require.NoError(t, err)
	require.NotNil(t, req.runConfig.Entrypoint)

	var expectedEntrypoint strslice.StrSlice
	if runtime.GOOS == "windows" {
		expectedEntrypoint = strslice.StrSlice(append([]string{"cmd"}, "/S", "/C", entrypointCmd))
	} else {
		expectedEntrypoint = strslice.StrSlice(append([]string{"/bin/sh"}, "-c", entrypointCmd))
	}
	assert.Equal(t, expectedEntrypoint, req.runConfig.Entrypoint)
}

func TestExpose(t *testing.T) {
	b := newBuilderWithMockBackend()

	exposedPort := "80"
	req := defaultDispatchReq(b, exposedPort)
	err := expose(req)
	require.NoError(t, err)

	require.NotNil(t, req.runConfig.ExposedPorts)
	require.Len(t, req.runConfig.ExposedPorts, 1)

	portsMapping, err := nat.ParsePortSpec(exposedPort)
	require.NoError(t, err)
	assert.Contains(t, req.runConfig.ExposedPorts, portsMapping[0].Port)
}

func TestUser(t *testing.T) {
	b := newBuilderWithMockBackend()
	userCommand := "foo"

	req := defaultDispatchReq(b, userCommand)
	err := user(req)
	require.NoError(t, err)
	assert.Equal(t, userCommand, req.runConfig.User)
}

func TestVolume(t *testing.T) {
	b := newBuilderWithMockBackend()

	exposedVolume := "/foo"

	req := defaultDispatchReq(b, exposedVolume)
	err := volume(req)
	require.NoError(t, err)

	require.NotNil(t, req.runConfig.Volumes)
	assert.Len(t, req.runConfig.Volumes, 1)
	assert.Contains(t, req.runConfig.Volumes, exposedVolume)
}

func TestStopSignal(t *testing.T) {
	b := newBuilderWithMockBackend()
	signal := "SIGKILL"

	req := defaultDispatchReq(b, signal)
	err := stopSignal(req)
	require.NoError(t, err)
	assert.Equal(t, signal, req.runConfig.StopSignal)
}

func TestArg(t *testing.T) {
	b := newBuilderWithMockBackend()

	argName := "foo"
	argVal := "bar"
	argDef := fmt.Sprintf("%s=%s", argName, argVal)

	err := arg(defaultDispatchReq(b, argDef))
	require.NoError(t, err)

	expected := map[string]string{argName: argVal}
	assert.Equal(t, expected, b.buildArgs.GetAllAllowed())
}

func TestShell(t *testing.T) {
	b := newBuilderWithMockBackend()

	shellCmd := "powershell"
	req := defaultDispatchReq(b, shellCmd)
	req.attributes = map[string]bool{"json": true}

	err := shell(req)
	require.NoError(t, err)

	expectedShell := strslice.StrSlice([]string{shellCmd})
	assert.Equal(t, expectedShell, req.runConfig.Shell)
}

func TestParseOptInterval(t *testing.T) {
	flInterval := &Flag{
		name:     "interval",
		flagType: stringType,
		Value:    "50ns",
	}
	_, err := parseOptInterval(flInterval)
	if err == nil {
		t.Fatalf("Error should be presented for interval %s", flInterval.Value)
	}

	flInterval.Value = "1ms"
	_, err = parseOptInterval(flInterval)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}
}
