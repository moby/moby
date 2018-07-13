package convert // import "github.com/docker/docker/daemon/cluster/convert"

import (
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/swarm/runtime"
	swarmapi "github.com/docker/swarmkit/api"
	google_protobuf3 "github.com/gogo/protobuf/types"
	"gotest.tools/assert"
)

func TestServiceConvertFromGRPCRuntimeContainer(t *testing.T) {
	gs := swarmapi.Service{
		Meta: swarmapi.Meta{
			Version: swarmapi.Version{
				Index: 1,
			},
			CreatedAt: nil,
			UpdatedAt: nil,
		},
		SpecVersion: &swarmapi.Version{
			Index: 1,
		},
		Spec: swarmapi.ServiceSpec{
			Task: swarmapi.TaskSpec{
				Runtime: &swarmapi.TaskSpec_Container{
					Container: &swarmapi.ContainerSpec{
						Image: "alpine:latest",
					},
				},
			},
		},
	}

	svc, err := ServiceFromGRPC(gs)
	if err != nil {
		t.Fatal(err)
	}

	if svc.Spec.TaskTemplate.Runtime != swarmtypes.RuntimeContainer {
		t.Fatalf("expected type %s; received %T", swarmtypes.RuntimeContainer, svc.Spec.TaskTemplate.Runtime)
	}
}

func TestServiceConvertFromGRPCGenericRuntimePlugin(t *testing.T) {
	kind := string(swarmtypes.RuntimePlugin)
	url := swarmtypes.RuntimeURLPlugin
	gs := swarmapi.Service{
		Meta: swarmapi.Meta{
			Version: swarmapi.Version{
				Index: 1,
			},
			CreatedAt: nil,
			UpdatedAt: nil,
		},
		SpecVersion: &swarmapi.Version{
			Index: 1,
		},
		Spec: swarmapi.ServiceSpec{
			Task: swarmapi.TaskSpec{
				Runtime: &swarmapi.TaskSpec_Generic{
					Generic: &swarmapi.GenericRuntimeSpec{
						Kind: kind,
						Payload: &google_protobuf3.Any{
							TypeUrl: string(url),
						},
					},
				},
			},
		},
	}

	svc, err := ServiceFromGRPC(gs)
	if err != nil {
		t.Fatal(err)
	}

	if svc.Spec.TaskTemplate.Runtime != swarmtypes.RuntimePlugin {
		t.Fatalf("expected type %s; received %T", swarmtypes.RuntimePlugin, svc.Spec.TaskTemplate.Runtime)
	}
}

func TestServiceConvertToGRPCGenericRuntimePlugin(t *testing.T) {
	s := swarmtypes.ServiceSpec{
		TaskTemplate: swarmtypes.TaskSpec{
			Runtime:    swarmtypes.RuntimePlugin,
			PluginSpec: &runtime.PluginSpec{},
		},
		Mode: swarmtypes.ServiceMode{
			Global: &swarmtypes.GlobalService{},
		},
	}

	svc, err := ServiceSpecToGRPC(s)
	if err != nil {
		t.Fatal(err)
	}

	v, ok := svc.Task.Runtime.(*swarmapi.TaskSpec_Generic)
	if !ok {
		t.Fatal("expected type swarmapi.TaskSpec_Generic")
	}

	if v.Generic.Payload.TypeUrl != string(swarmtypes.RuntimeURLPlugin) {
		t.Fatalf("expected url %s; received %s", swarmtypes.RuntimeURLPlugin, v.Generic.Payload.TypeUrl)
	}
}

func TestServiceConvertToGRPCContainerRuntime(t *testing.T) {
	image := "alpine:latest"
	s := swarmtypes.ServiceSpec{
		TaskTemplate: swarmtypes.TaskSpec{
			ContainerSpec: &swarmtypes.ContainerSpec{
				Image: image,
			},
		},
		Mode: swarmtypes.ServiceMode{
			Global: &swarmtypes.GlobalService{},
		},
	}

	svc, err := ServiceSpecToGRPC(s)
	if err != nil {
		t.Fatal(err)
	}

	v, ok := svc.Task.Runtime.(*swarmapi.TaskSpec_Container)
	if !ok {
		t.Fatal("expected type swarmapi.TaskSpec_Container")
	}

	if v.Container.Image != image {
		t.Fatalf("expected image %s; received %s", image, v.Container.Image)
	}
}

func TestServiceConvertToGRPCGenericRuntimeCustom(t *testing.T) {
	s := swarmtypes.ServiceSpec{
		TaskTemplate: swarmtypes.TaskSpec{
			Runtime: "customruntime",
		},
		Mode: swarmtypes.ServiceMode{
			Global: &swarmtypes.GlobalService{},
		},
	}

	if _, err := ServiceSpecToGRPC(s); err != ErrUnsupportedRuntime {
		t.Fatal(err)
	}
}

