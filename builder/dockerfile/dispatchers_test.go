package dockerfile

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/docker/engine-api/types/container"
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
		{"USER", func(args []string) error { return user(nil, args, nil, "") }}}

	for _, command := range commands {
		err := command.function([]string{})

		if err == nil {
			t.Fatalf("Error should be present for %s command", command.name)
		}

		expectedError := fmt.Sprintf("%s requires exactly one argument", command.name)

		if err.Error() != expectedError {
			t.Fatalf("Wrong error message for %s. Got: %s. Should be: %s", command.name, err.Error(), expectedError)
		}
	}
}

func TestCommandsAtLeastOneArgument(t *testing.T) {
	commands := []commandWithFunction{
		{"ENV", func(args []string) error { return env(nil, args, nil, "") }},
		{"LABEL", func(args []string) error { return label(nil, args, nil, "") }},
		{"ADD", func(args []string) error { return add(nil, args, nil, "") }},
		{"COPY", func(args []string) error { return dispatchCopy(nil, args, nil, "") }},
		{"ONBUILD", func(args []string) error { return onbuild(nil, args, nil, "") }},
		{"EXPOSE", func(args []string) error { return expose(nil, args, nil, "") }},
		{"VOLUME", func(args []string) error { return volume(nil, args, nil, "") }}}

	for _, command := range commands {
		err := command.function([]string{})

		if err == nil {
			t.Fatalf("Error should be present for %s command", command.name)
		}

		expectedError := fmt.Sprintf("%s requires at least one argument", command.name)

		if err.Error() != expectedError {
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

		expectedError := fmt.Sprintf("Bad input to %s, too many arguments", command.name)

		if err.Error() != expectedError {
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

	err := from(b, []string{"scratch"}, nil, "")

	if runtime.GOOS == "windows" {
		if err == nil {
			t.Fatalf("Error not set on Windows")
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
			t.Fatalf("Image shoule be empty, got: %s", b.image)
		}

		if b.noBaseImage != true {
			t.Fatalf("Image should not have any base image, got: %s", b.noBaseImage)
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
			t.Fatalf("Error should not be nil")
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
