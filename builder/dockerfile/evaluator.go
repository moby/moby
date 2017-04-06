// Package dockerfile is the evaluation step in the Dockerfile parse/evaluate pipeline.
//
// It incorporates a dispatch table based on the parser.Node values (see the
// parser package for more information) that are yielded from the parser itself.
// Calling NewBuilder with the BuildOpts struct can be used to customize the
// experience for execution purposes only. Parsing is controlled in the parser
// package, and this division of responsibility should be respected.
//
// Please see the jump table targets for the actual invocations, most of which
// will call out to the functions in internals.go to deal with their tasks.
//
// ONBUILD is a special case, which is covered in the onbuild() func in
// dispatchers.go.
//
// The evaluator uses the concept of "steps", which are usually each processable
// line in the Dockerfile. Each step is numbered and certain actions are taken
// before and after each step, such as creating an image ID and removing temporary
// containers and images. Note that ONBUILD creates a kinda-sorta "sub run" which
// includes its own set of steps (usually only one of them).
package dockerfile

import (
	"errors"
	"fmt"
	"strings"

	"github.com/docker/docker/builder/dockerfile/command"
	"github.com/docker/docker/builder/dockerfile/parser"
	runconfigopts "github.com/docker/docker/runconfig/opts"
)

// Environment variable interpolation will happen on these statements only.
var replaceEnvAllowed = map[string]bool{
	command.Env:        true,
	command.Label:      true,
	command.Add:        true,
	command.Copy:       true,
	command.Workdir:    true,
	command.Expose:     true,
	command.Volume:     true,
	command.User:       true,
	command.StopSignal: true,
	command.Arg:        true,
}

// Certain commands are allowed to have their args split into more
// words after env var replacements. Meaning:
//   ENV foo="123 456"
//   EXPOSE $foo
// should result in the same thing as:
//   EXPOSE 123 456
// and not treat "123 456" as a single word.
// Note that: EXPOSE "$foo" and EXPOSE $foo are not the same thing.
// Quotes will cause it to still be treated as single word.
var allowWordExpansion = map[string]bool{
	command.Expose: true,
}

var evaluateTable map[string]func(*Builder, []string, map[string]bool, string) error

func init() {
	evaluateTable = map[string]func(*Builder, []string, map[string]bool, string) error{
		command.Add:         add,
		command.Arg:         arg,
		command.Cmd:         cmd,
		command.Copy:        dispatchCopy, // copy() is a go builtin
		command.Entrypoint:  entrypoint,
		command.Env:         env,
		command.Expose:      expose,
		command.From:        from,
		command.Healthcheck: healthcheck,
		command.Label:       label,
		command.Maintainer:  maintainer,
		command.Onbuild:     onbuild,
		command.Run:         run,
		command.Shell:       shell,
		command.StopSignal:  stopSignal,
		command.User:        user,
		command.Volume:      volume,
		command.Workdir:     workdir,
	}
}

