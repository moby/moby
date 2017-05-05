package formatter

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/swarm"
	"github.com/stretchr/testify/assert"
)

func TestServiceContextWrite(t *testing.T) {
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
		// Table format
		{
			Context{Format: NewServiceListFormat("table", false)},
			`ID                  NAME                MODE                REPLICAS            IMAGE               PORTS
id_baz              baz                 global              2/4                                     *:80->8080/tcp
id_bar              bar                 replicated          2/4                                     *:80->8080/tcp
`,
		},
		{
			Context{Format: NewServiceListFormat("table", true)},
			`id_baz
id_bar
`,
		},
		{
			Context{Format: NewServiceListFormat("table {{.Name}}", false)},
			`NAME
baz
bar
`,
		},
		{
			Context{Format: NewServiceListFormat("table {{.Name}}", true)},
			`NAME
baz
bar
`,
		},
		// Raw Format
		{
			Context{Format: NewServiceListFormat("raw", false)},
			`id: id_baz
name: baz
mode: global
replicas: 2/4
image: 
ports: *:80->8080/tcp

id: id_bar
name: bar
mode: replicated
replicas: 2/4
image: 
ports: *:80->8080/tcp

`,
		},
		{
			Context{Format: NewServiceListFormat("raw", true)},
			`id: id_baz
id: id_bar
`,
		},
		// Custom Format
		{
			Context{Format: NewServiceListFormat("{{.Name}}", false)},
			`baz
bar
`,
		},
	}

	for _, testcase := range cases {
		services := []swarm.Service{
			{
				ID: "id_baz",
				Spec: swarm.ServiceSpec{
					Annotations: swarm.Annotations{Name: "baz"},
					EndpointSpec: &swarm.EndpointSpec{
						Ports: []swarm.PortConfig{
							{
								PublishMode:   "ingress",
								PublishedPort: 80,
								TargetPort:    8080,
								Protocol:      "tcp",
							},
						},
					},
				},
			},
			{
				ID: "id_bar",
				Spec: swarm.ServiceSpec{
					Annotations: swarm.Annotations{Name: "bar"},
					EndpointSpec: &swarm.EndpointSpec{
						Ports: []swarm.PortConfig{
							{
								PublishMode:   "ingress",
								PublishedPort: 80,
								TargetPort:    8080,
								Protocol:      "tcp",
							},
						},
					},
				},
			},
		}
		info := map[string]ServiceListInfo{
			"id_baz": {
				Mode:     "global",
				Replicas: "2/4",
			},
			"id_bar": {
				Mode:     "replicated",
				Replicas: "2/4",
			},
		}
		out := bytes.NewBufferString("")
		testcase.context.Output = out
		err := ServiceListWrite(testcase.context, services, info)
		if err != nil {
			assert.EqualError(t, err, testcase.expected)
		} else {
			assert.Equal(t, testcase.expected, out.String())
		}
	}
}

func TestServiceContextWriteJSON(t *testing.T) {
	services := []swarm.Service{
		{
			ID: "id_baz",
			Spec: swarm.ServiceSpec{
				Annotations: swarm.Annotations{Name: "baz"},
				EndpointSpec: &swarm.EndpointSpec{
					Ports: []swarm.PortConfig{
						{
							PublishMode:   "ingress",
							PublishedPort: 80,
							TargetPort:    8080,
							Protocol:      "tcp",
						},
					},
				},
			},
		},
		{
			ID: "id_bar",
			Spec: swarm.ServiceSpec{
				Annotations: swarm.Annotations{Name: "bar"},
				EndpointSpec: &swarm.EndpointSpec{
					Ports: []swarm.PortConfig{
						{
							PublishMode:   "ingress",
							PublishedPort: 80,
							TargetPort:    8080,
							Protocol:      "tcp",
						},
					},
				},
			},
		},
	}
	info := map[string]ServiceListInfo{
		"id_baz": {
			Mode:     "global",
			Replicas: "2/4",
		},
		"id_bar": {
			Mode:     "replicated",
			Replicas: "2/4",
		},
	}
	expectedJSONs := []map[string]interface{}{
		{"ID": "id_baz", "Name": "baz", "Mode": "global", "Replicas": "2/4", "Image": "", "Ports": "*:80->8080/tcp"},
		{"ID": "id_bar", "Name": "bar", "Mode": "replicated", "Replicas": "2/4", "Image": "", "Ports": "*:80->8080/tcp"},
	}

	out := bytes.NewBufferString("")
	err := ServiceListWrite(Context{Format: "{{json .}}", Output: out}, services, info)
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
func TestServiceContextWriteJSONField(t *testing.T) {
	services := []swarm.Service{
		{ID: "id_baz", Spec: swarm.ServiceSpec{Annotations: swarm.Annotations{Name: "baz"}}},
		{ID: "id_bar", Spec: swarm.ServiceSpec{Annotations: swarm.Annotations{Name: "bar"}}},
	}
	info := map[string]ServiceListInfo{
		"id_baz": {
			Mode:     "global",
			Replicas: "2/4",
		},
		"id_bar": {
			Mode:     "replicated",
			Replicas: "2/4",
		},
	}
	out := bytes.NewBufferString("")
	err := ServiceListWrite(Context{Format: "{{json .Name}}", Output: out}, services, info)
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		t.Logf("Output: line %d: %s", i, line)
		var s string
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, services[i].Spec.Name, s)
	}
}
