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
maintainer docker
expose 22
env    FOO BAR
cmd    ["echo", "hello", "world"]
run    sh -c 'echo root:testpass > /tmp/passwd'
run    mkdir -p /var/run/sshd
add    . /src
`

const DockerfileNoNewLine = `
# VERSION		0.1
# DOCKER-VERSION	0.2

from   ` + unitTestImageName + `
maintainer docker
expose 22
env    FOO BAR
cmd    ["echo", "hello", "world"]
run    sh -c 'echo root:testpass > /tmp/passwd'
run    mkdir -p /var/run/sshd
add    . /src`

func TestBuildFile(t *testing.T) {
	dockerfiles := []string{Dockerfile, DockerfileNoNewLine}
	for _, Dockerfile := range dockerfiles {
		runtime, err := newTestRuntime()
		if err != nil {
			t.Fatal(err)
		}
		defer nuke(runtime)

		srv := &Server{runtime: runtime}

		buildfile := NewBuildFile(srv, &utils.NopWriter{})

		context, err := fakeTar()
		if err != nil {
			t.Fatal(err)
		}

		imgID, err := buildfile.Build(strings.NewReader(Dockerfile), context)
		if err != nil {
			t.Fatal(err)
		}

		builder := NewBuilder(runtime)
		container, err := builder.Create(
			&Config{
				Image: imgID,
				Cmd:   []string{"cat", "/tmp/passwd"},
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		defer runtime.Destroy(container)

		output, err := container.Output()
		if err != nil {
			t.Fatal(err)
		}
		if string(output) != "root:testpass\n" {
			t.Fatalf("Unexpected output. Read '%s', expected '%s'", output, "root:testpass\n")
		}

		container2, err := builder.Create(
			&Config{
				Image: imgID,
				Cmd:   []string{"ls", "-d", "/var/run/sshd"},
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		defer runtime.Destroy(container2)

		output, err = container2.Output()
		if err != nil {
			t.Fatal(err)
		}
		if string(output) != "/var/run/sshd\n" {
			t.Fatal("/var/run/sshd has not been created")
		}

		container3, err := builder.Create(
			&Config{
				Image: imgID,
				Cmd:   []string{"ls", "/src"},
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		defer runtime.Destroy(container3)

		output, err = container3.Output()
		if err != nil {
			t.Fatal(err)
		}
		if string(output) != "etc\nvar\n" {
			t.Fatalf("Unexpected output. Expected: '%s', received: '%s'", "etc\nvar\n", string(output))
		}

	}
}
