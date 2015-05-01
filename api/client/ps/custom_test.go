package ps

import (
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stringid"
)

func TestContainerContextID(t *testing.T) {
	containerId := stringid.GenerateRandomID()
	unix := time.Now().Unix()

	var ctx containerContext
	cases := []struct {
		container types.Container
		trunc     bool
		expValue  string
		expHeader string
		call      func() string
	}{
		{types.Container{ID: containerId}, true, stringid.TruncateID(containerId), idHeader, ctx.ID},
		{types.Container{Names: []string{"/foobar_baz"}}, true, "foobar_baz", namesHeader, ctx.Names},
		{types.Container{Image: "ubuntu"}, true, "ubuntu", imageHeader, ctx.Image},
		{types.Container{Image: ""}, true, "<no image>", imageHeader, ctx.Image},
		{types.Container{Command: "sh -c 'ls -la'"}, true, `"sh -c 'ls -la'"`, commandHeader, ctx.Command},
		{types.Container{Created: int(unix)}, true, time.Unix(unix, 0).String(), createdAtHeader, ctx.CreatedAt},
		{types.Container{Ports: []types.Port{types.Port{PrivatePort: 8080, PublicPort: 8080, Type: "tcp"}}}, true, "8080/tcp", portsHeader, ctx.Ports},
		{types.Container{Status: "RUNNING"}, true, "RUNNING", statusHeader, ctx.Status},
		{types.Container{SizeRw: 10}, true, "10 B", sizeHeader, ctx.Size},
		{types.Container{SizeRw: 10, SizeRootFs: 20}, true, "10 B (virtual 20 B)", sizeHeader, ctx.Size},
		{types.Container{Labels: map[string]string{"cpu": "6", "storage": "ssd"}}, true, "cpu=6,storage=ssd", labelsHeader, ctx.Labels},
	}

	for _, c := range cases {
		ctx = containerContext{c: c.container, trunc: c.trunc}
		v := c.call()
		if v != c.expValue {
			t.Fatalf("Expected %s, was %s\n", c.expValue, v)
		}

		h := ctx.fullHeader()
		if h != c.expHeader {
			t.Fatalf("Expected %s, was %s\n", c.expHeader, h)
		}
	}

	c := types.Container{Labels: map[string]string{"com.docker.swarm.swarm-id": "33", "com.docker.swarm.node_name": "ubuntu"}}
	ctx = containerContext{c: c, trunc: true}

	sid := ctx.Label("com.docker.swarm.swarm-id")
	node := ctx.Label("com.docker.swarm.node_name")
	if sid != "33" {
		t.Fatal("Expected 33, was %s\n", sid)
	}

	if node != "ubuntu" {
		t.Fatal("Expected ubuntu, was %s\n", node)
	}

	h := ctx.fullHeader()
	if h != "SWARM ID\tNODE NAME" {
		t.Fatal("Expected %s, was %s\n", "SWARM ID\tNODE NAME", h)

	}

}
