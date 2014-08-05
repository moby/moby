package evaluator

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/erikh/buildfile/parser"

	"github.com/docker/docker/daemon"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/nat"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

var (
	evaluateTable = map[string]func(*buildFile, ...string) error{
		"env":        env,
		"maintainer": maintainer,
		"add":        add,
		"copy":       dispatchCopy, // copy() is a go builtin
		//"onbuild":        parseMaybeJSON,
		//"workdir":        parseString,
		//"docker-version": parseString,
		//"run":            parseMaybeJSON,
		//"cmd":            parseMaybeJSON,
		//"entrypoint":     parseMaybeJSON,
		//"expose":         parseMaybeJSON,
		//"volume":         parseMaybeJSON,
	}
)

type buildFile struct {
	dockerfile *parser.Node
	env        envMap
	image      string
	config     *runconfig.Config
	options    *BuildOpts
	maintainer string
}

type BuildOpts struct {
	Daemon          *daemon.Daemon
	Engine          *engine.Engine
	OutStream       io.Writer
	ErrStream       io.Writer
	Verbose         bool
	UtilizeCache    bool
	Remove          bool
	ForceRm         bool
	OutOld          io.Writer
	StreamFormatter *utils.StreamFormatter
	Auth            *registry.AuthConfig
	AuthConfigFile  *registry.ConfigFile
}

func NewBuildFile(file io.ReadWriteCloser, opts *BuildOpts) (*buildFile, error) {
	defer file.Close()
	ast, err := parser.Parse(file)
	if err != nil {
		return nil, err
	}

	return &buildFile{
		dockerfile: ast,
		env:        envMap{},
		config:     initRunConfig(),
		options:    opts,
	}, nil
}

func (b *buildFile) Run() error {
	node := b.dockerfile

	for i, n := range node.Children {
		if err := b.dispatch(i, n); err != nil {
			return err
		}
	}

	return nil
}

func initRunConfig() *runconfig.Config {
	return &runconfig.Config{
		PortSpecs: []string{},
		// FIXME(erikh) this should be a type that lives in runconfig
		ExposedPorts: map[nat.Port]struct{}{},
		Env:          []string{},
		Cmd:          []string{},

		// FIXME(erikh) this should also be a type in runconfig
		Volumes:    map[string]struct{}{},
		Entrypoint: []string{},
		OnBuild:    []string{},
	}
}

func (b *buildFile) dispatch(stepN int, ast *parser.Node) error {
	cmd := ast.Value
	strs := []string{}
	for ast.Next != nil {
		ast = ast.Next
		strs = append(strs, replaceEnv(b, stripQuotes(ast.Value)))
	}

	fmt.Fprintf(b.outStream, "Step %d : %s\n", i, cmd, expression)

	// XXX yes, we skip any cmds that are not valid; the parser should have
	// picked these out already.
	if f, ok := evaluateTable[cmd]; ok {
		return f(b, strs...)
	}

	return nil
}
