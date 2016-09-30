package service

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/command/formatter"
)

func TestPrettyPrintWithNoUpdateConfig(t *testing.T) {
	b := new(bytes.Buffer)

	endpointSpec := &swarm.EndpointSpec{
		Mode: "vip",
		Ports: []swarm.PortConfig{
			{
				Protocol:   swarm.PortConfigProtocolTCP,
				TargetPort: 5000,
			},
		},
	}

	two := uint64(2)

	s := swarm.Service{
		ID: "de179gar9d0o7ltdybungplod",
		Meta: swarm.Meta{
			Version:   swarm.Version{Index: 315},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Spec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{
				Name:   "my_service",
				Labels: map[string]string{"com.label": "foo"},
			},
			TaskTemplate: swarm.TaskSpec{
				ContainerSpec: swarm.ContainerSpec{
					Image: "foo/bar@sha256:this_is_a_test",
				},
			},
			Mode: swarm.ServiceMode{
				Replicated: &swarm.ReplicatedService{
					Replicas: &two,
				},
			},
			UpdateConfig: nil,
			Networks: []swarm.NetworkAttachmentConfig{
				{
					Target:  "5vpyomhb6ievnk0i0o60gcnei",
					Aliases: []string{"web"},
				},
			},
			EndpointSpec: endpointSpec,
		},
		Endpoint: swarm.Endpoint{
			Spec: *endpointSpec,
			Ports: []swarm.PortConfig{
				{
					Protocol:      swarm.PortConfigProtocolTCP,
					TargetPort:    5000,
					PublishedPort: 30000,
				},
			},
			VirtualIPs: []swarm.EndpointVirtualIP{
				{
					NetworkID: "6o4107cj2jx9tihgb0jyts6pj",
					Addr:      "10.255.0.4/16",
				},
			},
		},
		UpdateStatus: swarm.UpdateStatus{
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
		},
	}

	ctx := formatter.Context{
		Output: b,
		Format: formatter.NewServiceFormat("pretty"),
	}

	err := formatter.ServiceInspectWrite(ctx, []string{"de179gar9d0o7ltdybungplod"}, func(ref string) (interface{}, []byte, error) {
		return s, nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(b.String(), "UpdateStatus") {
		t.Fatal("Pretty print failed before parsing UpdateStatus")
	}
}
