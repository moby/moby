package docker

import (
	"fmt"
	"strconv"
	"strings"
)

type fromArgs struct {
	name string
}

type maintainerArgs struct {
	name string
}

type runArgs struct {
	cmd []string
}

type cmdArgs struct {
	cmd []string
}

type exposeArgs struct {
	ports []int
}

type envArgs struct {
	key   string
	value string
}

type addArgs struct {
	src string
	dst string
}

type entryPointArgs struct {
	cmd []string
}

type volumeArgs struct {
	volumes []string
}

type userArgs struct {
	name string
}

type workDirArgs struct {
	path string
}

type includeArgs struct {
	filename string
}

type dockerFile struct {
	instructions []interface{}
}

// used by the parser
var (
	parsedFile *dockerFile
	shell      = map[string]bool{
		"sh":        true,
		"/bin/sh":   true,
		"bash":      true,
		"/bin/bash": true,
		"csh":       true,
		"/bin/csh":  true,
	}
)

func fixcmd(cmd []string) []string {
	if !shell[cmd[0]] {
		cmd = append([]string{"/bin/sh", "-c"}, strings.Join(cmd, " "))
	}
	return cmd
}

// String version of instructions returned by the parser

func (args *fromArgs) String() string {
	return "FROM " + args.name
}

func (args *maintainerArgs) String() string {
	return "MAINTAINER " + args.name
}

func (args *runArgs) String() string {
	return "RUN " + strings.Join(args.cmd, " ")
}

func (args *cmdArgs) String() string {
	return "CMD " + strings.Join(args.cmd, " ")
}

func (args *exposeArgs) String() string {
	a := make([]string, len(args.ports))
	for i := 0; i < len(args.ports); i++ {
		a[i] = strconv.Itoa(args.ports[i])
	}
	return "EXPOSE " + strings.Join(a, " ")
}

func (args *envArgs) String() string {
	return "ENV " + args.key + " " + args.value
}

func (args *addArgs) String() string {
	return "ADD " + args.src + " " + args.dst
}

func (args *entryPointArgs) String() string {
	return "ENTRYPOINT " + strings.Join(args.cmd, " ")
}

func (args *volumeArgs) String() string {
	return "VOLUME " + strings.Join(args.volumes, " ")
}

func (args *userArgs) String() string {
	return "USER " + args.name
}

func (args *workDirArgs) String() string {
	return "WORKDIR " + args.path
}

func (args *includeArgs) String() string {
	return "INCLUDE " + args.filename
}

func (file *dockerFile) String() string {
	a := make([]string, len(file.instructions))
	for i, _ := range a {
		v := file.instructions[i]
		instr := v.(fmt.Stringer)
		a[i] = instr.String()
	}
	return strings.Join(a, "\n")
}
