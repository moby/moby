package logger

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Context provides enough information for a logging driver to do its function.
type Context struct {
	Config              map[string]string
	ContainerID         string
	ContainerName       string
	ContainerEntrypoint string
	ContainerArgs       []string
	ContainerImageID    string
	ContainerImageName  string
	ContainerCreated    time.Time
	LogPath             string
}

// Hostname returns the hostname from the underlying OS.
func (ctx *Context) Hostname() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("logger: can not resolve hostname: %v", err)
	}
	return hostname, nil
}

// Command returns the command that the container being logged was
// started with. The Entrypoint is prepended to the container
// arguments.
func (ctx *Context) Command() string {
	terms := []string{ctx.ContainerEntrypoint}
	for _, arg := range ctx.ContainerArgs {
		terms = append(terms, arg)
	}
	command := strings.Join(terms, " ")
	return command
}

// ID Returns the Container ID shortened to 12 characters.
func (ctx *Context) ID() string {
	return ctx.ContainerID[:12]
}

// FullID is an alias of ContainerID.
func (ctx *Context) FullID() string {
	return ctx.ContainerID
}

// Name returns the ContainerName without a preceding '/'.
func (ctx *Context) Name() string {
	return ctx.ContainerName[1:]
}

// ImageID returns the ContainerImageID shortened to 12 characters.
func (ctx *Context) ImageID() string {
	return ctx.ContainerImageID[:12]
}

// ImageFullID is an alias of ContainerID.
func (ctx *Context) ImageFullID() string {
	return ctx.ContainerImageID
}

// ImageName is an alias of ContainerImageName
func (ctx *Context) ImageName() string {
	return ctx.ContainerImageName
}
