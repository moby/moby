package docker

import (
	"github.com/dotcloud/docker/utils"
	"strings"
	"testing"
)

const Dockerfile = `
# VERSION		0.1
# DOCKER-VERSION	0.2

from   ` + unitTestImageName + `
run    sh -c 'echo root:testpass > /tmp/passwd'
run    mkdir -p /var/run/sshd
run    [ "$(cat /tmp/passwd)" = "root:testpass" ]
run    [ "$(ls -d /var/run/sshd)" = "/var/run/sshd" ]
`

const DockerfileNoNewLine = `
# VERSION		0.1
# DOCKER-VERSION	0.2

from   ` + unitTestImageName + `
run    sh -c 'echo root:testpass > /tmp/passwd'
run    mkdir -p /var/run/sshd
run    [ "$(cat /tmp/passwd)" = "root:testpass" ]
run    [ "$(ls -d /var/run/sshd)" = "/var/run/sshd" ]`

// FIXME: test building with a context

// FIXME: test building with a local ADD as first command

// FIXME: test building with 2 successive overlapping ADD commands

func TestBuild(t *testing.T) {
	dockerfiles := []string{Dockerfile, DockerfileNoNewLine}
	for _, Dockerfile := range dockerfiles {
		runtime, err := newTestRuntime()
		if err != nil {
			t.Fatal(err)
		}
		defer nuke(runtime)

		srv := &Server{runtime: runtime}

		buildfile := NewBuildFile(srv, &utils.NopWriter{})

		if _, err := buildfile.Build(strings.NewReader(Dockerfile), nil); err != nil {
			t.Fatal(err)
		}
	}
}
