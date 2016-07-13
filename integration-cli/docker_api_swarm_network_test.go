// +build !windows

package main

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/network"
	"github.com/docker/engine-api/types/swarm"
	"github.com/go-check/check"
)

func (s *DockerSwarmSuite) TestApiSwarmVIP(c *check.C) {
	testRequires(c, IPVSEnabled)

	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, true, false)
	d3 := s.AddDaemon(c, true, false)

	time.Sleep(2 * time.Second)

	ncr := types.NetworkCreateRequest{
		Name: "mynet",
		NetworkCreate: types.NetworkCreate{
			Driver: "overlay",
			IPAM: network.IPAM{
				Driver: "default",
			},
		},
	}
	status, outb, err := d1.SockRequest("POST", "/networks/create", ncr)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated, check.Commentf("output: %q", string(outb)))

	d1.createService(c, simpleTestService, setGlobalMode, func(s *swarm.Service) {
		s.Spec.TaskTemplate.ContainerSpec.Command = []string{"nc", "-ll", "-p", "1234", "-e", "hostname"}
		s.Spec.Name = "myservice"
		s.Spec.Networks = []swarm.NetworkAttachmentConfig{
			{Target: "mynet"},
		}
	})

	allContainers := make(map[string]int)
	allDaemons := []*SwarmDaemon{d1, d2, d3}
	for i, d := range allDaemons {
		waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, 1)
		for _, id := range d.activeContainers() {
			allContainers[id[:12]] = i
		}
	}

	out, err := d1.Cmd("service", "inspect", "-f", "{{(index .Endpoint.VirtualIPs 0).Addr}}", "myservice")
	c.Assert(err, checker.IsNil)
	vip, _, err := net.ParseCIDR(strings.TrimSpace(out))
	c.Assert(err, checker.IsNil)

	id := "client"
	d1.createService(c, simpleTestService, func(s *swarm.Service) {
		s.Spec.Name = id
		s.Spec.Networks = []swarm.NetworkAttachmentConfig{
			{Target: "mynet"},
		}
	})

	waitAndAssert(c, defaultReconciliationTimeout, reducedCheck(sumAsIntegers, d1.checkActiveContainerCount, d2.checkActiveContainerCount, d3.checkActiveContainerCount), checker.Equals, 4)

	tasks := d1.getServiceTasks(c, id)
	c.Assert(tasks, checker.HasLen, 1)
	task := tasks[0]
	c.Assert(task.NodeID, checker.Not(checker.Equals), "")
	c.Assert(task.Status.ContainerStatus.ContainerID, checker.Not(checker.Equals), "")

	// test both vip and service name dns respond from all containers
	for _, host := range []string{"myservice", vip.String()} {
		found := 0
		for i := 0; ; i++ {
			if found == 1<<uint(len(allDaemons))-1 {
				break
			}
			if i > len(allDaemons)*7 {
				c.Fatalf("max attempts reached: %v", found)
			}

			out, err = s.nodeCmd(c, task.NodeID, "exec", task.Status.ContainerStatus.ContainerID, "nc", host+":1234")
			c.Assert(err, checker.IsNil, check.Commentf(out))
			out = strings.TrimSpace(out)
			c.Assert(err, check.IsNil, check.Commentf("output: %s", out))
			index, ok := allContainers[out]
			c.Assert(ok, check.Equals, true, check.Commentf("no container found: %s", out))
			found |= 1 << uint(index)
		}
	}
}

func (s *DockerSwarmSuite) TestApiSwarmExposePort(c *check.C) {
	testRequires(c, IPVSEnabled)

	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, true, false)
	d3 := s.AddDaemon(c, true, false)

	time.Sleep(5 * time.Second)

	d1.createService(c, simpleTestService, setGlobalMode, func(s *swarm.Service) {
		s.Spec.TaskTemplate.ContainerSpec.Command = []string{"nc", "-ll", "-p", "1234", "-e", "hostname"}
		s.Spec.EndpointSpec = &swarm.EndpointSpec{
			Ports: []swarm.PortConfig{{
				Protocol:      "tcp",
				TargetPort:    1234,
				PublishedPort: 2345,
			}},
		}
	})

	allContainers := make(map[string]int)
	allDaemons := []*SwarmDaemon{d1, d2, d3}
	for i, d := range allDaemons {
		waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, 1)
		for _, id := range d.activeContainers() {
			allContainers[id[:12]] = i
		}
	}
	for _, d := range allDaemons {
		waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, 1)
		time.Sleep(1 * time.Second)
		found := 0
		for i := 0; ; i++ {
			if found == 1<<uint(len(allDaemons))-1 {
				break
			}
			if i > len(allDaemons)*7 {
				c.Fatalf("max attempts reached: %v", found)
			}
			conn, err := net.Dial("tcp", d.ip+":2345")
			c.Assert(err, check.IsNil)
			b := &bytes.Buffer{}
			_, err = io.Copy(b, conn)
			out := strings.TrimSpace(b.String())
			c.Assert(err, check.IsNil, check.Commentf("output: %s", out))
			index, ok := allContainers[out]
			c.Assert(ok, check.Equals, true, check.Commentf("no container found: %s", out))
			found |= 1 << uint(index)
		}
	}

}
