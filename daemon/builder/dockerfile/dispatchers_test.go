package dockerfile

import (
	"bytes"
	"context"
	"fmt"
	"net/netip"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/moby/buildkit/frontend/dockerfile/shell"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/v2/daemon/builder"
	"github.com/moby/moby/v2/daemon/internal/image"
	"github.com/moby/moby/v2/daemon/pkg/oci"
	"github.com/moby/moby/v2/daemon/server/backend"
	"github.com/moby/moby/v2/daemon/server/buildbackend"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func newBuilderWithMockBackend(t *testing.T) *Builder {
	t.Helper()
	mockBackend := &MockBackend{}
	ctx := context.Background()

	imageProber, err := newImageProber(ctx, mockBackend, nil, false)
	assert.NilError(t, err, "Could not create image prober")

	b := &Builder{
		options:       &buildbackend.BuildOptions{},
		docker:        mockBackend,
		Stdout:        new(bytes.Buffer),
		disableCommit: true,
		imageSources: newImageSources(builderOptions{
			Options: &buildbackend.BuildOptions{},
			Backend: mockBackend,
		}),
		imageProber:      imageProber,
		containerManager: newContainerManager(mockBackend),
	}
	return b
}

func TestEnv2Variables(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '\\', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())
	envCommand := &instructions.EnvCommand{
		Env: instructions.KeyValuePairs{
			instructions.KeyValuePair{Key: "var1", Value: "val1"},
			instructions.KeyValuePair{Key: "var2", Value: "val2"},
		},
	}
	err := dispatch(t.Context(), sb, envCommand)
	assert.NilError(t, err)

	expected := []string{
		"var1=val1",
		"var2=val2",
	}
	assert.Check(t, is.DeepEqual(sb.state.runConfig.Env, expected))
}

func TestEnvValueWithExistingRunConfigEnv(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '\\', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())
	sb.state.runConfig.Env = []string{"var1=old", "var2=fromenv"}
	envCommand := &instructions.EnvCommand{
		Env: instructions.KeyValuePairs{
			instructions.KeyValuePair{Key: "var1", Value: "val1"},
		},
	}
	err := dispatch(t.Context(), sb, envCommand)
	assert.NilError(t, err)
	expected := []string{
		"var1=val1",
		"var2=fromenv",
	}
	assert.Check(t, is.DeepEqual(sb.state.runConfig.Env, expected))
}

func TestMaintainer(t *testing.T) {
	maintainerEntry := "Some Maintainer <maintainer@example.com>"
	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '\\', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())
	cmd := &instructions.MaintainerCommand{Maintainer: maintainerEntry}
	err := dispatch(t.Context(), sb, cmd)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(sb.state.maintainer, maintainerEntry))
}

func TestLabel(t *testing.T) {
	labelName := "label"
	labelValue := "value"

	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '\\', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())
	cmd := &instructions.LabelCommand{
		Labels: instructions.KeyValuePairs{
			instructions.KeyValuePair{Key: labelName, Value: labelValue},
		},
	}
	err := dispatch(t.Context(), sb, cmd)
	assert.NilError(t, err)

	assert.Assert(t, is.Contains(sb.state.runConfig.Labels, labelName))
	assert.Check(t, is.Equal(sb.state.runConfig.Labels[labelName], labelValue))
}

func TestFromScratch(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '\\', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())
	cmd := &instructions.Stage{
		BaseName: "scratch",
	}
	err := initializeStage(t.Context(), sb, cmd)

	if runtime.GOOS == "windows" {
		assert.Check(t, is.Error(err, "Windows does not support FROM scratch"))
		return
	}

	assert.NilError(t, err)
	assert.Check(t, sb.state.hasFromImage())
	assert.Check(t, is.Equal(sb.state.imageID, ""))
	// TODO(thaJeztah): use github.com/moby/buildkit/util/system.DefaultPathEnv() once https://github.com/moby/buildkit/pull/3158 is resolved.
	expected := []string{"PATH=" + oci.DefaultPathEnv(runtime.GOOS)}
	assert.Check(t, is.DeepEqual(sb.state.runConfig.Env, expected))
}

func TestFromWithArg(t *testing.T) {
	tag, expected := ":sometag", "expectedthisid"

	getImage := func(name string) (builder.Image, builder.ROLayer, error) {
		assert.Check(t, is.Equal(name, "alpine"+tag))
		return &mockImage{id: "expectedthisid"}, nil, nil
	}
	b := newBuilderWithMockBackend(t)
	b.docker.(*MockBackend).getImageFunc = getImage
	args := NewBuildArgs(make(map[string]*string))

	val := "sometag"
	metaArg := instructions.ArgCommand{Args: []instructions.KeyValuePairOptional{{
		Key:   "THETAG",
		Value: &val,
	}}}
	cmd := &instructions.Stage{
		BaseName: "alpine:${THETAG}",
	}
	err := processMetaArg(metaArg, shell.NewLex('\\'), args)

	sb := newDispatchRequest(b, '\\', nil, args, newStagesBuildResults())
	assert.NilError(t, err)
	err = initializeStage(t.Context(), sb, cmd)
	assert.NilError(t, err)

	assert.Check(t, is.Equal(sb.state.imageID, expected))
	assert.Check(t, is.Equal(sb.state.baseImage.ImageID(), expected))
	assert.Check(t, is.Len(sb.state.buildArgs.GetAllAllowed(), 0))
	assert.Check(t, is.Len(sb.state.buildArgs.GetAllMeta(), 1))
}

