package main

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestVolumesAPIList(c *check.C) {
	prefix, _ := getPrefixAndSlashFromDaemonPlatform()
	dockerCmd(c, "run", "-v", prefix+"/foo", "busybox")

	status, b, err := sockRequest("GET", "/volumes", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	var volumes types.VolumesListResponse
	c.Assert(json.Unmarshal(b, &volumes), checker.IsNil)

	c.Assert(len(volumes.Volumes), checker.Equals, 1, check.Commentf("\n%v", volumes.Volumes))
}

func (s *DockerSuite) TestVolumesAPICreate(c *check.C) {
	config := types.VolumeCreateRequest{
		Name: "test",
	}
	status, b, err := sockRequest("POST", "/volumes/create", config)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusCreated, check.Commentf(string(b)))

	var vol types.Volume
	err = json.Unmarshal(b, &vol)
	c.Assert(err, checker.IsNil)

	c.Assert(filepath.Base(filepath.Dir(vol.Mountpoint)), checker.Equals, config.Name)
}

func (s *DockerSuite) TestVolumesAPIRemove(c *check.C) {
	prefix, _ := getPrefixAndSlashFromDaemonPlatform()
	dockerCmd(c, "run", "-v", prefix+"/foo", "--name=test", "busybox")

	status, b, err := sockRequest("GET", "/volumes", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK)

	var volumes types.VolumesListResponse
	c.Assert(json.Unmarshal(b, &volumes), checker.IsNil)
	c.Assert(len(volumes.Volumes), checker.Equals, 1, check.Commentf("\n%v", volumes.Volumes))

	v := volumes.Volumes[0]
	status, _, err = sockRequest("DELETE", "/volumes/"+v.Name, nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusConflict, check.Commentf("Should not be able to remove a volume that is in use"))

	dockerCmd(c, "rm", "-f", "test")
	status, data, err := sockRequest("DELETE", "/volumes/"+v.Name, nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusNoContent, check.Commentf(string(data)))

}

func (s *DockerSuite) TestVolumesAPIInspect(c *check.C) {
	config := types.VolumeCreateRequest{
		Name: "test",
	}
	status, b, err := sockRequest("POST", "/volumes/create", config)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusCreated, check.Commentf(string(b)))

	status, b, err = sockRequest("GET", "/volumes", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf(string(b)))

	var volumes types.VolumesListResponse
	c.Assert(json.Unmarshal(b, &volumes), checker.IsNil)
	c.Assert(len(volumes.Volumes), checker.Equals, 1, check.Commentf("\n%v", volumes.Volumes))

	var vol types.Volume
	status, b, err = sockRequest("GET", "/volumes/"+config.Name, nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf(string(b)))
	c.Assert(json.Unmarshal(b, &vol), checker.IsNil)
	c.Assert(vol.Name, checker.Equals, config.Name)
}
