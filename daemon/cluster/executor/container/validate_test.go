package container

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/docker/docker/daemon"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/swarmkit/api"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func newTestControllerWithMount(m api.Mount) (*controller, error) {
	return newController(&daemon.Daemon{}, &api.Task{
		ID:        stringid.GenerateRandomID(),
		ServiceID: stringid.GenerateRandomID(),
		Spec: api.TaskSpec{
			Runtime: &api.TaskSpec_Container{
				Container: &api.ContainerSpec{
					Image: "image_name",
					Labels: map[string]string{
						"com.docker.swarm.task.id": "id",
					},
					Mounts: []api.Mount{m},
				},
			},
		},
	})
}

func (s *DockerSuite) TestControllerValidateMountBind(c *check.C) {
	// with improper source
	if _, err := newTestControllerWithMount(api.Mount{
		Type:   api.MountTypeBind,
		Source: "foo",
		Target: testAbsPath,
	}); err == nil || !strings.Contains(err.Error(), "invalid bind mount source") {
		c.Fatalf("expected  error, got: %v", err)
	}

	// with non-existing source
	if _, err := newTestControllerWithMount(api.Mount{
		Type:   api.MountTypeBind,
		Source: "/some-non-existing-host-path/",
		Target: testAbsPath,
	}); err == nil || !strings.Contains(err.Error(), "invalid bind mount source") {
		c.Fatalf("expected  error, got: %v", err)
	}

	// with proper source
	tmpdir, err := ioutil.TempDir("", "TestControllerValidateMountBind")
	if err != nil {
		c.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.Remove(tmpdir)

	if _, err := newTestControllerWithMount(api.Mount{
		Type:   api.MountTypeBind,
		Source: tmpdir,
		Target: testAbsPath,
	}); err != nil {
		c.Fatalf("expected  error, got: %v", err)
	}
}

func (s *DockerSuite) TestControllerValidateMountVolume(c *check.C) {
	// with improper source
	if _, err := newTestControllerWithMount(api.Mount{
		Type:   api.MountTypeVolume,
		Source: testAbsPath,
		Target: testAbsPath,
	}); err == nil || !strings.Contains(err.Error(), "invalid volume mount source") {
		c.Fatalf("expected error, got: %v", err)
	}

	// with proper source
	if _, err := newTestControllerWithMount(api.Mount{
		Type:   api.MountTypeVolume,
		Source: "foo",
		Target: testAbsPath,
	}); err != nil {
		c.Fatalf("expected error, got: %v", err)
	}
}

func (s *DockerSuite) TestControllerValidateMountTarget(c *check.C) {
	tmpdir, err := ioutil.TempDir("", "TestControllerValidateMountTarget")
	if err != nil {
		c.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.Remove(tmpdir)

	// with improper target
	if _, err := newTestControllerWithMount(api.Mount{
		Type:   api.MountTypeBind,
		Source: testAbsPath,
		Target: "foo",
	}); err == nil || !strings.Contains(err.Error(), "invalid mount target") {
		c.Fatalf("expected error, got: %v", err)
	}

	// with proper target
	if _, err := newTestControllerWithMount(api.Mount{
		Type:   api.MountTypeBind,
		Source: tmpdir,
		Target: testAbsPath,
	}); err != nil {
		c.Fatalf("expected no error, got: %v", err)
	}
}

func (s *DockerSuite) TestControllerValidateMountTmpfs(c *check.C) {
	// with improper target
	if _, err := newTestControllerWithMount(api.Mount{
		Type:   api.MountTypeTmpfs,
		Source: "foo",
		Target: testAbsPath,
	}); err == nil || !strings.Contains(err.Error(), "invalid tmpfs source") {
		c.Fatalf("expected error, got: %v", err)
	}

	// with proper target
	if _, err := newTestControllerWithMount(api.Mount{
		Type:   api.MountTypeTmpfs,
		Target: testAbsPath,
	}); err != nil {
		c.Fatalf("expected no error, got: %v", err)
	}
}

func (s *DockerSuite) TestControllerValidateMountInvalidType(c *check.C) {
	// with improper target
	if _, err := newTestControllerWithMount(api.Mount{
		Type:   api.Mount_MountType(9999),
		Source: "foo",
		Target: testAbsPath,
	}); err == nil || !strings.Contains(err.Error(), "invalid mount type") {
		c.Fatalf("expected error, got: %v", err)
	}
}
