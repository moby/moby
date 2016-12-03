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
	"fmt"
	"strings"

	"github.com/docker/docker/builder/dockerfile/command"
	"github.com/docker/docker/builder/dockerfile/parser"
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
			return fmt.Errorf("ONBUILD requires at least one argument")
		}
		ast = ast.Next.Children[0]
		strList = append(strList, ast.Value)
		msg += " " + ast.Value

		if len(ast.Flags) > 0 {
			msg += " " + strings.Join(ast.Flags, " ")
		}

	}

	// count the number of nodes that we are going to traverse first
	// so we can pre-create the argument and message array. This speeds up the
	// allocation of those list a lot when they have a lot of arguments
	cursor := ast
	var n int
	for cursor.Next != nil {
		cursor = cursor.Next
		n++
	}
	msgList := make([]string, n)

	var i int
	// Append the build-time args to config-environment.
	// This allows builder config to override the variables, making the behavior similar to
	// a shell script i.e. `ENV foo bar` overrides value of `foo` passed in build
	// context. But `ENV foo $foo` will use the value from build context if one
	// isn't already been defined by a previous ENV primitive.
	// Note, we get this behavior because we know that ProcessWord() will
	// stop on the first occurrence of a variable name and not notice
	// a subsequent one. So, putting the buildArgs list after the Config.Env
	// list, in 'envs', is safe.
	envs := b.runConfig.Env
	for key, val := range b.options.BuildArgs {
		if !b.isBuildArgAllowed(key) {
			// skip build-args that are not in allowed list, meaning they have
			// not been defined by an "ARG" Dockerfile command yet.
			// This is an error condition but only if there is no "ARG" in the entire
			// Dockerfile, so we'll generate any necessary errors after we parsed
			// the entire file (see 'leftoverArgs' processing in evaluator.go )
			continue
		}
		envs = append(envs, fmt.Sprintf("%s=%s", key, *val))
	}
	for ast.Next != nil {
		ast = ast.Next
		var str string
		str = ast.Value
		if replaceEnvAllowed[cmd] {
			var err error
			var words []string

			if allowWordExpansion[cmd] {
				words, err = ProcessWords(str, envs, b.directive.EscapeToken)
				if err != nil {
					return err
				}
				strList = append(strList, words...)
			} else {
				str, err = ProcessWord(str, envs, b.directive.EscapeToken)
				if err != nil {
					return err
				}
				strList = append(strList, str)
			}
		} else {
			strList = append(strList, str)
		}
		msgList[i] = ast.Value
		i++
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
			return fmt.Errorf("ONBUILD requires at least one argument")
		}
	}

	// The instruction is part of ONBUILD trigger (not the instruction itself)
	if onbuild {
		switch upperCasedCmd {
		case "ONBUILD":
			return fmt.Errorf("Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed")
		case "MAINTAINER", "FROM":
			return fmt.Errorf("%s isn't allowed as an ONBUILD trigger", upperCasedCmd)
		}
	}

	if _, ok := evaluateTable[cmd]; ok {
		return nil
	}

	return fmt.Errorf("Unknown instruction: %s", upperCasedCmd)
}
