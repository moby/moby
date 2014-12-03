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
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	log "github.com/Sirupsen/logrus"
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

// Environment variable interpolation will happen on these statements only.
var replaceEnvAllowed = map[string]struct{}{
	"env":     {},
	"add":     {},
	"copy":    {},
	"workdir": {},
	"expose":  {},
	"volume":  {},
	"user":    {},
}

var evaluateTable map[string]func(*Builder, []string, map[string]bool, string) error

func init() {
	evaluateTable = map[string]func(*Builder, []string, map[string]bool, string) error{
		"env":        env,
		"maintainer": maintainer,
		"add":        add,
		"copy":       dispatchCopy, // copy() is a go builtin
		"from":       from,
		"onbuild":    onbuild,
		"workdir":    workdir,
		"run":        run,
		"cmd":        cmd,
		"entrypoint": entrypoint,
		"expose":     expose,
		"volume":     volume,
		"user":       user,
		"insert":     insert,
	}
}

// internal struct, used to maintain configuration of the Dockerfile's
// processing as it evaluates the parsing result.
type Builder struct {
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
	Pull        bool

	AuthConfig     *registry.AuthConfig
	AuthConfigFile *registry.ConfigFile

	// Deprecated, original writer used for ImagePull. To be removed.
	OutOld          io.Writer
	StreamFormatter *utils.StreamFormatter

	Config *runconfig.Config // runconfig for cmd, run, entrypoint etc.

	// both of these are controlled by the Remove and ForceRemove options in BuildOpts
	TmpContainers map[string]struct{} // a map of containers used for removes

	dockerfile  *parser.Node  // the syntax tree of the dockerfile
	image       string        // image name for commit processing
	maintainer  string        // maintainer name. could probably be removed.
	cmdSet      bool          // indicates is CMD was set in current Dockerfile
	context     tarsum.TarSum // the context is a tarball that is uploaded by the client
	contextPath string        // the path of the temporary directory the local context is unpacked to (server side)

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
func (b *Builder) Run(context io.Reader) (string, error) {
	if err := b.readContext(context); err != nil {
		return "", err
	}

	defer func() {
		if err := os.RemoveAll(b.contextPath); err != nil {
			log.Debugf("[BUILDER] failed to remove temporary context: %s", err)
		}
	}()

	filename := path.Join(b.contextPath, "Dockerfile")

	fi, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("Cannot build a directory without a Dockerfile")
	}
	if fi.Size() == 0 {
		return "", ErrDockerfileEmpty
	}

	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}

	defer f.Close()

	ast, err := parser.Parse(f)
	if err != nil {
		return "", err
	}

	b.dockerfile = ast

	// some initializations that would not have been supplied by the caller.
	b.Config = &runconfig.Config{}
	b.TmpContainers = map[string]struct{}{}

	for i, n := range b.dockerfile.Children {
		if err := b.dispatch(i, n); err != nil {
			if b.ForceRemove {
				b.clearTmp()
			}
			return "", err
		}
		fmt.Fprintf(b.OutStream, " ---> %s\n", utils.TruncateID(b.image))
		if b.Remove {
			b.clearTmp()
		}
	}

	if b.image == "" {
		return "", fmt.Errorf("No image was generated. Is your Dockerfile empty?\n")
	}

	fmt.Fprintf(b.OutStream, "Successfully built %s\n", utils.TruncateID(b.image))
	return b.image, nil
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
// "onbuild" and the child entrypoint is the command of the ONBUILD statmeent,
// such as `RUN` in ONBUILD RUN foo. There is special case logic in here to
// deal with that, at least until it becomes more of a general concern with new
// features.
func (b *Builder) dispatch(stepN int, ast *parser.Node) error {
	cmd := ast.Value
	attrs := ast.Attributes
	original := ast.Original
	strs := []string{}
	msg := fmt.Sprintf("Step %d : %s", stepN, strings.ToUpper(cmd))

	if cmd == "onbuild" {
		ast = ast.Next.Children[0]
		strs = append(strs, ast.Value)
		msg += " " + ast.Value
	}

	for ast.Next != nil {
		ast = ast.Next
		var str string
		str = ast.Value
		if _, ok := replaceEnvAllowed[cmd]; ok {
			str = b.replaceEnv(ast.Value)
		}
		strs = append(strs, str)
		msg += " " + ast.Value
	}

	fmt.Fprintln(b.OutStream, msg)

	// XXX yes, we skip any cmds that are not valid; the parser should have
	// picked these out already.
	if f, ok := evaluateTable[cmd]; ok {
		return f(b, strs, attrs, original)
	}

	fmt.Fprintf(b.ErrStream, "# Skipping unknown instruction %s\n", strings.ToUpper(cmd))

	return nil
}
