package ps

import (
	"bytes"
	"testing"

	"github.com/docker/docker/api/types"
)

func TestFormat(t *testing.T) {
	contexts := []struct {
		context  Context
		expected string
	}{
		// Errors
		{
			Context{
				Format: "{{InvalidFunction}}",
			},
			`Template parsing error: template: :1: function "InvalidFunction" not defined
`,
		},
		{
			Context{
				Format: "{{nil}}",
			},
			`Template parsing error: template: :1:2: executing "" at <nil>: nil is not a command
`,
		},
		// Table Format
		{
			Context{
				Format: "table",
			},
			`CONTAINER ID        IMAGE               COMMAND             CREATED             STATUS              PORTS               NAMES
containerID1        ubuntu              ""                  45 years ago                                                foobar_baz
containerID2        ubuntu              ""                  45 years ago                                                foobar_bar
`,
		},
		{
			Context{
				Format: "table {{.Image}}",
			},
			"IMAGE\nubuntu\nubuntu\n",
		},
		{
			Context{
				Format: "table {{.Image}}",
				Size:   true,
			},
			"IMAGE               SIZE\nubuntu              0 B\nubuntu              0 B\n",
		},
		{
			Context{
				Format: "table {{.Image}}",
				Quiet:  true,
			},
			"IMAGE\nubuntu\nubuntu\n",
		},
		{
			Context{
				Format: "table",
				Quiet:  true,
			},
			"containerID1\ncontainerID2\n",
		},
		// Raw Format
		{
			Context{
				Format: "raw",
			},
			`container_id: containerID1
image: ubuntu
command: ""
created_at: 1970-01-01 00:00:00 +0000 UTC
status: 
names: foobar_baz
labels: 
ports: 

container_id: containerID2
image: ubuntu
command: ""
created_at: 1970-01-01 00:00:00 +0000 UTC
status: 
names: foobar_bar
labels: 
ports: 

`,
		},
		{
			Context{
				Format: "raw",
				Size:   true,
			},
			`container_id: containerID1
image: ubuntu
command: ""
created_at: 1970-01-01 00:00:00 +0000 UTC
status: 
names: foobar_baz
labels: 
ports: 
size: 0 B

container_id: containerID2
image: ubuntu
command: ""
created_at: 1970-01-01 00:00:00 +0000 UTC
status: 
names: foobar_bar
labels: 
ports: 
size: 0 B

`,
		},
		{
			Context{
				Format: "raw",
				Quiet:  true,
			},
			"container_id: containerID1\ncontainer_id: containerID2\n",
		},
		// Custom Format
		{
			Context{
				Format: "{{.Image}}",
			},
			"ubuntu\nubuntu\n",
		},
		{
			Context{
				Format: "{{.Image}}",
				Size:   true,
			},
			"ubuntu\nubuntu\n",
		},
	}

	for _, context := range contexts {
		containers := []types.Container{
			{ID: "containerID1", Names: []string{"/foobar_baz"}, Image: "ubuntu"},
			{ID: "containerID2", Names: []string{"/foobar_bar"}, Image: "ubuntu"},
		}
		out := bytes.NewBufferString("")
		context.context.Output = out
		Format(context.context, containers)
		actual := out.String()
		if actual != context.expected {
			t.Fatalf("Expected \n%s, got \n%s", context.expected, actual)
		}
		// Clean buffer
		out.Reset()
	}
}

func TestCustomFormatNoContainers(t *testing.T) {
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
				Format: "{{.Image}}",
				Output: out,
				Size:   true,
			},
			"",
		},
		{
			Context{
				Format: "table {{.Image}}",
				Output: out,
				Size:   true,
			},
			"IMAGE               SIZE\n",
		},
	}

	for _, context := range contexts {
		customFormat(context.context, containers)
		actual := out.String()
		if actual != context.expected {
			t.Fatalf("Expected \n%s, got \n%s", context.expected, actual)
		}
		// Clean buffer
		out.Reset()
	}
}
