package formatter

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/testutil/assert"
)

func TestContainerPsContext(t *testing.T) {
	containerID := stringid.GenerateRandomID()
	unix := time.Now().Add(-65 * time.Second).Unix()

	var ctx containerContext
	cases := []struct {
		container types.Container
		trunc     bool
		expValue  string
		expHeader string
		call      func() string
	}{
		{types.Container{ID: containerID}, true, stringid.TruncateID(containerID), containerIDHeader, ctx.ID},
		{types.Container{ID: containerID}, false, containerID, containerIDHeader, ctx.ID},
		{types.Container{Names: []string{"/foobar_baz"}}, true, "foobar_baz", namesHeader, ctx.Names},
		{types.Container{Image: "ubuntu"}, true, "ubuntu", imageHeader, ctx.Image},
		{types.Container{Image: "verylongimagename"}, true, "verylongimagename", imageHeader, ctx.Image},
		{types.Container{Image: "verylongimagename"}, false, "verylongimagename", imageHeader, ctx.Image},
		{types.Container{
			Image:   "a5a665ff33eced1e0803148700880edab4",
			ImageID: "a5a665ff33eced1e0803148700880edab4269067ed77e27737a708d0d293fbf5",
		},
			true,
			"a5a665ff33ec",
			imageHeader,
			ctx.Image,
		},
		{types.Container{
			Image:   "a5a665ff33eced1e0803148700880edab4",
			ImageID: "a5a665ff33eced1e0803148700880edab4269067ed77e27737a708d0d293fbf5",
		},
			false,
			"a5a665ff33eced1e0803148700880edab4",
			imageHeader,
			ctx.Image,
		},
		{types.Container{Image: ""}, true, "<no image>", imageHeader, ctx.Image},
		{types.Container{Command: "sh -c 'ls -la'"}, true, `"sh -c 'ls -la'"`, commandHeader, ctx.Command},
		{types.Container{Created: unix}, true, time.Unix(unix, 0).String(), createdAtHeader, ctx.CreatedAt},
		{types.Container{Ports: []types.Port{{PrivatePort: 8080, PublicPort: 8080, Type: "tcp"}}}, true, "8080/tcp", portsHeader, ctx.Ports},
		{types.Container{Status: "RUNNING"}, true, "RUNNING", statusHeader, ctx.Status},
		{types.Container{SizeRw: 10}, true, "10 B", sizeHeader, ctx.Size},
		{types.Container{SizeRw: 10, SizeRootFs: 20}, true, "10 B (virtual 20 B)", sizeHeader, ctx.Size},
		{types.Container{}, true, "", labelsHeader, ctx.Labels},
		{types.Container{Labels: map[string]string{"cpu": "6", "storage": "ssd"}}, true, "cpu=6,storage=ssd", labelsHeader, ctx.Labels},
		{types.Container{Created: unix}, true, "About a minute", runningForHeader, ctx.RunningFor},
		{types.Container{
			Mounts: []types.MountPoint{
				{
					Name:   "this-is-a-long-volume-name-and-will-be-truncated-if-trunc-is-set",
					Driver: "local",
					Source: "/a/path",
				},
			},
		}, true, "this-is-a-lo...", mountsHeader, ctx.Mounts},
		{types.Container{
			Mounts: []types.MountPoint{
				{
					Driver: "local",
					Source: "/a/path",
				},
			},
		}, false, "/a/path", mountsHeader, ctx.Mounts},
		{types.Container{
			Mounts: []types.MountPoint{
				{
					Name:   "733908409c91817de8e92b0096373245f329f19a88e2c849f02460e9b3d1c203",
					Driver: "local",
					Source: "/a/path",
				},
			},
		}, false, "733908409c91817de8e92b0096373245f329f19a88e2c849f02460e9b3d1c203", mountsHeader, ctx.Mounts},
	}

	for _, c := range cases {
		ctx = containerContext{c: c.container, trunc: c.trunc}
		v := c.call()
		if strings.Contains(v, ",") {
			compareMultipleValues(t, v, c.expValue)
		} else if v != c.expValue {
			t.Fatalf("Expected %s, was %s\n", c.expValue, v)
		}

		h := ctx.FullHeader()
		if h != c.expHeader {
			t.Fatalf("Expected %s, was %s\n", c.expHeader, h)
		}
	}

	c1 := types.Container{Labels: map[string]string{"com.docker.swarm.swarm-id": "33", "com.docker.swarm.node_name": "ubuntu"}}
	ctx = containerContext{c: c1, trunc: true}

	sid := ctx.Label("com.docker.swarm.swarm-id")
	node := ctx.Label("com.docker.swarm.node_name")
	if sid != "33" {
		t.Fatalf("Expected 33, was %s\n", sid)
	}

	if node != "ubuntu" {
		t.Fatalf("Expected ubuntu, was %s\n", node)
	}

	h := ctx.FullHeader()
	if h != "SWARM ID\tNODE NAME" {
		t.Fatalf("Expected %s, was %s\n", "SWARM ID\tNODE NAME", h)

	}

	c2 := types.Container{}
	ctx = containerContext{c: c2, trunc: true}

	label := ctx.Label("anything.really")
	if label != "" {
		t.Fatalf("Expected an empty string, was %s", label)
	}

	ctx = containerContext{c: c2, trunc: true}
	FullHeader := ctx.FullHeader()
	if FullHeader != "" {
		t.Fatalf("Expected FullHeader to be empty, was %s", FullHeader)
	}

}

