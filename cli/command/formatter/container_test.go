package formatter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stringid"
	"github.com/stretchr/testify/assert"
)

func TestContainerPsContext(t *testing.T) {
	containerID := stringid.GenerateRandomID()
	unix := time.Now().Add(-65 * time.Second).Unix()

	var ctx containerContext
	cases := []struct {
		container types.Container
		trunc     bool
		expValue  string
		call      func() string
	}{
		{types.Container{ID: containerID}, true, stringid.TruncateID(containerID), ctx.ID},
		{types.Container{ID: containerID}, false, containerID, ctx.ID},
		{types.Container{Names: []string{"/foobar_baz"}}, true, "foobar_baz", ctx.Names},
		{types.Container{Image: "ubuntu"}, true, "ubuntu", ctx.Image},
		{types.Container{Image: "verylongimagename"}, true, "verylongimagename", ctx.Image},
		{types.Container{Image: "verylongimagename"}, false, "verylongimagename", ctx.Image},
		{types.Container{
			Image:   "a5a665ff33eced1e0803148700880edab4",
			ImageID: "a5a665ff33eced1e0803148700880edab4269067ed77e27737a708d0d293fbf5",
		},
			true,
			"a5a665ff33ec",
			ctx.Image,
		},
		{types.Container{
			Image:   "a5a665ff33eced1e0803148700880edab4",
			ImageID: "a5a665ff33eced1e0803148700880edab4269067ed77e27737a708d0d293fbf5",
		},
			false,
			"a5a665ff33eced1e0803148700880edab4",
			ctx.Image,
		},
		{types.Container{Image: ""}, true, "<no image>", ctx.Image},
		{types.Container{Command: "sh -c 'ls -la'"}, true, `"sh -c 'ls -la'"`, ctx.Command},
		{types.Container{Created: unix}, true, time.Unix(unix, 0).String(), ctx.CreatedAt},
		{types.Container{Ports: []types.Port{{PrivatePort: 8080, PublicPort: 8080, Type: "tcp"}}}, true, "8080/tcp", ctx.Ports},
		{types.Container{Status: "RUNNING"}, true, "RUNNING", ctx.Status},
		{types.Container{SizeRw: 10}, true, "10B", ctx.Size},
		{types.Container{SizeRw: 10, SizeRootFs: 20}, true, "10B (virtual 20B)", ctx.Size},
		{types.Container{}, true, "", ctx.Labels},
		{types.Container{Labels: map[string]string{"cpu": "6", "storage": "ssd"}}, true, "cpu=6,storage=ssd", ctx.Labels},
		{types.Container{Created: unix}, true, "About a minute ago", ctx.RunningFor},
		{types.Container{
			Mounts: []types.MountPoint{
				{
					Name:   "this-is-a-long-volume-name-and-will-be-truncated-if-trunc-is-set",
					Driver: "local",
					Source: "/a/path",
				},
			},
		}, true, "this-is-a-lo...", ctx.Mounts},
		{types.Container{
			Mounts: []types.MountPoint{
				{
					Driver: "local",
					Source: "/a/path",
				},
			},
		}, false, "/a/path", ctx.Mounts},
		{types.Container{
			Mounts: []types.MountPoint{
				{
					Name:   "733908409c91817de8e92b0096373245f329f19a88e2c849f02460e9b3d1c203",
					Driver: "local",
					Source: "/a/path",
				},
			},
		}, false, "733908409c91817de8e92b0096373245f329f19a88e2c849f02460e9b3d1c203", ctx.Mounts},
	}

	for _, c := range cases {
		ctx = containerContext{c: c.container, trunc: c.trunc}
		v := c.call()
		if strings.Contains(v, ",") {
			compareMultipleValues(t, v, c.expValue)
		} else if v != c.expValue {
			t.Fatalf("Expected %s, was %s\n", c.expValue, v)
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

	c2 := types.Container{}
	ctx = containerContext{c: c2, trunc: true}

	label := ctx.Label("anything.really")
	if label != "" {
		t.Fatalf("Expected an empty string, was %s", label)
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
containerID1        ubuntu              ""                  24 hours ago                                                foobar_baz          0B
containerID2        ubuntu              ""                  24 hours ago                                                foobar_bar          0B
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
size: 0B

container_id: containerID2
image: ubuntu
command: ""
created_at: %s
status:
names: foobar_bar
labels:
ports:
size: 0B

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
		// Special headers for customerized table format
		{
			Context{Format: NewContainerFormat(`table {{truncate .ID 5}}\t{{json .Image}} {{.RunningFor}}/{{title .Status}}/{{pad .Ports 2 2}}.{{upper .Names}} {{lower .Status}}`, false, true)},
			`CONTAINER ID        IMAGE CREATED/STATUS/  PORTS  .NAMES STATUS
conta               "ubuntu" 24 hours ago//.FOOBAR_BAZ 
conta               "ubuntu" 24 hours ago//.FOOBAR_BAR 
`,
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
			assert.EqualError(t, err, testcase.expected)
		} else {
			assert.Equal(t, testcase.expected, out.String())
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

func TestContainerContextWriteJSON(t *testing.T) {
	unix := time.Now().Add(-65 * time.Second).Unix()
	containers := []types.Container{
		{ID: "containerID1", Names: []string{"/foobar_baz"}, Image: "ubuntu", Created: unix},
		{ID: "containerID2", Names: []string{"/foobar_bar"}, Image: "ubuntu", Created: unix},
	}
	expectedCreated := time.Unix(unix, 0).String()
	expectedJSONs := []map[string]interface{}{
		{"Command": "\"\"", "CreatedAt": expectedCreated, "ID": "containerID1", "Image": "ubuntu", "Labels": "", "LocalVolumes": "0", "Mounts": "", "Names": "foobar_baz", "Networks": "", "Ports": "", "RunningFor": "About a minute ago", "Size": "0B", "Status": ""},
		{"Command": "\"\"", "CreatedAt": expectedCreated, "ID": "containerID2", "Image": "ubuntu", "Labels": "", "LocalVolumes": "0", "Mounts": "", "Names": "foobar_bar", "Networks": "", "Ports": "", "RunningFor": "About a minute ago", "Size": "0B", "Status": ""},
	}
	out := bytes.NewBufferString("")
	err := ContainerWrite(Context{Format: "{{json .}}", Output: out}, containers)
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		t.Logf("Output: line %d: %s", i, line)
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, expectedJSONs[i], m)
	}
}

func TestContainerContextWriteJSONField(t *testing.T) {
	containers := []types.Container{
		{ID: "containerID1", Names: []string{"/foobar_baz"}, Image: "ubuntu"},
		{ID: "containerID2", Names: []string{"/foobar_bar"}, Image: "ubuntu"},
	}
	out := bytes.NewBufferString("")
	err := ContainerWrite(Context{Format: "{{json .ID}}", Output: out}, containers)
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		t.Logf("Output: line %d: %s", i, line)
		var s string
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, containers[i].ID, s)
	}
}

func TestContainerBackCompat(t *testing.T) {
	containers := []types.Container{{ID: "brewhaha"}}
	cases := []string{
		"ID",
		"Names",
		"Image",
		"Command",
		"CreatedAt",
		"RunningFor",
		"Ports",
		"Status",
		"Size",
		"Labels",
		"Mounts",
	}
	buf := bytes.NewBuffer(nil)
	for _, c := range cases {
		ctx := Context{Format: Format(fmt.Sprintf("{{ .%s }}", c)), Output: buf}
		if err := ContainerWrite(ctx, containers); err != nil {
			t.Logf("could not render template for field '%s': %v", c, err)
			t.Fail()
		}
		buf.Reset()
	}
}
