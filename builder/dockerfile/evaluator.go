// Package dockerfile is the evaluation step in the Dockerfile parse/evaluate pipeline.
//
// It incorporates a dispatch table based on the parser.Node values (see the
// parser package for more information) that are yielded from the parser itself.
// Calling NewBuilder with the BuildOpts struct can be used to customize the
// experience for execution purposes only. Parsing is controlled in the parser
// package, and this division of resposibility should be respected.
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
	"runtime"
	"strings"

	"github.com/docker/docker/builder/dockerfile/command"
	"github.com/docker/docker/builder/dockerfile/parser"
)

// Environment variable interpolation will happen on these statements only.
var replaceEnvAllowed = map[string]struct{}{
	command.Env:        {},
	command.Label:      {},
	command.Add:        {},
	command.Copy:       {},
	command.Workdir:    {},
	command.Expose:     {},
	command.Volume:     {},
	command.User:       {},
	command.StopSignal: {},
	command.Arg:        {},
}

var evaluateTable map[string]func(*Builder, []string, map[string]bool, string) error

func init() {
	evaluateTable = map[string]func(*Builder, []string, map[string]bool, string) error{
		command.Env:        env,
		command.Label:      label,
		command.Maintainer: maintainer,
		command.Add:        add,
		command.Copy:       dispatchCopy, // copy() is a go builtin
		command.From:       from,
		command.Onbuild:    onbuild,
		command.Workdir:    workdir,
		command.Run:        run,
		command.Cmd:        cmd,
		command.Entrypoint: entrypoint,
		command.Expose:     expose,
		command.Volume:     volume,
		command.User:       user,
		command.StopSignal: stopSignal,
		command.Arg:        arg,
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
func (b *Builder) dispatch(stepN int, ast *parser.Node) error {
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
	strs := []string{}
	msg := fmt.Sprintf("Step %d : %s", stepN+1, upperCasedCmd)

	if len(ast.Flags) > 0 {
		msg += " " + strings.Join(ast.Flags, " ")
	}

	if cmd == "onbuild" {
		if ast.Next == nil {
			return fmt.Errorf("ONBUILD requires at least one argument")
		}
		ast = ast.Next.Children[0]
		strs = append(strs, ast.Value)
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
	l := len(strs)
	strList := make([]string, n+l)
	copy(strList, strs)
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
	for key, val := range b.BuildArgs {
		if !b.isBuildArgAllowed(key) {
			// skip build-args that are not in allowed list, meaning they have
			// not been defined by an "ARG" Dockerfile command yet.
			// This is an error condition but only if there is no "ARG" in the entire
			// Dockerfile, so we'll generate any necessary errors after we parsed
			// the entire file (see 'leftoverArgs' processing in evaluator.go )
			continue
		}
		envs = append(envs, fmt.Sprintf("%s=%s", key, val))
	}
	for ast.Next != nil {
		ast = ast.Next
		var str string
		str = ast.Value
		if _, ok := replaceEnvAllowed[cmd]; ok {
			var err error
			str, err = ProcessWord(ast.Value, envs)
			if err != nil {
				return err
			}
		}
		strList[i+l] = str
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

// platformSupports is a short-term function to give users a quality error
// message if a Dockerfile uses a command not supported on the platform.
func platformSupports(command string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	switch command {
	case "expose", "volume", "user", "stopsignal", "arg":
		return fmt.Errorf("The daemon on this platform does not support the command '%s'", command)
	}
	return nil
}