func TestContainerContextWrite(t *testing.T) {
	unixTime := time.Now().AddDate(0, 0, -1).Unix()
	expectedTime := time.Unix(unixTime, 0).String()

	cases := []struct {
		context  Context
		expected string
	}{
		// Errors
		{
			Context{Format: "{{InvalidFunction}}"},
			`Template parsing error: template: :1: function "InvalidFunction" not defined
`,
		},
		{
			Context{Format: "{{nil}}"},
			`Template parsing error: template: :1:2: executing "" at <nil>: nil is not a command
`,
		},
		// Table Format
		{
			Context{Format: NewContainerFormat("table", false, true)},
			`CONTAINER ID        IMAGE               COMMAND             CREATED             STATUS              PORTS               NAMES               SIZE
containerID1        ubuntu              ""                  24 hours ago                                                foobar_baz          0 B
containerID2        ubuntu              ""                  24 hours ago                                                foobar_bar          0 B
`,
		},
		{
			Context{Format: NewContainerFormat("table", false, false)},
			`CONTAINER ID        IMAGE               COMMAND             CREATED             STATUS              PORTS               NAMES
containerID1        ubuntu              ""                  24 hours ago                                                foobar_baz
containerID2        ubuntu              ""                  24 hours ago                                                foobar_bar
`,
		},
		{
			Context{Format: NewContainerFormat("table {{.Image}}", false, false)},
			"IMAGE\nubuntu\nubuntu\n",
		},
		{
			Context{Format: NewContainerFormat("table {{.Image}}", false, true)},
			"IMAGE\nubuntu\nubuntu\n",
		},
		{
			Context{Format: NewContainerFormat("table {{.Image}}", true, false)},
			"IMAGE\nubuntu\nubuntu\n",
		},
		{
			Context{Format: NewContainerFormat("table", true, false)},
			"containerID1\ncontainerID2\n",
		},
		// Raw Format
		{
			Context{Format: NewContainerFormat("raw", false, false)},
			fmt.Sprintf(`container_id: containerID1
image: ubuntu
command: ""
created_at: %s
status:
names: foobar_baz
labels:
ports:

container_id: containerID2
image: ubuntu
command: ""
created_at: %s
status:
names: foobar_bar
labels:
ports:

`, expectedTime, expectedTime),
		},
		{
			Context{Format: NewContainerFormat("raw", false, true)},
			fmt.Sprintf(`container_id: containerID1
image: ubuntu
command: ""
created_at: %s
status:
names: foobar_baz
labels:
ports:
size: 0 B

container_id: containerID2
image: ubuntu
command: ""
created_at: %s
status:
names: foobar_bar
labels:
ports:
size: 0 B

`, expectedTime, expectedTime),
		},
		{
			Context{Format: NewContainerFormat("raw", true, false)},
			"container_id: containerID1\ncontainer_id: containerID2\n",
		},
		// Custom Format
		{
			Context{Format: "{{.Image}}"},
			"ubuntu\nubuntu\n",
		},
		{
			Context{Format: NewContainerFormat("{{.Image}}", false, true)},
			"ubuntu\nubuntu\n",
		},
	}

	for _, testcase := range cases {
		containers := []types.Container{
			{ID: "containerID1", Names: []string{"/foobar_baz"}, Image: "ubuntu", Created: unixTime},
			{ID: "containerID2", Names: []string{"/foobar_bar"}, Image: "ubuntu", Created: unixTime},
		}
		out := bytes.NewBufferString("")
		testcase.context.Output = out
		err := ContainerWrite(testcase.context, containers)
		if err != nil {
			assert.Error(t, err, testcase.expected)
		} else {
			assert.Equal(t, out.String(), testcase.expected)
		}
	}
}

func TestContainerContextWriteWithNoContainers(t *testing.T) {
	out := bytes.NewBufferString("")
	containers := []types.Container{}

	contexts := []struct {
		context  Context
		expected string
	}{
		{
			Context{
				Format: "{{.Image}}",
				Output: out,
			},
			"",
		},
		{
			Context{
				Format: "table {{.Image}}",
				Output: out,
			},
			"IMAGE\n",
		},
		{
			Context{
				Format: NewContainerFormat("{{.Image}}", false, true),
				Output: out,
			},
			"",
		},
		{
			Context{
				Format: NewContainerFormat("table {{.Image}}", false, true),
				Output: out,
			},
			"IMAGE\n",
		},
		{
			Context{
				Format: "table {{.Image}}\t{{.Size}}",
				Output: out,
			},
			"IMAGE               SIZE\n",
		},
		{
			Context{
				Format: NewContainerFormat("table {{.Image}}\t{{.Size}}", false, true),
				Output: out,
			},
			"IMAGE               SIZE\n",
		},
	}

	for _, context := range contexts {
		ContainerWrite(context.context, containers)
		assert.Equal(t, context.expected, out.String())
		// Clean buffer
		out.Reset()
	}
}
