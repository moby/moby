package container // import "github.com/docker/docker/daemon/cluster/executor/container"

import (
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	swarmapi "github.com/moby/swarmkit/v2/api"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestIsolationConversion(t *testing.T) {
	cases := []struct {
		name string
		from swarmapi.ContainerSpec_Isolation
		to   container.Isolation
	}{
		{name: "default", from: swarmapi.ContainerIsolationDefault, to: container.IsolationDefault},
		{name: "process", from: swarmapi.ContainerIsolationProcess, to: container.IsolationProcess},
		{name: "hyperv", from: swarmapi.ContainerIsolationHyperV, to: container.IsolationHyperV},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			task := swarmapi.Task{
				Spec: swarmapi.TaskSpec{
					Runtime: &swarmapi.TaskSpec_Container{
						Container: &swarmapi.ContainerSpec{
							Image:     "alpine:latest",
							Isolation: c.from,
						},
					},
				},
			}
			config := containerConfig{task: &task}
			// NOTE(dperny): you shouldn't ever pass nil outside of testing,
			// because if there are CSI volumes, the code will panic. However,
			// in testing. this is acceptable.
			assert.Equal(t, c.to, config.hostConfig(nil).Isolation)
		})
	}
}

func TestContainerLabels(t *testing.T) {
	c := &containerConfig{
		task: &swarmapi.Task{
			ID: "real-task.id",
			Spec: swarmapi.TaskSpec{
				Runtime: &swarmapi.TaskSpec_Container{
					Container: &swarmapi.ContainerSpec{
						Labels: map[string]string{
							"com.docker.swarm.task":         "user-specified-task",
							"com.docker.swarm.task.id":      "user-specified-task.id",
							"com.docker.swarm.task.name":    "user-specified-task.name",
							"com.docker.swarm.node.id":      "user-specified-node.id",
							"com.docker.swarm.service.id":   "user-specified-service.id",
							"com.docker.swarm.service.name": "user-specified-service.name",
							"this-is-a-user-label":          "this is a user label's value",
						},
					},
				},
			},
			ServiceID: "real-service.id",
			Slot:      123,
			NodeID:    "real-node.id",
			Annotations: swarmapi.Annotations{
				Name: "real-service.name.123.real-task.id",
			},
			ServiceAnnotations: swarmapi.Annotations{
				Name: "real-service.name",
			},
		},
	}

	expected := map[string]string{
		"com.docker.swarm.task":         "",
		"com.docker.swarm.task.id":      "real-task.id",
		"com.docker.swarm.task.name":    "real-service.name.123.real-task.id",
		"com.docker.swarm.node.id":      "real-node.id",
		"com.docker.swarm.service.id":   "real-service.id",
		"com.docker.swarm.service.name": "real-service.name",
		"this-is-a-user-label":          "this is a user label's value",
	}

	labels := c.labels()
	assert.DeepEqual(t, expected, labels)
}

func TestCredentialSpecConversion(t *testing.T) {
	cases := []struct {
		name string
		from swarmapi.Privileges_CredentialSpec
		to   []string
	}{
		{
			name: "none",
			from: swarmapi.Privileges_CredentialSpec{},
			to:   nil,
		},
		{
			name: "config",
			from: swarmapi.Privileges_CredentialSpec{
				Source: &swarmapi.Privileges_CredentialSpec_Config{Config: "0bt9dmxjvjiqermk6xrop3ekq"},
			},
			to: []string{"credentialspec=config://0bt9dmxjvjiqermk6xrop3ekq"},
		},
		{
			name: "file",
			from: swarmapi.Privileges_CredentialSpec{
				Source: &swarmapi.Privileges_CredentialSpec_File{File: "foo.json"},
			},
			to: []string{"credentialspec=file://foo.json"},
		},
		{
			name: "registry",
			from: swarmapi.Privileges_CredentialSpec{
				Source: &swarmapi.Privileges_CredentialSpec_Registry{Registry: "testing"},
			},
			to: []string{"credentialspec=registry://testing"},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			task := swarmapi.Task{
				Spec: swarmapi.TaskSpec{
					Runtime: &swarmapi.TaskSpec_Container{
						Container: &swarmapi.ContainerSpec{
							Privileges: &swarmapi.Privileges{
								CredentialSpec: &c.from,
							},
						},
					},
				},
			}
			config := containerConfig{task: &task}
			// NOTE(dperny): you shouldn't ever pass nil outside of testing,
			// because if there are CSI volumes, the code will panic. However,
			// in testing. this is acceptable.
			assert.DeepEqual(t, c.to, config.hostConfig(nil).SecurityOpt)
		})
	}
}

func TestTmpfsConversion(t *testing.T) {
	cases := []struct {
		name string
		from []swarmapi.Mount
		to   []mount.Mount
	}{
		{
			name: "tmpfs-exec",
			from: []swarmapi.Mount{
				{
					Source: "/foo",
					Target: "/bar",
					Type:   swarmapi.MountTypeTmpfs,
					TmpfsOptions: &swarmapi.Mount_TmpfsOptions{
						Options: "[[\"exec\"]]",
					},
				},
			},
			to: []mount.Mount{
				{
					Source: "/foo",
					Target: "/bar",
					Type:   mount.TypeTmpfs,
					TmpfsOptions: &mount.TmpfsOptions{
						Options: [][]string{[]string{"exec"}},
					},
				},
			},
		},
		{
			name: "tmpfs-noexec",
			from: []swarmapi.Mount{
				{
					Source: "/foo",
					Target: "/bar",
					Type:   swarmapi.MountTypeTmpfs,
					TmpfsOptions: &swarmapi.Mount_TmpfsOptions{
						Options: "[[\"noexec\"]]",
					},
				},
			},
			to: []mount.Mount{
				{
					Source: "/foo",
					Target: "/bar",
					Type:   mount.TypeTmpfs,
					TmpfsOptions: &mount.TmpfsOptions{
						Options: [][]string{[]string{"noexec"}},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			task := swarmapi.Task{
				Spec: swarmapi.TaskSpec{
					Runtime: &swarmapi.TaskSpec_Container{
						Container: &swarmapi.ContainerSpec{
							Image:  "alpine:latest",
							Mounts: c.from,
						},
					},
				},
			}
			config := containerConfig{task: &task}
			assert.Check(t, is.DeepEqual(c.to, config.hostConfig(nil).Mounts))
		})
	}
}