func TestFromWithArgButBuildArgsNotGiven(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	args := NewBuildArgs(make(map[string]*string))

	metaArg := instructions.ArgCommand{}
	cmd := &instructions.Stage{
		BaseName: "${THETAG}",
	}
	err := processMetaArg(metaArg, shell.NewLex('\\'), args)

	sb := newDispatchRequest(b, '\\', nil, args, newStagesBuildResults())
	assert.NilError(t, err)
	err = initializeStage(t.Context(), sb, cmd)
	assert.Error(t, err, "base name (${THETAG}) should not be blank")
}

func TestFromWithUndefinedArg(t *testing.T) {
	tag, expected := "sometag", "expectedthisid"

	getImage := func(name string) (builder.Image, builder.ROLayer, error) {
		assert.Check(t, is.Equal(name, "alpine"))
		return &mockImage{id: "expectedthisid"}, nil, nil
	}
	b := newBuilderWithMockBackend(t)
	b.docker.(*MockBackend).getImageFunc = getImage
	sb := newDispatchRequest(b, '\\', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())

	b.options.BuildArgs = map[string]*string{"THETAG": &tag}

	cmd := &instructions.Stage{
		BaseName: "alpine${THETAG}",
	}
	err := initializeStage(t.Context(), sb, cmd)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(sb.state.imageID, expected))
}

func TestFromMultiStageWithNamedStage(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	firstFrom := &instructions.Stage{BaseName: "someimg", Name: "base"}
	secondFrom := &instructions.Stage{BaseName: "base"}
	previousResults := newStagesBuildResults()
	firstSB := newDispatchRequest(b, '\\', nil, NewBuildArgs(make(map[string]*string)), previousResults)
	secondSB := newDispatchRequest(b, '\\', nil, NewBuildArgs(make(map[string]*string)), previousResults)
	err := initializeStage(t.Context(), firstSB, firstFrom)
	assert.NilError(t, err)
	assert.Check(t, firstSB.state.hasFromImage())
	previousResults.indexed["base"] = firstSB.state.runConfig
	previousResults.flat = append(previousResults.flat, firstSB.state.runConfig)
	err = initializeStage(t.Context(), secondSB, secondFrom)
	assert.NilError(t, err)
	assert.Check(t, secondSB.state.hasFromImage())
}

func TestOnbuild(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '\\', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())
	cmd := &instructions.OnbuildCommand{
		Expression: "ADD . /app/src",
	}
	err := dispatch(t.Context(), sb, cmd)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(sb.state.runConfig.OnBuild[0], "ADD . /app/src"))
}

func TestWorkdir(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '`', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())
	sb.state.baseImage = &mockImage{}
	workingDir := "/app"
	if runtime.GOOS == "windows" {
		workingDir = "C:\\app"
	}
	cmd := &instructions.WorkdirCommand{
		Path: workingDir,
	}

	err := dispatch(t.Context(), sb, cmd)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(sb.state.runConfig.WorkingDir, workingDir))
}

func TestCmd(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '`', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())
	sb.state.baseImage = &mockImage{}

	err := dispatch(t.Context(), sb, &instructions.CmdCommand{
		ShellDependantCmdLine: instructions.ShellDependantCmdLine{
			CmdLine:      []string{"./executable"},
			PrependShell: true,
		},
	})
	assert.NilError(t, err)

	var expectedCommand []string
	if runtime.GOOS == "windows" {
		expectedCommand = []string{"cmd", "/S", "/C", "./executable"}
	} else {
		expectedCommand = []string{"/bin/sh", "-c", "./executable"}
	}

	assert.Check(t, is.DeepEqual(sb.state.runConfig.Cmd, expectedCommand))
	assert.Check(t, sb.state.cmdSet)
}

func TestHealthcheckNone(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '`', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())
	cmd := &instructions.HealthCheckCommand{
		Health: &container.HealthConfig{
			Test: []string{"NONE"},
		},
	}
	err := dispatch(t.Context(), sb, cmd)
	assert.NilError(t, err)

	assert.Assert(t, sb.state.runConfig.Healthcheck != nil)
	assert.Check(t, is.DeepEqual(sb.state.runConfig.Healthcheck.Test, []string{"NONE"}))
}

func TestHealthcheckCmd(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '`', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())
	expectedTest := []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"}
	cmd := &instructions.HealthCheckCommand{
		Health: &container.HealthConfig{
			Test: expectedTest,
		},
	}
	err := dispatch(t.Context(), sb, cmd)
	assert.NilError(t, err)

	assert.Assert(t, sb.state.runConfig.Healthcheck != nil)
	assert.Check(t, is.DeepEqual(sb.state.runConfig.Healthcheck.Test, expectedTest))
}

func TestEntrypoint(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '`', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())
	sb.state.baseImage = &mockImage{}

	err := dispatch(t.Context(), sb, &instructions.EntrypointCommand{
		ShellDependantCmdLine: instructions.ShellDependantCmdLine{
			CmdLine:      []string{"/usr/sbin/nginx"},
			PrependShell: true,
		},
	})
	assert.NilError(t, err)

	var expectedEntrypoint []string
	if runtime.GOOS == "windows" {
		expectedEntrypoint = []string{"cmd", "/S", "/C", "/usr/sbin/nginx"}
	} else {
		expectedEntrypoint = []string{"/bin/sh", "-c", "/usr/sbin/nginx"}
	}
	assert.Check(t, is.DeepEqual(sb.state.runConfig.Entrypoint, expectedEntrypoint))
}

