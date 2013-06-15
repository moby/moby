package docker

import (
	"github.com/dotcloud/docker/utils"
	"testing"
)

const Dockerfile = `
# VERSION		0.1
# DOCKER-VERSION	0.2

from   ` + unitTestImageName + `
run    sh -c 'echo root:testpass > /tmp/passwd'
run    mkdir -p /var/run/sshd
`

const DockerfileNoNewLine = `
# VERSION		0.1
# DOCKER-VERSION	0.2

from   ` + unitTestImageName + `
run    sh -c 'echo root:testpass > /tmp/passwd'
run    mkdir -p /var/run/sshd`

// mkTestContext generates a build context from the contents of the provided dockerfile.
// This context is suitable for use as an argument to BuildFile.Build()
func mkTestContext(dockerfile string, t *testing.T) Archive {
	context, err := mkBuildContext(dockerfile)
	if err != nil {
		t.Fatal(err)
	}
	return context
}

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

		imgID, err := buildfile.Build(mkTestContext(Dockerfile, t))
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
	}
}
