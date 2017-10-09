package instructions

import (
	"strings"
	"testing"

	"github.com/docker/docker/builder/dockerfile/command"
	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/docker/docker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandsExactlyOneArgument(t *testing.T) {
	commands := []string{
		"MAINTAINER",
		"WORKDIR",
		"USER",
		"STOPSIGNAL",
	}

	for _, command := range commands {
		ast, err := parser.Parse(strings.NewReader(command))
		require.NoError(t, err)
		_, err = ParseInstruction(ast.AST.Children[0])
		assert.EqualError(t, err, errExactlyOneArgument(command).Error())
	}
}

func TestCommandsAtLeastOneArgument(t *testing.T) {
	commands := []string{
		"ENV",
		"LABEL",
		"ONBUILD",
		"HEALTHCHECK",
		"EXPOSE",
		"VOLUME",
	}

	for _, command := range commands {
		ast, err := parser.Parse(strings.NewReader(command))
		require.NoError(t, err)
		_, err = ParseInstruction(ast.AST.Children[0])
		assert.EqualError(t, err, errAtLeastOneArgument(command).Error())
	}
}

func TestCommandsNoDestinationArgument(t *testing.T) {
	commands := []string{
		"ADD",
		"COPY",
	}

	for _, command := range commands {
		ast, err := parser.Parse(strings.NewReader(command + " arg1"))
		require.NoError(t, err)
		_, err = ParseInstruction(ast.AST.Children[0])
		assert.EqualError(t, err, errNoDestinationArgument(command).Error())
	}
}

func TestCommandsTooManyArguments(t *testing.T) {
	commands := []string{
		"ENV",
		"LABEL",
	}

	for _, command := range commands {
		node := &parser.Node{
			Original: command + "arg1 arg2 arg3",
			Value:    strings.ToLower(command),
			Next: &parser.Node{
				Value: "arg1",
				Next: &parser.Node{
					Value: "arg2",
					Next: &parser.Node{
						Value: "arg3",
					},
				},
			},
		}
		_, err := ParseInstruction(node)
		assert.EqualError(t, err, errTooManyArguments(command).Error())
	}
}

func TestCommandsBlankNames(t *testing.T) {
	commands := []string{
		"ENV",
		"LABEL",
	}

	for _, command := range commands {
		node := &parser.Node{
			Original: command + " =arg2",
			Value:    strings.ToLower(command),
			Next: &parser.Node{
				Value: "",
				Next: &parser.Node{
					Value: "arg2",
				},
			},
		}
		_, err := ParseInstruction(node)
		assert.EqualError(t, err, errBlankCommandNames(command).Error())
	}
}

func TestHealthCheckCmd(t *testing.T) {
	node := &parser.Node{
		Value: command.Healthcheck,
		Next: &parser.Node{
			Value: "CMD",
			Next: &parser.Node{
				Value: "hello",
				Next: &parser.Node{
					Value: "world",
				},
			},
		},
	}
	cmd, err := ParseInstruction(node)
	assert.NoError(t, err)
	hc, ok := cmd.(*HealthCheckCommand)
	assert.True(t, ok)
	expected := []string{"CMD-SHELL", "hello world"}
	assert.Equal(t, expected, hc.Health.Test)
}

func TestParseOptInterval(t *testing.T) {
	flInterval := &Flag{
		name:     "interval",
		flagType: stringType,
		Value:    "50ns",
	}
	_, err := parseOptInterval(flInterval)
	testutil.ErrorContains(t, err, "cannot be less than 1ms")

	flInterval.Value = "1ms"
	_, err = parseOptInterval(flInterval)
	require.NoError(t, err)
}

func TestErrorCases(t *testing.T) {
	cases := []struct {
		name          string
		dockerfile    string
		expectedError string
	}{
		{
			name: "copyEmptyWhitespace",
			dockerfile: `COPY	
		quux \
      bar`,
			expectedError: "COPY requires at least two arguments",
		},
		{
			name:          "ONBUILD forbidden FROM",
			dockerfile:    "ONBUILD FROM scratch",
			expectedError: "FROM isn't allowed as an ONBUILD trigger",
		},
		{
			name:          "ONBUILD forbidden MAINTAINER",
			dockerfile:    "ONBUILD MAINTAINER docker.io",
			expectedError: "MAINTAINER isn't allowed as an ONBUILD trigger",
		},
		{
			name:          "ARG two arguments",
			dockerfile:    "ARG foo bar",
			expectedError: "ARG requires exactly one argument",
		},
		{
			name:          "MAINTAINER unknown flag",
			dockerfile:    "MAINTAINER --boo joe@example.com",
			expectedError: "Unknown flag: boo",
		},
		{
			name:          "Chaining ONBUILD",
			dockerfile:    `ONBUILD ONBUILD RUN touch foobar`,
			expectedError: "Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed",
		},
		{
			name:          "Invalid instruction",
			dockerfile:    `foo bar`,
			expectedError: "unknown instruction: FOO",
		},
	}
	for _, c := range cases {
		r := strings.NewReader(c.dockerfile)
		ast, err := parser.Parse(r)

		if err != nil {
			t.Fatalf("Error when parsing Dockerfile: %s", err)
		}
		n := ast.AST.Children[0]
		_, err = ParseInstruction(n)
		testutil.ErrorContains(t, err, c.expectedError)
	}

}