func TestExpose(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '`', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())

	exposedPort := "80"
	cmd := &instructions.ExposeCommand{
		Ports: []string{exposedPort},
	}
	err := dispatch(t.Context(), sb, cmd)
	assert.NilError(t, err)

	assert.Assert(t, sb.state.runConfig.ExposedPorts != nil)
	assert.Assert(t, is.Len(sb.state.runConfig.ExposedPorts, 1))

	assert.Check(t, is.Contains(sb.state.runConfig.ExposedPorts, network.MustParsePort("80/tcp")))
}

func TestUser(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '`', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())

	cmd := &instructions.UserCommand{
		User: "test",
	}
	err := dispatch(t.Context(), sb, cmd)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(sb.state.runConfig.User, "test"))
}

func TestVolume(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '`', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())

	exposedVolume := "/foo"

	cmd := &instructions.VolumeCommand{
		Volumes: []string{exposedVolume},
	}
	err := dispatch(t.Context(), sb, cmd)
	assert.NilError(t, err)
	assert.Assert(t, sb.state.runConfig.Volumes != nil)
	assert.Check(t, is.Len(sb.state.runConfig.Volumes, 1))
	assert.Check(t, is.Contains(sb.state.runConfig.Volumes, exposedVolume))
}

func TestStopSignal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not support stopsignal")
		return
	}
	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '`', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())
	sb.state.baseImage = &mockImage{}
	const signal = "SIGKILL"

	cmd := &instructions.StopSignalCommand{
		Signal: signal,
	}
	err := dispatch(t.Context(), sb, cmd)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(sb.state.runConfig.StopSignal, signal))
}

func TestArg(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '`', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())

	argName := "foo"
	argVal := "bar"
	cmd := &instructions.ArgCommand{Args: []instructions.KeyValuePairOptional{{Key: argName, Value: &argVal}}}
	err := dispatch(t.Context(), sb, cmd)
	assert.NilError(t, err)

	expected := map[string]string{argName: argVal}
	assert.Check(t, is.DeepEqual(sb.state.buildArgs.GetAllAllowed(), expected))
}

func TestShell(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '`', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())

	shellCmd := []string{"powershell"}
	err := dispatch(t.Context(), sb, &instructions.ShellCommand{
		Shell: shellCmd,
	})
	assert.NilError(t, err)

	expected := shellCmd
	assert.Check(t, is.DeepEqual(sb.state.runConfig.Shell, expected))
}

func TestPrependEnvOnCmd(t *testing.T) {
	buildArgs := NewBuildArgs(nil)
	buildArgs.AddArg("NO_PROXY", nil)

	args := []string{"sorted=nope", "args=not", "http_proxy=foo", "NO_PROXY=YA"}
	cmd := []string{"foo", "bar"}
	cmdWithEnv := prependEnvOnCmd(buildArgs, args, cmd)
	expected := []string{
		"|3", "NO_PROXY=YA", "args=not", "sorted=nope", "foo", "bar",
	}
	assert.Check(t, is.DeepEqual(cmdWithEnv, expected))
}

func TestRunWithBuildArgs(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	args := NewBuildArgs(make(map[string]*string))
	args.argsFromOptions["HTTP_PROXY"] = strPtr("FOO")
	b.disableCommit = false
	sb := newDispatchRequest(b, '`', nil, args, newStagesBuildResults())

	runConfig := &container.Config{}
	origCmd := []string{"cmd", "in", "from", "image"}

	var cmdWithShell []string
	if runtime.GOOS == "windows" {
		cmdWithShell = []string{strings.Join(append(getShell(runConfig, runtime.GOOS), []string{"echo foo"}...), " ")}
	} else {
		cmdWithShell = append(getShell(runConfig, runtime.GOOS), "echo foo")
	}

	envVars := []string{"|1", "one=two"}
	cachedCmd := append(envVars, cmdWithShell...)

	imageCache := &mockImageCache{
		getCacheFunc: func(parentID string, cfg *container.Config) (string, error) {
			// Check the runConfig.Cmd sent to probeCache()
			assert.Check(t, is.DeepEqual(cfg.Cmd, cachedCmd))
			assert.Check(t, is.Nil(cfg.Entrypoint))
			return "", nil
		},
	}

	mockBackend := b.docker.(*MockBackend)
	mockBackend.makeImageCacheFunc = func(_ []string) builder.ImageCache {
		return imageCache
	}

	prober, err := newImageProber(t.Context(), mockBackend, nil, false)
	assert.NilError(t, err, "Could not create image prober")
	b.imageProber = prober

	mockBackend.getImageFunc = func(_ string) (builder.Image, builder.ROLayer, error) {
		return &mockImage{
			id:     "abcdef",
			config: &container.Config{Cmd: origCmd},
		}, nil, nil
	}
	mockBackend.containerCreateFunc = func(config backend.ContainerCreateConfig) (container.CreateResponse, error) {
		// Check the runConfig.Cmd sent to create()
		assert.Check(t, is.DeepEqual(config.Config.Cmd, cmdWithShell))
		assert.Check(t, is.Contains(config.Config.Env, "one=two"))
		assert.Check(t, is.DeepEqual(config.Config.Entrypoint, []string{""}))
		return container.CreateResponse{ID: "12345"}, nil
	}
	mockBackend.commitFunc = func(cfg backend.CommitConfig) (image.ID, error) {
		// Check the runConfig.Cmd sent to commit()
		assert.Check(t, is.DeepEqual(cfg.Config.Cmd, origCmd))
		assert.Check(t, is.DeepEqual(cfg.ContainerConfig.Cmd, cachedCmd))
		assert.Check(t, is.Nil(cfg.Config.Entrypoint))
		return "", nil
	}
	from := &instructions.Stage{BaseName: "abcdef"}
	err = initializeStage(t.Context(), sb, from)
	assert.NilError(t, err)
	sb.state.buildArgs.AddArg("one", strPtr("two"))

	// This is hugely annoying. On the Windows side, it relies on the
	// RunCommand being able to emit String() and Name() (as implemented by
	// withNameAndCode). Unfortunately, that is internal, and no way to directly
	// set. However, we can fortunately use ParseInstruction in the instructions
	// package to parse a fake node which can be used as our instructions.RunCommand
	// instead.
	node := &parser.Node{
		Original: `RUN echo foo`,
		Value:    "run",
	}
	runint, err := instructions.ParseInstruction(node)
	assert.NilError(t, err)
	runinst := runint.(*instructions.RunCommand)
	runinst.CmdLine = []string{"echo foo"}
	runinst.PrependShell = true

	assert.NilError(t, dispatch(t.Context(), sb, runinst))

	// Check that runConfig.Cmd has not been modified by run
	assert.Check(t, is.DeepEqual(sb.state.runConfig.Cmd, origCmd))
}

