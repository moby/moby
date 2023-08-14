// This file will be removed when we completely drop support for
// passing HostConfig to container start API.

package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func formatV123StartAPIURL(url string) string {
	return "/v1.23" + url
}

func (s *DockerAPISuite) TestDeprecatedContainerAPIStartHostConfig(c *testing.T) {
	name := "test-deprecated-api-124"
	dockerCmd(c, "create", "--name", name, "busybox")
	config := map[string]interface{}{
		"Binds": []string{"/aa:/bb"},
	}
	res, body, err := request.Post(testutil.GetContext(c), "/containers/"+name+"/start", request.JSONBody(config))
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusBadRequest)
	if versions.GreaterThanOrEqualTo(testEnv.DaemonAPIVersion(), "1.32") {
		// assertions below won't work before 1.32
		buf, err := request.ReadBody(body)
		assert.NilError(c, err)

		assert.Equal(c, res.StatusCode, http.StatusBadRequest)
		assert.Assert(c, strings.Contains(string(buf), "was deprecated since API v1.22"))
	}
}

func (s *DockerAPISuite) TestDeprecatedContainerAPIStartVolumeBinds(c *testing.T) {
	// TODO Windows CI: Investigate further why this fails on Windows to Windows CI.
	testRequires(c, DaemonIsLinux)
	path := "/foo"
	if testEnv.DaemonInfo.OSType == "windows" {
		path = `c:\foo`
	}
	name := "testing"
	config := map[string]interface{}{
		"Image":   "busybox",
		"Volumes": map[string]struct{}{path: {}},
	}

	res, _, err := request.Post(testutil.GetContext(c), formatV123StartAPIURL("/containers/create?name="+name), request.JSONBody(config))
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusCreated)

	bindPath := RandomTmpDirPath("test", testEnv.DaemonInfo.OSType)
	config = map[string]interface{}{
		"Binds": []string{bindPath + ":" + path},
	}
	res, _, err = request.Post(testutil.GetContext(c), formatV123StartAPIURL("/containers/"+name+"/start"), request.JSONBody(config))
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusNoContent)

	pth, err := inspectMountSourceField(name, path)
	assert.NilError(c, err)
	assert.Equal(c, pth, bindPath, "expected volume host path to be %s, got %s", bindPath, pth)
}

// Test for GH#10618
func (s *DockerAPISuite) TestDeprecatedContainerAPIStartDupVolumeBinds(c *testing.T) {
	// TODO Windows to Windows CI - Port this
	testRequires(c, DaemonIsLinux)
	name := "testdups"
	config := map[string]interface{}{
		"Image":   "busybox",
		"Volumes": map[string]struct{}{"/tmp": {}},
	}

	res, _, err := request.Post(testutil.GetContext(c), formatV123StartAPIURL("/containers/create?name="+name), request.JSONBody(config))
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusCreated)

	bindPath1 := RandomTmpDirPath("test1", testEnv.DaemonInfo.OSType)
	bindPath2 := RandomTmpDirPath("test2", testEnv.DaemonInfo.OSType)

	config = map[string]interface{}{
		"Binds": []string{bindPath1 + ":/tmp", bindPath2 + ":/tmp"},
	}
	res, body, err := request.Post(testutil.GetContext(c), formatV123StartAPIURL("/containers/"+name+"/start"), request.JSONBody(config))
	assert.NilError(c, err)

	buf, err := request.ReadBody(body)
	assert.NilError(c, err)

	if versions.LessThan(testEnv.DaemonAPIVersion(), "1.32") {
		assert.Equal(c, res.StatusCode, http.StatusInternalServerError)
	} else {
		assert.Equal(c, res.StatusCode, http.StatusBadRequest)
	}
	assert.Assert(c, strings.Contains(string(buf), "Duplicate mount point"), "Expected failure due to duplicate bind mounts to same path, instead got: %q with error: %v", string(buf), err)
}

func (s *DockerAPISuite) TestDeprecatedContainerAPIStartVolumesFrom(c *testing.T) {
	// TODO Windows to Windows CI - Port this
	testRequires(c, DaemonIsLinux)
	volName := "voltst"
	volPath := "/tmp"

	dockerCmd(c, "run", "--name", volName, "-v", volPath, "busybox")

	name := "TestContainerAPIStartVolumesFrom"
	config := map[string]interface{}{
		"Image":   "busybox",
		"Volumes": map[string]struct{}{volPath: {}},
	}

	res, _, err := request.Post(testutil.GetContext(c), formatV123StartAPIURL("/containers/create?name="+name), request.JSONBody(config))
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusCreated)

	config = map[string]interface{}{
		"VolumesFrom": []string{volName},
	}
	res, _, err = request.Post(testutil.GetContext(c), formatV123StartAPIURL("/containers/"+name+"/start"), request.JSONBody(config))
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusNoContent)

	pth, err := inspectMountSourceField(name, volPath)
	assert.NilError(c, err)
	pth2, err := inspectMountSourceField(volName, volPath)
	assert.NilError(c, err)
	assert.Equal(c, pth, pth2, "expected volume host path to be %s, got %s", pth, pth2)
}

