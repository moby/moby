package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/docker/docker/internal/test/registry"
	"github.com/go-check/check"
	"github.com/pkg/errors"
	"gotest.tools/assert"
)

func makefile(path string, contents string) (string, error) {
	f, err := ioutil.TempFile(path, "tmp")
	if err != nil {
		return "", err
	}
	err = ioutil.WriteFile(f.Name(), []byte(contents), os.ModePerm)
	if err != nil {
		return "", err
	}
	return f.Name(), nil
}

// TestV2Only ensures that a daemon does not
// attempt to contact any v1 registry endpoints.
func (s *DockerRegistrySuite) TestV2Only(c *check.C) {

	reg, err := registry.NewMock(c)
	defer reg.Close()
	assert.NilError(c, err)

	chErr := make(chan error, 10)
	reg.RegisterHandler("/v2/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})

	reg.RegisterHandler("/v1/.*", func(w http.ResponseWriter, r *http.Request) {
		// this is in a goroutine, so send it to the error chan which we'll check later
		chErr <- errors.New("V1 registry contacted")
	})

	repoName := fmt.Sprintf("%s/busybox", reg.URL())

	tmp, err := ioutil.TempDir("", "integration-cli-")
	assert.NilError(c, err)
	defer os.RemoveAll(tmp)

	dockerfileName, err := makefile(tmp, fmt.Sprintf("FROM %s/busybox", reg.URL()))
	assert.NilError(c, err, "Unable to create test dockerfile")

	// So this test is just making sure that the registry v1 endpoint is not contacted
	// It would be nice if the errors from these commands are actually checked.
	// Maybe this test would be best suited in integration/ instead of integration-cli

	dockerCmdWithError("build", "--file", dockerfileName, tmp)

	dockerCmdWithError("run", repoName)
	dockerCmdWithError("login", "-u", "richard", "-p", "testtest", reg.URL())
	dockerCmdWithError("tag", "busybox", repoName)
	dockerCmdWithError("push", repoName)
	dockerCmdWithError("pull", repoName)

	close(chErr)
	assert.NilError(c, err)
}