func TestRunIgnoresHealthcheck(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	args := NewBuildArgs(make(map[string]*string))
	sb := newDispatchRequest(b, '`', nil, args, newStagesBuildResults())
	b.disableCommit = false

	origCmd := []string{"cmd", "in", "from", "image"}

	imageCache := &mockImageCache{
		getCacheFunc: func(parentID string, cfg *container.Config) (string, error) {
			return "", nil
		},
	}

	mockBackend := b.docker.(*MockBackend)
	mockBackend.makeImageCacheFunc = func(_ []string) builder.ImageCache {
		return imageCache
	}
	imgProber, err := newImageProber(t.Context(), mockBackend, nil, false)
	assert.NilError(t, err, "Could not create image prober")

	b.imageProber = imgProber
	mockBackend.getImageFunc = func(_ string) (builder.Image, builder.ROLayer, error) {
		return &mockImage{
			id:     "abcdef",
			config: &container.Config{Cmd: origCmd},
		}, nil, nil
	}
	mockBackend.containerCreateFunc = func(config backend.ContainerCreateConfig) (container.CreateResponse, error) {
		return container.CreateResponse{ID: "12345"}, nil
	}
	mockBackend.commitFunc = func(cfg backend.CommitConfig) (image.ID, error) {
		return "", nil
	}
	from := &instructions.Stage{BaseName: "abcdef"}
	err = initializeStage(t.Context(), sb, from)
	assert.NilError(t, err)

	expectedTest := []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"}
	healthint, err := instructions.ParseInstruction(&parser.Node{
		Original: `HEALTHCHECK CMD curl -f http://localhost/ || exit 1`,
		Value:    "healthcheck",
		Next: &parser.Node{
			Value: "cmd",
			Next: &parser.Node{
				Value: `curl -f http://localhost/ || exit 1`,
			},
		},
	})
	assert.NilError(t, err)
	cmd := healthint.(*instructions.HealthCheckCommand)

	assert.NilError(t, dispatch(t.Context(), sb, cmd))
	assert.Assert(t, sb.state.runConfig.Healthcheck != nil)

	mockBackend.containerCreateFunc = func(config backend.ContainerCreateConfig) (container.CreateResponse, error) {
		// Check the Healthcheck is disabled.
		assert.Check(t, is.DeepEqual(config.Config.Healthcheck.Test, []string{"NONE"}))
		return container.CreateResponse{ID: "123456"}, nil
	}

	sb.state.buildArgs.AddArg("one", strPtr("two"))
	runint, err := instructions.ParseInstruction(&parser.Node{Original: `RUN echo foo`, Value: "run"})
	assert.NilError(t, err)
	run := runint.(*instructions.RunCommand)
	run.PrependShell = true

	assert.NilError(t, dispatch(t.Context(), sb, run))
	assert.Check(t, is.DeepEqual(sb.state.runConfig.Healthcheck.Test, expectedTest))
}

