package evaluator

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
	"github.com/docker/docker/nat"
	"github.com/docker/docker/pkg/tarsum"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

var (
	ErrDockerfileEmpty = errors.New("Dockerfile cannot be empty")
)

var evaluateTable map[string]func(*buildFile, []string) error

func init() {
	evaluateTable = map[string]func(*buildFile, []string) error{
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

type envMap map[string]string
type uniqueMap map[string]struct{}

type buildFile struct {
	dockerfile *parser.Node
	env        envMap
	image      string
	config     *runconfig.Config
	options    *BuildOpts
	maintainer string

	// cmdSet indicates is CMD was set in current Dockerfile
	cmdSet bool

	context       *tarsum.TarSum
	contextPath   string
	tmpContainers uniqueMap
	tmpImages     uniqueMap
}

type BuildOpts struct {
	Daemon         *daemon.Daemon
	Engine         *engine.Engine
	OutStream      io.Writer
	ErrStream      io.Writer
	Verbose        bool
	UtilizeCache   bool
	Remove         bool
	ForceRemove    bool
	AuthConfig     *registry.AuthConfig
	AuthConfigFile *registry.ConfigFile

	// Deprecated, original writer used for ImagePull. To be removed.
	OutOld          io.Writer
	StreamFormatter *utils.StreamFormatter
}

func NewBuilder(opts *BuildOpts) (*buildFile, error) {
	return &buildFile{
		dockerfile:    nil,
		env:           envMap{},
		config:        initRunConfig(),
		options:       opts,
		tmpContainers: make(uniqueMap),
		tmpImages:     make(uniqueMap),
	}, nil
}

func (b *buildFile) Run(context io.Reader) (string, error) {
	err := b.readContext(context)

	if err != nil {
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

	b.dockerfile = ast

	for i, n := range b.dockerfile.Children {
		if err := b.dispatch(i, n); err != nil {
			if b.options.ForceRemove {
				b.clearTmp(b.tmpContainers)
			}
			return "", err
		} else if b.options.Remove {
			b.clearTmp(b.tmpContainers)
		}
	}

	if b.image == "" {
		return "", fmt.Errorf("No image was generated. This may be because the Dockerfile does not, like, do anything.\n")
	}

	fmt.Fprintf(b.options.OutStream, "Successfully built %s\n", utils.TruncateID(b.image))
	return b.image, nil
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
		Entrypoint: []string{"/bin/sh", "-c"},
		OnBuild:    []string{},
	}
}

func (b *buildFile) dispatch(stepN int, ast *parser.Node) error {
	cmd := ast.Value
	strs := []string{}

	if cmd == "onbuild" {
		fmt.Fprintf(b.options.OutStream, "%#v\n", ast.Next.Children[0].Value)
		ast = ast.Next.Children[0]
		strs = append(strs, ast.Value)
	}

	for ast.Next != nil {
		ast = ast.Next
		strs = append(strs, replaceEnv(b, ast.Value))
	}

	fmt.Fprintf(b.options.OutStream, "Step %d : %s %s\n", stepN, strings.ToUpper(cmd), strings.Join(strs, " "))

	// XXX yes, we skip any cmds that are not valid; the parser should have
	// picked these out already.
	if f, ok := evaluateTable[cmd]; ok {
		return f(b, strs)
	}

	return nil
}
