// +build !windows

package main

import (
	"encoding/json"
	"net/http"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/request"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestContainersRunWithMultipleHugetlbLimit(c *check.C) {
	testRequires(c, DaemonIsLinux, SameHostDaemon, hugetlbLimitSupport)

	sysInfo := sysinfo.New(true)
	dSize, err := sysInfo.GetDefaultHugepageSize()
	c.Assert(err, check.IsNil)

	data := map[string]interface{}{
		"Image":      "busybox",
		"Cmd":        []string{"/bin/sh", "-c", "ps"},
		"HostConfig": map[string]interface{}{"Hugetlbs": []map[string]interface{}{{"Limit": 1073741824, "PageSize": dSize}, {"Limit": 2147483648, "PageSize": dSize}}},
	}
	status, resp, err := request.Post("/containers/create?name=test1", request.JSONBody(data))
	b, _ := request.ReadBody(resp)
	c.Assert(err, checker.IsNil, check.Commentf(string(b[:])))
	c.Assert(status.StatusCode, checker.Equals, http.StatusCreated, check.Commentf(string(b[:])))

	status, _, err = request.Post("/containers/test1/start")
	c.Assert(err, checker.IsNil)
	c.Assert(status.StatusCode, checker.Equals, http.StatusNoContent)

	body := getInspectBody(c, "v1.31", "test1")
	var inspectJSON types.ContainerJSON
	err = json.Unmarshal(body, &inspectJSON)
	c.Assert(err, checker.IsNil, check.Commentf("Unable to unmarshal body for version v1.26"))
	c.Assert(len(inspectJSON.HostConfig.Hugetlbs), checker.Equals, 1, check.Commentf("expect one hugetlb limit setting, but get %d", len(inspectJSON.HostConfig.Hugetlbs)))
	c.Assert(inspectJSON.HostConfig.Hugetlbs[0].Limit, checker.Equals, uint64(2147483648), check.Commentf("expect hugetlb limit to be 2G, get %d", inspectJSON.HostConfig.Hugetlbs[0].Limit))
}

func (s *DockerSuite) TestContainersRunWithInvalidHugetlbSize(c *check.C) {
	testRequires(c, DaemonIsLinux, SameHostDaemon, hugetlbLimitSupport)

	data := map[string]interface{}{
		"Image":      "busybox",
		"Cmd":        []string{"/bin/sh", "-c", "ps"},
		"HostConfig": map[string]interface{}{"Hugetlbs": []map[string]interface{}{{"Limit": 2147483648, "PageSize": "7MB"}}},
	}
	_, resp, _ := request.Post("/containers/create?name=test1", request.JSONBody(data))
	expected := "Invalid hugepage size:7MB"
	b, _ := request.ReadBody(resp)
	c.Assert(string(b[:]), checker.Contains, expected)
}