func TestDispatchUnsupportedOptions(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '`', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())
	sb.state.baseImage = &mockImage{}
	sb.state.operatingSystem = runtime.GOOS

	t.Run("ADD with chmod", func(t *testing.T) {
		cmd := &instructions.AddCommand{
			SourcesAndDest: instructions.SourcesAndDest{
				SourcePaths: []string{"."},
				DestPath:    ".",
			},
			Chmod: "0655",
		}
		err := dispatch(t.Context(), sb, cmd)
		assert.Error(t, err, "the --chmod option requires BuildKit. Refer to https://docs.docker.com/go/buildkit/ to learn how to build images with BuildKit enabled")
	})

	t.Run("COPY with chmod", func(t *testing.T) {
		cmd := &instructions.CopyCommand{
			SourcesAndDest: instructions.SourcesAndDest{
				SourcePaths: []string{"."},
				DestPath:    ".",
			},
			Chmod: "0655",
		}
		err := dispatch(t.Context(), sb, cmd)
		assert.Error(t, err, "the --chmod option requires BuildKit. Refer to https://docs.docker.com/go/buildkit/ to learn how to build images with BuildKit enabled")
	})

	t.Run("RUN with unsupported options", func(t *testing.T) {
		runint, err := instructions.ParseInstruction(&parser.Node{Original: `RUN echo foo`, Value: "run"})
		assert.NilError(t, err)
		cmd := runint.(*instructions.RunCommand)

		// classic builder "RUN" currently doesn't support any flags, but testing
		// both "known" flags and "bogus" flags for completeness, and in case
		// one or more of these flags will be supported in future
		for _, f := range []string{"mount", "network", "security", "any-flag"} {
			cmd.FlagsUsed = []string{f}
			err := dispatch(t.Context(), sb, cmd)
			assert.Error(t, err, fmt.Sprintf("the --%s option requires BuildKit. Refer to https://docs.docker.com/go/buildkit/ to learn how to build images with BuildKit enabled", f))
		}
	})
}

// Copied and modified from https://github.com/docker/go-connections/blob/c296721c0d56d3acad2973376ded214103a4fd2e/nat/nat_test.go#L390-L499
func TestParsePortSpecs(t *testing.T) {
	var (
		portSet    network.PortSet
		bindingMap network.PortMap
		err        error
	)

	tcp1234 := network.MustParsePort("1234/tcp")
	udp2345 := network.MustParsePort("2345/udp")
	sctp3456 := network.MustParsePort("3456/sctp")

	portSet, bindingMap, err = parsePortSpecs([]string{tcp1234.String(), udp2345.String(), sctp3456.String()})
	if err != nil {
		t.Fatalf("Error while processing ParsePortSpecs: %s", err)
	}

	if _, ok := portSet[tcp1234]; !ok {
		t.Fatal("1234/tcp was not parsed properly")
	}

	if _, ok := portSet[udp2345]; !ok {
		t.Fatal("2345/udp was not parsed properly")
	}

	if _, ok := portSet[sctp3456]; !ok {
		t.Fatal("3456/sctp was not parsed properly")
	}

	for portSpec, bindings := range bindingMap {
		if len(bindings) != 1 {
			t.Fatalf("%s should have exactly one binding", portSpec)
		}

		if bindings[0].HostIP.IsValid() {
			t.Fatalf("HostIP should not be set for %s", portSpec)
		}

		if bindings[0].HostPort != "" {
			t.Fatalf("HostPort should not be set for %s", portSpec)
		}
	}

	portSet, bindingMap, err = parsePortSpecs([]string{"1234:1234/tcp", "2345:2345/udp", "3456:3456/sctp"})
	if err != nil {
		t.Fatalf("Error while processing ParsePortSpecs: %s", err)
	}

	if _, ok := portSet[tcp1234]; !ok {
		t.Fatal("1234/tcp was not parsed properly")
	}

	if _, ok := portSet[udp2345]; !ok {
		t.Fatal("2345/udp was not parsed properly")
	}

	if _, ok := portSet[sctp3456]; !ok {
		t.Fatal("3456/sctp was not parsed properly")
	}

	for portSpec, bindings := range bindingMap {
		_, port := splitProtoPort(portSpec.String())

		if len(bindings) != 1 {
			t.Fatalf("%s should have exactly one binding", portSpec)
		}

		if bindings[0].HostIP.IsValid() {
			t.Fatalf("HostIP should not be set for %s", portSpec)
		}

		if bindings[0].HostPort != port {
			t.Fatalf("HostPort(%s) should be %s for %s", bindings[0].HostPort, port, portSpec)
		}
	}

	portSet, bindingMap, err = parsePortSpecs([]string{"0.0.0.0:1234:1234/tcp", "0.0.0.0:2345:2345/udp", "0.0.0.0:3456:3456/sctp"})
	if err != nil {
		t.Fatalf("Error while processing ParsePortSpecs: %s", err)
	}

	if _, ok := portSet[tcp1234]; !ok {
		t.Fatal("1234/tcp was not parsed properly")
	}

	if _, ok := portSet[udp2345]; !ok {
		t.Fatal("2345/udp was not parsed properly")
	}

	if _, ok := portSet[sctp3456]; !ok {
		t.Fatal("3456/sctp was not parsed properly")
	}

	for portSpec, bindings := range bindingMap {
		_, port := splitProtoPort(portSpec.String())

		if len(bindings) != 1 {
			t.Fatalf("%s should have exactly one binding", portSpec)
		}

		if bindings[0].HostIP != netip.IPv4Unspecified() {
			t.Fatalf("HostIP is not 0.0.0.0 for %s", portSpec)
		}

		if bindings[0].HostPort != port {
			t.Fatalf("HostPort should be %s for %s", port, portSpec)
		}
	}

	_, _, err = parsePortSpecs([]string{"localhost:1234:1234/tcp"})
	if err == nil {
		t.Fatal("Received no error while trying to parse a hostname instead of ip")
	}
}

