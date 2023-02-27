package main

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/docker/docker/testutil/registry"
	"gotest.tools/v3/assert"
)

func makefile(path string, contents string) (string, error) {
	f, err := os.CreateTemp(path, "tmp")
	if err != nil {
		return "", err
	}
	err = os.WriteFile(f.Name(), []byte(contents), os.ModePerm)
	if err != nil {
		return "", err
	}
	return f.Name(), nil
}

// TestV2Only ensures that a daemon does not
// attempt to contact any v1 registry endpoints.
func (s *DockerRegistrySuite) TestV2Only(c *testing.T) {
	reg, err := registry.NewMock(c)
	assert.NilError(c, err)
	defer reg.Close()

	reg.RegisterHandler("/v2/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})

	reg.RegisterHandler("/v1/.*", func(w http.ResponseWriter, r *http.Request) {
		c.Fatal("V1 registry contacted")
	})

	repoName := fmt.Sprintf("%s/busybox", reg.URL())

	s.d.Start(c, "--insecure-registry", reg.URL())

	tmp, err := os.MkdirTemp("", "integration-cli-")
	assert.NilError(c, err)
	defer os.RemoveAll(tmp)

	dockerfileName, err := makefile(tmp, fmt.Sprintf("FROM %s/busybox", reg.URL()))
	assert.NilError(c, err, "Unable to create test dockerfile")

	_, _ = s.d.Cmd("build", "--file", dockerfileName, tmp)
	_, _ = s.d.Cmd("run", repoName)
	_, _ = s.d.Cmd("login", "-u", "richard", "-p", "testtest", reg.URL())
	_, _ = s.d.Cmd("tag", "busybox", repoName)
	_, _ = s.d.Cmd("push", repoName)
	_, _ = s.d.Cmd("pull", repoName)
}
