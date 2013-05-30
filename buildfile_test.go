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
`

func TestBuild(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	buildfile := NewBuildFile(srv, &utils.NopWriter{})

	imgId, err := buildfile.Build(strings.NewReader(Dockerfile), nil)
	if err != nil {
		t.Fatal(err)
	}

	builder := NewBuilder(runtime)
	container, err := builder.Create(
		&Config{
			Image: imgId,
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
			Image: imgId,
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
}