// This method is the entrypoint to all statement handling routines.
//
// Almost all nodes will have this structure:
// Child[Node, Node, Node] where Child is from parser.Node.Children and each
// node comes from parser.Node.Next. This forms a "line" with a statement and
// arguments and we process them in this normalized form by hitting
// evaluateTable with the leaf nodes of the command and the Builder object.
//
// ONBUILD is a special case; in this case the parser will emit:
// Child[Node, Child[Node, Node...]] where the first node is the literal
// "onbuild" and the child entrypoint is the command of the ONBUILD statement,
// such as `RUN` in ONBUILD RUN foo. There is special case logic in here to
// deal with that, at least until it becomes more of a general concern with new
// features.
func (b *Builder) dispatch(stepN int, stepTotal int, ast *parser.Node) error {
	cmd := ast.Value
	upperCasedCmd := strings.ToUpper(cmd)

	// To ensure the user is given a decent error message if the platform
	// on which the daemon is running does not support a builder command.
	if err := platformSupports(strings.ToLower(cmd)); err != nil {
		return err
	}

	attrs := ast.Attributes
	original := ast.Original
	flags := ast.Flags
	strList := []string{}
	msg := fmt.Sprintf("Step %d/%d : %s", stepN+1, stepTotal, upperCasedCmd)

	if len(ast.Flags) > 0 {
		msg += " " + strings.Join(ast.Flags, " ")
	}

	if cmd == "onbuild" {
		if ast.Next == nil {
			return errors.New("ONBUILD requires at least one argument")
		}
		ast = ast.Next.Children[0]
		strList = append(strList, ast.Value)
		msg += " " + ast.Value

		if len(ast.Flags) > 0 {
			msg += " " + strings.Join(ast.Flags, " ")
		}

	}

	msgList := initMsgList(ast)
	// Append build args to runConfig environment variables
	envs := append(b.runConfig.Env, b.buildArgsWithoutConfigEnv()...)

	for i := 0; ast.Next != nil; i++ {
		ast = ast.Next
		words, err := b.evaluateEnv(cmd, ast.Value, envs)
		if err != nil {
			return err
		}
		strList = append(strList, words...)
		msgList[i] = ast.Value
	}

	msg += " " + strings.Join(msgList, " ")
	fmt.Fprintln(b.Stdout, msg)

	// XXX yes, we skip any cmds that are not valid; the parser should have
	// picked these out already.
	if f, ok := evaluateTable[cmd]; ok {
		b.flags = NewBFlags()
		b.flags.Args = flags
		return f(b, strList, attrs, original)
	}

	return fmt.Errorf("Unknown instruction: %s", upperCasedCmd)
}

// count the number of nodes that we are going to traverse first
// allocation of those list a lot when they have a lot of arguments
func initMsgList(cursor *parser.Node) []string {
	var n int
	for ; cursor.Next != nil; n++ {
		cursor = cursor.Next
	}
	return make([]string, n)
}

func (b *Builder) evaluateEnv(cmd string, str string, envs []string) ([]string, error) {
	if !replaceEnvAllowed[cmd] {
		return []string{str}, nil
	}
	var processFunc func(string, []string, rune) ([]string, error)
	if allowWordExpansion[cmd] {
		processFunc = ProcessWords
	} else {
		processFunc = ProcessWord
	}
	return processFunc(str, envs, b.directive.EscapeToken)
}

// buildArgsWithoutConfigEnv returns a list of key=value pairs for all the build
// args that are not overriden by runConfig environment variables.
func (b *Builder) buildArgsWithoutConfigEnv() []string {
	envs := []string{}
	configEnv := runconfigopts.ConvertKVStringsToMap(b.runConfig.Env)

	for key, val := range b.getBuildArgs() {
		if _, ok := configEnv[key]; !ok {
			envs = append(envs, fmt.Sprintf("%s=%s", key, val))
		}
	}
	return envs
}

// checkDispatch does a simple check for syntax errors of the Dockerfile.
// Because some of the instructions can only be validated through runtime,
// arg, env, etc., this syntax check will not be complete and could not replace
// the runtime check. Instead, this function is only a helper that allows
// user to find out the obvious error in Dockerfile earlier on.
// onbuild bool: indicate if instruction XXX is part of `ONBUILD XXX` trigger
func (b *Builder) checkDispatch(ast *parser.Node, onbuild bool) error {
	cmd := ast.Value
	upperCasedCmd := strings.ToUpper(cmd)

	// To ensure the user is given a decent error message if the platform
	// on which the daemon is running does not support a builder command.
	if err := platformSupports(strings.ToLower(cmd)); err != nil {
		return err
	}

	// The instruction itself is ONBUILD, we will make sure it follows with at
	// least one argument
	if upperCasedCmd == "ONBUILD" {
		if ast.Next == nil {
			return errors.New("ONBUILD requires at least one argument")
		}
	}

	// The instruction is part of ONBUILD trigger (not the instruction itself)
	if onbuild {
		switch upperCasedCmd {
		case "ONBUILD":
			return errors.New("Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed")
		case "MAINTAINER", "FROM":
			return fmt.Errorf("%s isn't allowed as an ONBUILD trigger", upperCasedCmd)
		}
	}

	if _, ok := evaluateTable[cmd]; ok {
		return nil
	}

	return fmt.Errorf("Unknown instruction: %s", upperCasedCmd)
}
