package main

import (
	"encoding/json"
	"net/http"
	"path"

	"github.com/docker/docker/api/types"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestVolumesApiList(c *check.C) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "-v", "/foo", "busybox")

	status, b, err := sockRequest("GET", "/volumes", nil)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusOK)

	var volumes types.VolumesListResponse
	c.Assert(json.Unmarshal(b, &volumes), check.IsNil)

	c.Assert(len(volumes.Volumes), check.Equals, 1, check.Commentf("\n%v", volumes.Volumes))
}

func (s *DockerSuite) TestVolumesApiCreate(c *check.C) {
	testRequires(c, DaemonIsLinux)
	config := types.VolumeCreateRequest{
		Name: "test",
	}
	status, b, err := sockRequest("POST", "/volumes", config)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusCreated, check.Commentf(string(b)))

	var vol types.Volume
	err = json.Unmarshal(b, &vol)
	c.Assert(err, check.IsNil)

	c.Assert(path.Base(path.Dir(vol.Mountpoint)), check.Equals, config.Name)
}

func (s *DockerSuite) TestVolumesApiRemove(c *check.C) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "-v", "/foo", "--name=test", "busybox")

	status, b, err := sockRequest("GET", "/volumes", nil)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusOK)

	var volumes types.VolumesListResponse
	c.Assert(json.Unmarshal(b, &volumes), check.IsNil)
	c.Assert(len(volumes.Volumes), check.Equals, 1, check.Commentf("\n%v", volumes.Volumes))

	v := volumes.Volumes[0]
	status, _, err = sockRequest("DELETE", "/volumes/"+v.Name, nil)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusConflict, check.Commentf("Should not be able to remove a volume that is in use"))

	dockerCmd(c, "rm", "-f", "test")
	status, data, err := sockRequest("DELETE", "/volumes/"+v.Name, nil)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusNoContent, check.Commentf(string(data)))

}

func (s *DockerSuite) TestVolumesApiInspect(c *check.C) {
	testRequires(c, DaemonIsLinux)
	config := types.VolumeCreateRequest{
		Name: "test",
	}
	status, b, err := sockRequest("POST", "/volumes", config)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusCreated, check.Commentf(string(b)))

	status, b, err = sockRequest("GET", "/volumes", nil)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusOK, check.Commentf(string(b)))

	var volumes types.VolumesListResponse
	c.Assert(json.Unmarshal(b, &volumes), check.IsNil)
	c.Assert(len(volumes.Volumes), check.Equals, 1, check.Commentf("\n%v", volumes.Volumes))

	var vol types.Volume
	status, b, err = sockRequest("GET", "/volumes/"+config.Name, nil)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusOK, check.Commentf(string(b)))
	c.Assert(json.Unmarshal(b, &vol), check.IsNil)
	c.Assert(vol.Name, check.Equals, config.Name)
}
