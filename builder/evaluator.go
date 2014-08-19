// builder is the evaluation step in the Dockerfile parse/evaluate pipeline.
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
package builder

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/docker/docker/builder/parser"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/tarsum"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

var (
	ErrDockerfileEmpty = errors.New("Dockerfile cannot be empty")
)

var evaluateTable map[string]func(*BuildFile, []string, map[string]bool) error

func init() {
	evaluateTable = map[string]func(*BuildFile, []string, map[string]bool) error{
		"env":            env,
		"maintainer":     maintainer,
		"add":            add,
		"copy":           dispatchCopy, // copy() is a go builtin
		"from":           from,
		"onbuild":        onbuild,
		"workdir":        workdir,
		"docker-version": nullDispatch, // we don't care about docker-version
		"run":            run,
		"cmd":            cmd,
		"entrypoint":     entrypoint,
		"expose":         expose,
		"volume":         volume,
		"user":           user,
		"insert":         insert,
	}
}

// internal struct, used to maintain configuration of the Dockerfile's
// processing as it evaluates the parsing result.
type BuildFile struct {
	Dockerfile *parser.Node      // the syntax tree of the dockerfile
	Config     *runconfig.Config // runconfig for cmd, run, entrypoint etc.
	Options    *BuildOpts        // see below

	// both of these are controlled by the Remove and ForceRemove options in BuildOpts
	TmpContainers map[string]struct{} // a map of containers used for removes

	image       string         // image name for commit processing
	maintainer  string         // maintainer name. could probably be removed.
	cmdSet      bool           // indicates is CMD was set in current Dockerfile
	context     *tarsum.TarSum // the context is a tarball that is uploaded by the client
	contextPath string         // the path of the temporary directory the local context is unpacked to (server side)

}

type BuildOpts struct {
	Daemon *daemon.Daemon
	Engine *engine.Engine

	// effectively stdio for the run. Because it is not stdio, I said
	// "Effectively". Do not use stdio anywhere in this package for any reason.
	OutStream io.Writer
	ErrStream io.Writer

	Verbose      bool
	UtilizeCache bool

	// controls how images and containers are handled between steps.
	Remove      bool
	ForceRemove bool

	AuthConfig     *registry.AuthConfig
	AuthConfigFile *registry.ConfigFile

	// Deprecated, original writer used for ImagePull. To be removed.
	OutOld          io.Writer
	StreamFormatter *utils.StreamFormatter
}

// Run the builder with the context. This is the lynchpin of this package. This
// will (barring errors):
//
// * call readContext() which will set up the temporary directory and unpack
//   the context into it.
// * read the dockerfile
// * parse the dockerfile
// * walk the parse tree and execute it by dispatching to handlers. If Remove
//   or ForceRemove is set, additional cleanup around containers happens after
//   processing.
// * Print a happy message and return the image ID.
//
func (b *BuildFile) Run(context io.Reader) (string, error) {
	if err := b.readContext(context); err != nil {
		return "", err
	}

	filename := path.Join(b.contextPath, "Dockerfile")
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return "", fmt.Errorf("Cannot build a directory without a Dockerfile")
	}
	fileBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	if len(fileBytes) == 0 {
		return "", ErrDockerfileEmpty
	}
	ast, err := parser.Parse(bytes.NewReader(fileBytes))
	if err != nil {
		return "", err
	}

	b.Dockerfile = ast

	for i, n := range b.Dockerfile.Children {
		if err := b.dispatch(i, n); err != nil {
			if b.Options.ForceRemove {
				b.clearTmp()
			}
			return "", err
		}
		fmt.Fprintf(b.Options.OutStream, " ---> %s\n", utils.TruncateID(b.image))
		if b.Options.Remove {
			b.clearTmp()
		}
	}

	if b.image == "" {
		return "", fmt.Errorf("No image was generated. Is your Dockerfile empty?\n")
	}

	fmt.Fprintf(b.Options.OutStream, "Successfully built %s\n", utils.TruncateID(b.image))
	return b.image, nil
}

// This method is the entrypoint to all statement handling routines.
//
// Almost all nodes will have this structure:
// Child[Node, Node, Node] where Child is from parser.Node.Children and each
// node comes from parser.Node.Next. This forms a "line" with a statement and
// arguments and we process them in this normalized form by hitting
// evaluateTable with the leaf nodes of the command and the BuildFile object.
//
// ONBUILD is a special case; in this case the parser will emit:
// Child[Node, Child[Node, Node...]] where the first node is the literal
// "onbuild" and the child entrypoint is the command of the ONBUILD statmeent,
// such as `RUN` in ONBUILD RUN foo. There is special case logic in here to
// deal with that, at least until it becomes more of a general concern with new
// features.
func (b *BuildFile) dispatch(stepN int, ast *parser.Node) error {
	cmd := ast.Value
	attrs := ast.Attributes
	strs := []string{}
	msg := fmt.Sprintf("Step %d : %s", stepN, strings.ToUpper(cmd))

	if cmd == "onbuild" {
		fmt.Fprintf(b.Options.OutStream, "%#v\n", ast.Next.Children[0].Value)
		ast = ast.Next.Children[0]
		strs = append(strs, b.replaceEnv(ast.Value))
		msg += " " + ast.Value
	}

	for ast.Next != nil {
		ast = ast.Next
		strs = append(strs, b.replaceEnv(ast.Value))
		msg += " " + ast.Value
	}

	fmt.Fprintf(b.Options.OutStream, "%s\n", msg)

	// XXX yes, we skip any cmds that are not valid; the parser should have
	// picked these out already.
	if f, ok := evaluateTable[cmd]; ok {
		return f(b, strs, attrs)
	}

	fmt.Fprintf(b.Options.ErrStream, "# Skipping unknown instruction %s\n", strings.ToUpper(cmd))

	return nil
}