// Copied from https://github.com/docker/go-connections/blob/c296721c0d56d3acad2973376ded214103a4fd2e/nat/nat_test.go#L244-L274
func TestParsePortSpecEmptyContainerPort(t *testing.T) {
	tests := []struct {
		name     string
		spec     string
		expError string
	}{
		{
			name:     "empty spec",
			spec:     "",
			expError: `no port specified: <empty>`,
		},
		{
			name:     "empty container port",
			spec:     `0.0.0.0:1234-1235:/tcp`,
			expError: `no port specified: 0.0.0.0:1234-1235:/tcp<empty>`,
		},
		{
			name:     "empty container port and proto",
			spec:     `0.0.0.0:1234-1235:`,
			expError: `no port specified: 0.0.0.0:1234-1235:<empty>`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parsePortSpec(tc.spec)
			if err == nil || err.Error() != tc.expError {
				t.Fatalf("expected %v, got: %v", tc.expError, err)
			}
		})
	}
}

// Copied and modified from https://github.com/docker/go-connections/blob/c296721c0d56d3acad2973376ded214103a4fd2e/nat/nat_test.go#L276-L302
func TestParsePortSpecFull(t *testing.T) {
	portMappings, err := parsePortSpec("0.0.0.0:1234-1235:3333-3334/tcp")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	expected := []network.PortMap{
		{
			network.MustParsePort("3333/tcp"): []network.PortBinding{
				{
					HostIP:   netip.IPv4Unspecified(),
					HostPort: "1234",
				},
			},
		},
		{
			network.MustParsePort("3334/tcp"): []network.PortBinding{
				{
					HostIP:   netip.IPv4Unspecified(),
					HostPort: "1235",
				},
			},
		},
	}

	if !reflect.DeepEqual(expected, portMappings) {
		t.Fatalf("wrong port mappings: got=%v, want=%v", portMappings, expected)
	}
}