func TestServiceConvertToGRPCIsolation(t *testing.T) {
	cases := []struct {
		name string
		from containertypes.Isolation
		to   swarmapi.ContainerSpec_Isolation
	}{
		{name: "empty", from: containertypes.IsolationEmpty, to: swarmapi.ContainerIsolationDefault},
		{name: "default", from: containertypes.IsolationDefault, to: swarmapi.ContainerIsolationDefault},
		{name: "process", from: containertypes.IsolationProcess, to: swarmapi.ContainerIsolationProcess},
		{name: "hyperv", from: containertypes.IsolationHyperV, to: swarmapi.ContainerIsolationHyperV},
		{name: "proCess", from: containertypes.Isolation("proCess"), to: swarmapi.ContainerIsolationProcess},
		{name: "hypErv", from: containertypes.Isolation("hypErv"), to: swarmapi.ContainerIsolationHyperV},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := swarmtypes.ServiceSpec{
				TaskTemplate: swarmtypes.TaskSpec{
					ContainerSpec: &swarmtypes.ContainerSpec{
						Image:     "alpine:latest",
						Isolation: c.from,
					},
				},
				Mode: swarmtypes.ServiceMode{
					Global: &swarmtypes.GlobalService{},
				},
			}
			res, err := ServiceSpecToGRPC(s)
			assert.NilError(t, err)
			v, ok := res.Task.Runtime.(*swarmapi.TaskSpec_Container)
			if !ok {
				t.Fatal("expected type swarmapi.TaskSpec_Container")
			}
			assert.Equal(t, c.to, v.Container.Isolation)
		})
	}
}

func TestServiceConvertFromGRPCIsolation(t *testing.T) {
	cases := []struct {
		name string
		from swarmapi.ContainerSpec_Isolation
		to   containertypes.Isolation
	}{
		{name: "default", to: containertypes.IsolationDefault, from: swarmapi.ContainerIsolationDefault},
		{name: "process", to: containertypes.IsolationProcess, from: swarmapi.ContainerIsolationProcess},
		{name: "hyperv", to: containertypes.IsolationHyperV, from: swarmapi.ContainerIsolationHyperV},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gs := swarmapi.Service{
				Meta: swarmapi.Meta{
					Version: swarmapi.Version{
						Index: 1,
					},
					CreatedAt: nil,
					UpdatedAt: nil,
				},
				SpecVersion: &swarmapi.Version{
					Index: 1,
				},
				Spec: swarmapi.ServiceSpec{
					Task: swarmapi.TaskSpec{
						Runtime: &swarmapi.TaskSpec_Container{
							Container: &swarmapi.ContainerSpec{
								Image:     "alpine:latest",
								Isolation: c.from,
							},
						},
					},
				},
			}

			svc, err := ServiceFromGRPC(gs)
			if err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, c.to, svc.Spec.TaskTemplate.ContainerSpec.Isolation)
		})
	}
}

func TestServiceConvertToGRPCNetworkAtachmentRuntime(t *testing.T) {
	someid := "asfjkl"
	s := swarmtypes.ServiceSpec{
		TaskTemplate: swarmtypes.TaskSpec{
			Runtime: swarmtypes.RuntimeNetworkAttachment,
			NetworkAttachmentSpec: &swarmtypes.NetworkAttachmentSpec{
				ContainerID: someid,
			},
		},
	}

	// discard the service, which will be empty
	_, err := ServiceSpecToGRPC(s)
	if err == nil {
		t.Fatalf("expected error %v but got no error", ErrUnsupportedRuntime)
	}
	if err != ErrUnsupportedRuntime {
		t.Fatalf("expected error %v but got error %v", ErrUnsupportedRuntime, err)
	}
}

func TestServiceConvertToGRPCMismatchedRuntime(t *testing.T) {
	// NOTE(dperny): an earlier version of this test was for code that also
	// converted network attachment tasks to GRPC. that conversion code was
	// removed, so if this loop body seems a bit complicated, that's why.
	for i, rt := range []swarmtypes.RuntimeType{
		swarmtypes.RuntimeContainer,
		swarmtypes.RuntimePlugin,
	} {
		for j, spec := range []swarmtypes.TaskSpec{
			{ContainerSpec: &swarmtypes.ContainerSpec{}},
			{PluginSpec: &runtime.PluginSpec{}},
		} {
			// skip the cases, where the indices match, which would not error
			if i == j {
				continue
			}
			// set the task spec, then change the runtime
			s := swarmtypes.ServiceSpec{
				TaskTemplate: spec,
			}
			s.TaskTemplate.Runtime = rt

			if _, err := ServiceSpecToGRPC(s); err != ErrMismatchedRuntime {
				t.Fatalf("expected %v got %v", ErrMismatchedRuntime, err)
			}
		}
	}
}

func TestTaskConvertFromGRPCNetworkAttachment(t *testing.T) {
	containerID := "asdfjkl"
	s := swarmapi.TaskSpec{
		Runtime: &swarmapi.TaskSpec_Attachment{
			Attachment: &swarmapi.NetworkAttachmentSpec{
				ContainerID: containerID,
			},
		},
	}
	ts, err := taskSpecFromGRPC(s)
	if err != nil {
		t.Fatal(err)
	}
	if ts.NetworkAttachmentSpec == nil {
		t.Fatal("expected task spec to have network attachment spec")
	}
	if ts.NetworkAttachmentSpec.ContainerID != containerID {
		t.Fatalf("expected network attachment spec container id to be %q, was %q", containerID, ts.NetworkAttachmentSpec.ContainerID)
	}
	if ts.Runtime != swarmtypes.RuntimeNetworkAttachment {
		t.Fatalf("expected Runtime to be %v", swarmtypes.RuntimeNetworkAttachment)
	}
}
