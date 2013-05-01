package docker

import (
	"strings"
	"testing"
)

const Dockerfile = `
# VERSION		0.1
# DOCKER-VERSION	0.2

from   docker-ut
run    sh -c 'echo root:testpass > /tmp/passwd'
run    mkdir -p /var/run/sshd
insert https://raw.github.com/dotcloud/docker/master/CHANGELOG.md /tmp/CHANGELOG.md
`

func TestBuild(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	builder := NewBuilder(runtime)

	img, err := builder.Build(strings.NewReader(Dockerfile), &nopWriter{})
	if err != nil {
		t.Fatal(err)
	}

	container, err := builder.Create(
		&Config{
			Image: img.Id,
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
			Image: img.Id,
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
			Image: img.Id,
			Cmd:   []string{"cat", "/tmp/CHANGELOG.md"},
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
	if len(output) == 0 {
		t.Fatal("/tmp/CHANGELOG.md has not been copied")
	}
}