// #9981 - Allow a docker created volume (ie, one in /var/lib/docker/volumes) to be used to overwrite (via passing in Binds on api start) an existing volume
func (s *DockerAPISuite) TestDeprecatedPostContainerBindNormalVolume(c *testing.T) {
	// TODO Windows to Windows CI - Port this
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "create", "-v", "/foo", "--name=one", "busybox")

	fooDir, err := inspectMountSourceField("one", "/foo")
	assert.NilError(c, err)

	dockerCmd(c, "create", "-v", "/foo", "--name=two", "busybox")

	bindSpec := map[string][]string{"Binds": {fooDir + ":/foo"}}
	res, _, err := request.Post(testutil.GetContext(c), formatV123StartAPIURL("/containers/two/start"), request.JSONBody(bindSpec))
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusNoContent)

	fooDir2, err := inspectMountSourceField("two", "/foo")
	assert.NilError(c, err)
	assert.Equal(c, fooDir2, fooDir, "expected volume path to be %s, got: %s", fooDir, fooDir2)
}

func (s *DockerAPISuite) TestDeprecatedStartWithTooLowMemoryLimit(c *testing.T) {
	// TODO Windows: Port once memory is supported
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "create", "busybox")

	containerID := strings.TrimSpace(out)

	config := `{
                "CpuShares": 100,
                "Memory":    524287
        }`

	res, body, err := request.Post(testutil.GetContext(c), formatV123StartAPIURL("/containers/"+containerID+"/start"), request.RawString(config), request.JSON)
	assert.NilError(c, err)
	b, err := request.ReadBody(body)
	assert.NilError(c, err)
	if versions.LessThan(testEnv.DaemonAPIVersion(), "1.32") {
		assert.Equal(c, res.StatusCode, http.StatusInternalServerError)
	} else {
		assert.Equal(c, res.StatusCode, http.StatusBadRequest)
	}
	assert.Assert(c, is.Contains(string(b), "Minimum memory limit allowed is 6MB"))
}

// #14640
func (s *DockerAPISuite) TestDeprecatedPostContainersStartWithoutLinksInHostConfig(c *testing.T) {
	// TODO Windows: Windows doesn't support supplying a hostconfig on start.
	// An alternate test could be written to validate the negative testing aspect of this
	testRequires(c, DaemonIsLinux)
	name := "test-host-config-links"
	dockerCmd(c, append([]string{"create", "--name", name, "busybox"}, sleepCommandForDaemonPlatform()...)...)

	hc := inspectFieldJSON(c, name, "HostConfig")
	config := `{"HostConfig":` + hc + `}`

	res, b, err := request.Post(testutil.GetContext(c), formatV123StartAPIURL("/containers/"+name+"/start"), request.RawString(config), request.JSON)
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusNoContent)
	b.Close()
}

// #14640
func (s *DockerAPISuite) TestDeprecatedPostContainersStartWithLinksInHostConfig(c *testing.T) {
	// TODO Windows: Windows doesn't support supplying a hostconfig on start.
	// An alternate test could be written to validate the negative testing aspect of this
	testRequires(c, DaemonIsLinux)
	name := "test-host-config-links"
	dockerCmd(c, "run", "--name", "foo", "-d", "busybox", "top")
	dockerCmd(c, "create", "--name", name, "--link", "foo:bar", "busybox", "top")

	hc := inspectFieldJSON(c, name, "HostConfig")
	config := `{"HostConfig":` + hc + `}`

	res, b, err := request.Post(testutil.GetContext(c), formatV123StartAPIURL("/containers/"+name+"/start"), request.RawString(config), request.JSON)
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusNoContent)
	b.Close()
}

// #14640
func (s *DockerAPISuite) TestDeprecatedPostContainersStartWithLinksInHostConfigIdLinked(c *testing.T) {
	// Windows does not support links
	testRequires(c, DaemonIsLinux)
	name := "test-host-config-links"
	out, _ := dockerCmd(c, "run", "--name", "link0", "-d", "busybox", "top")
	defer dockerCmd(c, "stop", "link0")
	id := strings.TrimSpace(out)
	dockerCmd(c, "create", "--name", name, "--link", id, "busybox", "top")
	defer dockerCmd(c, "stop", name)

	hc := inspectFieldJSON(c, name, "HostConfig")
	config := `{"HostConfig":` + hc + `}`

	res, b, err := request.Post(testutil.GetContext(c), formatV123StartAPIURL("/containers/"+name+"/start"), request.RawString(config), request.JSON)
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusNoContent)
	b.Close()
}

func (s *DockerAPISuite) TestDeprecatedStartWithNilDNS(c *testing.T) {
	// TODO Windows: Add once DNS is supported
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "create", "busybox")
	containerID := strings.TrimSpace(out)

	config := `{"HostConfig": {"Dns": null}}`

	res, b, err := request.Post(testutil.GetContext(c), formatV123StartAPIURL("/containers/"+containerID+"/start"), request.RawString(config), request.JSON)
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusNoContent)
	b.Close()

	dns := inspectFieldJSON(c, containerID, "HostConfig.Dns")
	assert.Equal(c, dns, "[]")
}
