package ps

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stringid"
)

func TestContainerPsContext(t *testing.T) {
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
		{types.Container{Ports: []types.Port{{PrivatePort: 8080, PublicPort: 8080, Type: "tcp"}}}, true, "8080/tcp", portsHeader, ctx.Ports},
		{types.Container{Status: "RUNNING"}, true, "RUNNING", statusHeader, ctx.Status},
		{types.Container{SizeRw: 10}, true, "10 B", sizeHeader, ctx.Size},
		{types.Container{SizeRw: 10, SizeRootFs: 20}, true, "10 B (virtual 20 B)", sizeHeader, ctx.Size},
		{types.Container{Labels: map[string]string{"cpu": "6", "storage": "ssd"}}, true, "cpu=6,storage=ssd", labelsHeader, ctx.Labels},
	}

	for _, c := range cases {
		ctx = containerContext{c: c.container, trunc: c.trunc}
		v := c.call()
		if strings.Contains(v, ",") {
			// comma-separated values means probably a map input, which won't
			// be guaranteed to have the same order as our expected value
			// We'll create maps and use reflect.DeepEquals to check instead:
			entriesMap := make(map[string]string)
			expMap := make(map[string]string)
			entries := strings.Split(v, ",")
			expectedEntries := strings.Split(c.expValue, ",")
			for _, entry := range entries {
				keyval := strings.Split(entry, "=")
				entriesMap[keyval[0]] = keyval[1]
			}
			for _, expected := range expectedEntries {
				keyval := strings.Split(expected, "=")
				expMap[keyval[0]] = keyval[1]
			}
			if !reflect.DeepEqual(expMap, entriesMap) {
				t.Fatalf("Expected entries: %v, got: %v", c.expValue, v)
			}
		} else if v != c.expValue {
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
		t.Fatalf("Expected 33, was %s\n", sid)
	}

	if node != "ubuntu" {
		t.Fatalf("Expected ubuntu, was %s\n", node)
	}

	h := ctx.fullHeader()
	if h != "SWARM ID\tNODE NAME" {
		t.Fatalf("Expected %s, was %s\n", "SWARM ID\tNODE NAME", h)

	}
}

func TestContainerPsFormatError(t *testing.T) {
	out := bytes.NewBufferString("")
	ctx := Context{
		Format: "{{InvalidFunction}}",
		Output: out,
	}

	customFormat(ctx, make([]types.Container, 0))
	if out.String() != "Template parsing error: template: :1: function \"InvalidFunction\" not defined\n" {
		t.Fatalf("Expected format error, got `%v`\n", out.String())
	}
}
