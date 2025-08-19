package dockerfile

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/moby/buildkit/frontend/dockerfile/shell"
	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/builder"
	"github.com/moby/moby/v2/daemon/internal/image"
	"github.com/moby/moby/v2/daemon/pkg/oci"
	"github.com/moby/moby/v2/daemon/server/backend"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func newBuilderWithMockBackend(t *testing.T) *Builder {
	t.Helper()
	mockBackend := &MockBackend{}
	opts := &build.ImageBuildOptions{}
	ctx := context.Background()

	imageProber, err := newImageProber(ctx, mockBackend, nil, false)
	assert.NilError(t, err, "Could not create image prober")

	b := &Builder{
		options:       opts,
		docker:        mockBackend,
		Stdout:        new(bytes.Buffer),
		disableCommit: true,
		imageSources: newImageSources(builderOptions{
			Options: opts,
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
	err := dispatch(context.TODO(), sb, envCommand)
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
	err := dispatch(context.TODO(), sb, envCommand)
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
	err := dispatch(context.TODO(), sb, cmd)
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
	err := dispatch(context.TODO(), sb, cmd)
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
	err := initializeStage(context.TODO(), sb, cmd)

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
	err = initializeStage(context.TODO(), sb, cmd)
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
	err = initializeStage(context.TODO(), sb, cmd)
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
	err := initializeStage(context.TODO(), sb, cmd)
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
	err := initializeStage(context.TODO(), firstSB, firstFrom)
	assert.NilError(t, err)
	assert.Check(t, firstSB.state.hasFromImage())
	previousResults.indexed["base"] = firstSB.state.runConfig
	previousResults.flat = append(previousResults.flat, firstSB.state.runConfig)
	err = initializeStage(context.TODO(), secondSB, secondFrom)
	assert.NilError(t, err)
	assert.Check(t, secondSB.state.hasFromImage())
}

func TestOnbuild(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '\\', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())
	cmd := &instructions.OnbuildCommand{
		Expression: "ADD . /app/src",
	}
	err := dispatch(context.TODO(), sb, cmd)
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

	err := dispatch(context.TODO(), sb, cmd)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(sb.state.runConfig.WorkingDir, workingDir))
}

func TestCmd(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '`', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())
	sb.state.baseImage = &mockImage{}

	err := dispatch(context.TODO(), sb, &instructions.CmdCommand{
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
	err := dispatch(context.TODO(), sb, cmd)
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
	err := dispatch(context.TODO(), sb, cmd)
	assert.NilError(t, err)

	assert.Assert(t, sb.state.runConfig.Healthcheck != nil)
	assert.Check(t, is.DeepEqual(sb.state.runConfig.Healthcheck.Test, expectedTest))
}

func TestEntrypoint(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '`', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())
	sb.state.baseImage = &mockImage{}

	err := dispatch(context.TODO(), sb, &instructions.EntrypointCommand{
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
	err := dispatch(context.TODO(), sb, cmd)
	assert.NilError(t, err)

	assert.Assert(t, sb.state.runConfig.ExposedPorts != nil)
	assert.Assert(t, is.Len(sb.state.runConfig.ExposedPorts, 1))

	assert.Check(t, is.Contains(sb.state.runConfig.ExposedPorts, container.PortRangeProto("80/tcp")))
}

func TestUser(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '`', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())

	cmd := &instructions.UserCommand{
		User: "test",
	}
	err := dispatch(context.TODO(), sb, cmd)
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
	err := dispatch(context.TODO(), sb, cmd)
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
	err := dispatch(context.TODO(), sb, cmd)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(sb.state.runConfig.StopSignal, signal))
}

func TestArg(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '`', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())

	argName := "foo"
	argVal := "bar"
	cmd := &instructions.ArgCommand{Args: []instructions.KeyValuePairOptional{{Key: argName, Value: &argVal}}}
	err := dispatch(context.TODO(), sb, cmd)
	assert.NilError(t, err)

	expected := map[string]string{argName: argVal}
	assert.Check(t, is.DeepEqual(sb.state.buildArgs.GetAllAllowed(), expected))
}

func TestShell(t *testing.T) {
	b := newBuilderWithMockBackend(t)
	sb := newDispatchRequest(b, '`', nil, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())

	shellCmd := []string{"powershell"}
	err := dispatch(context.TODO(), sb, &instructions.ShellCommand{
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

	prober, err := newImageProber(context.TODO(), mockBackend, nil, false)
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
	err = initializeStage(context.TODO(), sb, from)
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

	assert.NilError(t, dispatch(context.TODO(), sb, runinst))

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
	imageProber, err := newImageProber(context.TODO(), mockBackend, nil, false)
	assert.NilError(t, err, "Could not create image prober")

	b.imageProber = imageProber
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
	err = initializeStage(context.TODO(), sb, from)
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

	assert.NilError(t, dispatch(context.TODO(), sb, cmd))
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

	assert.NilError(t, dispatch(context.TODO(), sb, run))
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
		err := dispatch(context.TODO(), sb, cmd)
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
		err := dispatch(context.TODO(), sb, cmd)
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
			err := dispatch(context.TODO(), sb, cmd)
			assert.Error(t, err, fmt.Sprintf("the --%s option requires BuildKit. Refer to https://docs.docker.com/go/buildkit/ to learn how to build images with BuildKit enabled", f))
		}
	})
}