// Copied and modified from https://github.com/docker/go-connections/blob/c296721c0d56d3acad2973376ded214103a4fd2e/nat/nat_test.go#L304-L388
func TestPartPortSpecIPV6(t *testing.T) {
	type test struct {
		name     string
		spec     string
		expected []network.PortMap
	}
	cases := []test{
		{
			name: "square angled IPV6 without host port",
			spec: "[2001:4860:0:2001::68]::333",
			expected: []network.PortMap{
				{
					network.MustParsePort("333/tcp"): []network.PortBinding{
						{
							HostIP:   netip.MustParseAddr("2001:4860:0:2001::68"),
							HostPort: "",
						},
					},
				},
			},
		},
		{
			name: "square angled IPV6 with host port",
			spec: "[::1]:80:80",
			expected: []network.PortMap{
				{
					network.MustParsePort("80/tcp"): []network.PortBinding{
						{
							HostIP:   netip.IPv6Loopback(),
							HostPort: "80",
						},
					},
				},
			},
		},
		{
			name: "IPV6 without host port",
			spec: "2001:4860:0:2001::68::333",
			expected: []network.PortMap{
				{
					network.MustParsePort("333/tcp"): []network.PortBinding{
						{
							HostIP:   netip.MustParseAddr("2001:4860:0:2001::68"),
							HostPort: "",
						},
					},
				},
			},
		},
		{
			name: "IPV6 with host port",
			spec: "::1:80:80",
			expected: []network.PortMap{
				{
					network.MustParsePort("80/tcp"): []network.PortBinding{
						{
							HostIP:   netip.IPv6Loopback(),
							HostPort: "80",
						},
					},
				},
			},
		},
		{
			name: ":: IPV6, without host port",
			spec: "::::80",
			expected: []network.PortMap{
				{
					network.MustParsePort("80/tcp"): []network.PortBinding{
						{
							HostIP:   netip.IPv6Unspecified(),
							HostPort: "",
						},
					},
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			portMappings, err := parsePortSpec(c.spec)
			if err != nil {
				t.Fatalf("expected nil error, got: %v", err)
			}
			if !reflect.DeepEqual(c.expected, portMappings) {
				t.Fatalf("wrong port mappings: got=%v, want=%v", portMappings, c.expected)
			}
		})
	}
}

// Copied and modified from https://github.com/docker/go-connections/blob/c296721c0d56d3acad2973376ded214103a4fd2e/nat/nat_test.go#L501-L600
func TestParsePortSpecsWithRange(t *testing.T) {
	var (
		portSet    network.PortSet
		bindingMap network.PortMap
		err        error
	)

	portSet, bindingMap, err = parsePortSpecs([]string{"1234-1236/tcp", "2345-2347/udp", "3456-3458/sctp"})
	if err != nil {
		t.Fatalf("Error while processing ParsePortSpecs: %s", err)
	}

	if _, ok := portSet[network.MustParsePort("1235/tcp")]; !ok {
		t.Fatal("1234-1236/tcp was not parsed properly")
	}

	if _, ok := portSet[network.MustParsePort("2346/udp")]; !ok {
		t.Fatal("2345-2347/udp was not parsed properly")
	}

	if _, ok := portSet[network.MustParsePort("3456/sctp")]; !ok {
		t.Fatal("3456-3458/sctp was not parsed properly")
	}

	for portSpec, bindings := range bindingMap {
		if len(bindings) != 1 {
			t.Fatalf("%s should have exactly one binding", portSpec)
		}

		if bindings[0].HostIP.IsValid() {
			t.Fatalf("HostIP should not be set for %s", portSpec)
		}

		if bindings[0].HostPort != "" {
			t.Fatalf("HostPort should not be set for %s", portSpec)
		}
	}

	portSet, bindingMap, err = parsePortSpecs([]string{"1234-1236:1234-1236/tcp", "2345-2347:2345-2347/udp", "3456-3458:3456-3458/sctp"})
	if err != nil {
		t.Fatalf("Error while processing ParsePortSpecs: %s", err)
	}

	if _, ok := portSet[network.MustParsePort("1235/tcp")]; !ok {
		t.Fatal("1234-1236 was not parsed properly")
	}

	if _, ok := portSet[network.MustParsePort("2346/udp")]; !ok {
		t.Fatal("2345-2347 was not parsed properly")
	}

	if _, ok := portSet[network.MustParsePort("3456/sctp")]; !ok {
		t.Fatal("3456-3458 was not parsed properly")
	}

	for portSpec, bindings := range bindingMap {
		_, port := splitProtoPort(portSpec.String())
		if len(bindings) != 1 {
			t.Fatalf("%s should have exactly one binding", portSpec)
		}

		if bindings[0].HostIP.IsValid() {
			t.Fatalf("HostIP should not be set for %s", portSpec)
		}

		if bindings[0].HostPort != port {
			t.Fatalf("HostPort should be %s for %s", port, portSpec)
		}
	}

	portSet, bindingMap, err = parsePortSpecs([]string{"0.0.0.0:1234-1236:1234-1236/tcp", "0.0.0.0:2345-2347:2345-2347/udp", "0.0.0.0:3456-3458:3456-3458/sctp"})
	if err != nil {
		t.Fatalf("Error while processing ParsePortSpecs: %s", err)
	}

	if _, ok := portSet[network.MustParsePort("1235/tcp")]; !ok {
		t.Fatal("1234-1236 was not parsed properly")
	}

	if _, ok := portSet[network.MustParsePort("2346/udp")]; !ok {
		t.Fatal("2345-2347 was not parsed properly")
	}

	if _, ok := portSet[network.MustParsePort("3456/sctp")]; !ok {
		t.Fatal("3456-3458 was not parsed properly")
	}

	for portSpec, bindings := range bindingMap {
		_, port := splitProtoPort(portSpec.String())
		if len(bindings) != 1 || bindings[0].HostIP != netip.IPv4Unspecified() || bindings[0].HostPort != port {
			t.Fatalf("Expect single binding to port %s but found %s", port, bindings)
		}
	}

	_, _, err = parsePortSpecs([]string{"localhost:1234-1236:1234-1236/tcp"})

	if err == nil {
		t.Fatal("Received no error while trying to parse a hostname instead of ip")
	}
}

// Copied and modified from https://github.com/docker/go-connections/blob/c296721c0d56d3acad2973376ded214103a4fd2e/nat/nat_test.go#L602-L642
func TestParseNetworkOptsPrivateOnly(t *testing.T) {
	ports, bindings, err := parsePortSpecs([]string{"192.168.1.100::80"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 1 {
		t.Errorf("Expected 1 got %d", len(ports))
	}
	if len(bindings) != 1 {
		t.Errorf("Expected 1 got %d", len(bindings))
	}
	for k := range ports {
		if k.Proto() != "tcp" {
			t.Errorf("Expected tcp got %s", k.Proto())
		}
		if k.Num() != 80 {
			t.Errorf("Expected 80 got %d", k.Num())
		}
		b, exists := bindings[k]
		if !exists {
			t.Error("Binding does not exist")
		}
		if len(b) != 1 {
			t.Errorf("Expected 1 got %d", len(b))
		}
		s := b[0]
		if s.HostPort != "" {
			t.Errorf("Expected \"\" got %s", s.HostPort)
		}
		if s.HostIP != netip.MustParseAddr("192.168.1.100") {
			t.Errorf("Expected 192.168.1.100 got %s", s.HostIP)
		}
	}
}

// Copied and modified from https://github.com/docker/go-connections/blob/c296721c0d56d3acad2973376ded214103a4fd2e/nat/nat_test.go#L644-L684
func TestParseNetworkOptsPublic(t *testing.T) {
	ports, bindings, err := parsePortSpecs([]string{"192.168.1.100:8080:80"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 1 {
		t.Errorf("Expected 1 got %d", len(ports))
	}
	if len(bindings) != 1 {
		t.Errorf("Expected 1 got %d", len(bindings))
	}
	for k := range ports {
		if k.Proto() != "tcp" {
			t.Errorf("Expected tcp got %s", k.Proto())
		}
		if k.Num() != 80 {
			t.Errorf("Expected 80 got %d", k.Num())
		}
		b, exists := bindings[k]
		if !exists {
			t.Error("Binding does not exist")
		}
		if len(b) != 1 {
			t.Errorf("Expected 1 got %d", len(b))
		}
		s := b[0]
		if s.HostPort != "8080" {
			t.Errorf("Expected 8080 got %s", s.HostPort)
		}
		if s.HostIP != netip.MustParseAddr("192.168.1.100") {
			t.Errorf("Expected 192.168.1.100 got %s", s.HostIP)
		}
	}
}

// Copied and modified from https://github.com/docker/go-connections/blob/c296721c0d56d3acad2973376ded214103a4fd2e/nat/nat_test.go#L686-L701
func TestParseNetworkOptsPublicNoPort(t *testing.T) {
	ports, bindings, err := parsePortSpecs([]string{"192.168.1.100"})

	if err == nil {
		t.Error("Expected error Invalid containerPort")
	}
	if ports != nil {
		t.Errorf("Expected nil got %s", ports)
	}
	if bindings != nil {
		t.Errorf("Expected nil got %s", bindings)
	}
}

// Copied and modified from https://github.com/docker/go-connections/blob/c296721c0d56d3acad2973376ded214103a4fd2e/nat/nat_test.go#L703-L717
func TestParseNetworkOptsNegativePorts(t *testing.T) {
	ports, bindings, err := parsePortSpecs([]string{"192.168.1.100:-1:-1"})

	if err == nil {
		t.Error("Expected error Invalid containerPort")
	}
	if len(ports) != 0 {
		t.Errorf("Expected 0 got %d: %#v", len(ports), ports)
	}
	if len(bindings) != 0 {
		t.Errorf("Expected 0 got %d: %#v", len(bindings), bindings)
	}
}

// Copied and modified from https://github.com/docker/go-connections/blob/c296721c0d56d3acad2973376ded214103a4fd2e/nat/nat_test.go#L719-L759
func TestParseNetworkOptsUdp(t *testing.T) {
	ports, bindings, err := parsePortSpecs([]string{"192.168.1.100::6000/udp"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 1 {
		t.Errorf("Expected 1 got %d: %#v", len(ports), ports)
	}
	if len(bindings) != 1 {
		t.Errorf("Expected 1 got %d", len(bindings))
	}
	for k := range ports {
		if k.Proto() != "udp" {
			t.Errorf("Expected udp got %s", k.Proto())
		}
		if k.Num() != 6000 {
			t.Errorf("Expected 6000 got %d", k.Num())
		}
		b, exists := bindings[k]
		if !exists {
			t.Error("Binding does not exist")
		}
		if len(b) != 1 {
			t.Errorf("Expected 1 got %d", len(b))
		}
		s := b[0]
		if s.HostPort != "" {
			t.Errorf("Expected \"\" got %s", s.HostPort)
		}
		if s.HostIP != netip.MustParseAddr("192.168.1.100") {
			t.Errorf("Expected 192.168.1.100 got %s", s.HostIP)
		}
	}
}

// Copied and modified from https://github.com/docker/go-connections/blob/c296721c0d56d3acad2973376ded214103a4fd2e/nat/nat_test.go#L761-L801
func TestParseNetworkOptsSctp(t *testing.T) {
	ports, bindings, err := parsePortSpecs([]string{"192.168.1.100::6000/sctp"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 1 {
		t.Errorf("Expected 1 got %d: %#v", len(ports), ports)
	}
	if len(bindings) != 1 {
		t.Errorf("Expected 1 got %d: %#v", len(bindings), bindings)
	}
	for k := range ports {
		if k.Proto() != "sctp" {
			t.Errorf("Expected sctp got %s", k.Proto())
		}
		if k.Num() != 6000 {
			t.Errorf("Expected 6000 got %d", k.Num())
		}
		b, exists := bindings[k]
		if !exists {
			t.Error("Binding does not exist")
		}
		if len(b) != 1 {
			t.Errorf("Expected 1 got %d", len(b))
		}
		s := b[0]
		if s.HostPort != "" {
			t.Errorf("Expected \"\" got %s", s.HostPort)
		}
		if s.HostIP != netip.MustParseAddr("192.168.1.100") {
			t.Errorf("Expected 192.168.1.100 got %s", s.HostIP)
		}
	}
}

// Copied from https://github.com/docker/go-connections/blob/c296721c0d56d3acad2973376ded214103a4fd2e/nat/nat_test.go#L146-L242
func TestSplitProtoPort(t *testing.T) {
	tests := []struct {
		doc      string
		input    string
		expPort  string
		expProto string
	}{
		{
			doc: "empty value",
		},
		{
			doc:      "zero value",
			input:    "0",
			expPort:  "0",
			expProto: "tcp",
		},
		{
			doc:      "empty port",
			input:    "/udp",
			expPort:  "",
			expProto: "",
		},
		{
			doc:      "single port",
			input:    "1234",
			expPort:  "1234",
			expProto: "tcp",
		},
		{
			doc:      "single port with empty protocol",
			input:    "1234/",
			expPort:  "1234",
			expProto: "tcp",
		},
		{
			doc:      "single port with protocol",
			input:    "1234/udp",
			expPort:  "1234",
			expProto: "udp",
		},
		{
			doc:      "port range",
			input:    "80-8080",
			expPort:  "80-8080",
			expProto: "tcp",
		},
		{
			doc:      "port range with empty protocol",
			input:    "80-8080/",
			expPort:  "80-8080",
			expProto: "tcp",
		},
		{
			doc:      "port range with protocol",
			input:    "80-8080/udp",
			expPort:  "80-8080",
			expProto: "udp",
		},
		// SplitProtoPort currently does not validate or normalize, so these are expected returns
		{
			doc:      "negative value",
			input:    "-1",
			expPort:  "-1",
			expProto: "tcp",
		},
		{
			doc:      "uppercase protocol",
			input:    "1234/UDP",
			expPort:  "1234",
			expProto: "UDP",
		},
		{
			doc:      "any value",
			input:    "any port value",
			expPort:  "any port value",
			expProto: "tcp",
		},
		{
			doc:      "any value with protocol",
			input:    "any port value/any proto value",
			expPort:  "any port value",
			expProto: "any proto value",
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			proto, port := splitProtoPort(tc.input)
			if proto != tc.expProto {
				t.Errorf("expected proto %s, got %s", tc.expProto, proto)
			}
			if port != tc.expPort {
				t.Errorf("expected port %s, got %s", tc.expPort, port)
			}
		})
	}
}
