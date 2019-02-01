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

func TestServiceConvertToGRPCCredentialSpec(t *testing.T) {
	cases := []struct {
		name        string
		from        swarmtypes.CredentialSpec
		to          swarmapi.Privileges_CredentialSpec
		expectedErr string
	}{
		{
			name:        "empty credential spec",
			from:        swarmtypes.CredentialSpec{},
			to:          swarmapi.Privileges_CredentialSpec{},
			expectedErr: `invalid CredentialSpec: must either provide "file", "registry", or "config" for credential spec`,
		},
		{
			name: "config and file credential spec",
			from: swarmtypes.CredentialSpec{
				Config: "0bt9dmxjvjiqermk6xrop3ekq",
				File:   "spec.json",
			},
			to:          swarmapi.Privileges_CredentialSpec{},
			expectedErr: `invalid CredentialSpec: cannot specify both "config" and "file" credential specs`,
		},
		{
			name: "config and registry credential spec",
			from: swarmtypes.CredentialSpec{
				Config:   "0bt9dmxjvjiqermk6xrop3ekq",
				Registry: "testing",
			},
			to:          swarmapi.Privileges_CredentialSpec{},
			expectedErr: `invalid CredentialSpec: cannot specify both "config" and "registry" credential specs`,
		},
		{
			name: "file and registry credential spec",
			from: swarmtypes.CredentialSpec{
				File:     "spec.json",
				Registry: "testing",
			},
			to:          swarmapi.Privileges_CredentialSpec{},
			expectedErr: `invalid CredentialSpec: cannot specify both "file" and "registry" credential specs`,
		},
		{
			name: "config and file and registry credential spec",
			from: swarmtypes.CredentialSpec{
				Config:   "0bt9dmxjvjiqermk6xrop3ekq",
				File:     "spec.json",
				Registry: "testing",
			},
			to:          swarmapi.Privileges_CredentialSpec{},
			expectedErr: `invalid CredentialSpec: cannot specify both "config", "file", and "registry" credential specs`,
		},
		{
			name: "config credential spec",
			from: swarmtypes.CredentialSpec{Config: "0bt9dmxjvjiqermk6xrop3ekq"},
			to: swarmapi.Privileges_CredentialSpec{
				Source: &swarmapi.Privileges_CredentialSpec_Config{Config: "0bt9dmxjvjiqermk6xrop3ekq"},
			},
		},
		{
			name: "file credential spec",
			from: swarmtypes.CredentialSpec{File: "foo.json"},
			to: swarmapi.Privileges_CredentialSpec{
				Source: &swarmapi.Privileges_CredentialSpec_File{File: "foo.json"},
			},
		},
		{
			name: "registry credential spec",
			from: swarmtypes.CredentialSpec{Registry: "testing"},
			to: swarmapi.Privileges_CredentialSpec{
				Source: &swarmapi.Privileges_CredentialSpec_Registry{Registry: "testing"},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			s := swarmtypes.ServiceSpec{
				TaskTemplate: swarmtypes.TaskSpec{
					ContainerSpec: &swarmtypes.ContainerSpec{
						Privileges: &swarmtypes.Privileges{
							CredentialSpec: &c.from,
						},
					},
				},
			}

			res, err := ServiceSpecToGRPC(s)
			if c.expectedErr != "" {
				assert.Error(t, err, c.expectedErr)
				return
			}

			assert.NilError(t, err)
			v, ok := res.Task.Runtime.(*swarmapi.TaskSpec_Container)
			if !ok {
				t.Fatal("expected type swarmapi.TaskSpec_Container")
			}
			assert.DeepEqual(t, c.to, *v.Container.Privileges.CredentialSpec)
		})
	}
}

func TestServiceConvertFromGRPCCredentialSpec(t *testing.T) {
	cases := []struct {
		name string
		from swarmapi.Privileges_CredentialSpec
		to   *swarmtypes.CredentialSpec
	}{
		{
			name: "empty credential spec",
			from: swarmapi.Privileges_CredentialSpec{},
			to:   &swarmtypes.CredentialSpec{},
		},
		{
			name: "config credential spec",
			from: swarmapi.Privileges_CredentialSpec{
				Source: &swarmapi.Privileges_CredentialSpec_Config{Config: "0bt9dmxjvjiqermk6xrop3ekq"},
			},
			to: &swarmtypes.CredentialSpec{Config: "0bt9dmxjvjiqermk6xrop3ekq"},
		},
		{
			name: "file credential spec",
			from: swarmapi.Privileges_CredentialSpec{
				Source: &swarmapi.Privileges_CredentialSpec_File{File: "foo.json"},
			},
			to: &swarmtypes.CredentialSpec{File: "foo.json"},
		},
		{
			name: "registry credential spec",
			from: swarmapi.Privileges_CredentialSpec{
				Source: &swarmapi.Privileges_CredentialSpec_Registry{Registry: "testing"},
			},
			to: &swarmtypes.CredentialSpec{Registry: "testing"},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			gs := swarmapi.Service{
				Spec: swarmapi.ServiceSpec{
					Task: swarmapi.TaskSpec{
						Runtime: &swarmapi.TaskSpec_Container{
							Container: &swarmapi.ContainerSpec{
								Privileges: &swarmapi.Privileges{
									CredentialSpec: &tc.from,
								},
							},
						},
					},
				},
			}

			svc, err := ServiceFromGRPC(gs)
			assert.NilError(t, err)
			assert.DeepEqual(t, svc.Spec.TaskTemplate.ContainerSpec.Privileges.CredentialSpec, tc.to)
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
