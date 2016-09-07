// +build !windows

package runconfig

import (
	"bytes"
	"fmt"
	"io/ioutil"

	"github.com/docker/engine-api/types/container"
	"github.com/go-check/check"
)

// TODO Windows: This will need addressing for a Windows daemon.
func (s *DockerSuite) TestNetworkModeTest(c *check.C) {
	networkModes := map[container.NetworkMode][]bool{
		// private, bridge, host, container, none, default
		"":                         {true, false, false, false, false, false},
		"something:weird":          {true, false, false, false, false, false},
		"bridge":                   {true, true, false, false, false, false},
		DefaultDaemonNetworkMode(): {true, true, false, false, false, false},
		"host":           {false, false, true, false, false, false},
		"container:name": {false, false, false, true, false, false},
		"none":           {true, false, false, false, true, false},
		"default":        {true, false, false, false, false, true},
	}
	networkModeNames := map[container.NetworkMode]string{
		"":                         "",
		"something:weird":          "something:weird",
		"bridge":                   "bridge",
		DefaultDaemonNetworkMode(): "bridge",
		"host":           "host",
		"container:name": "container",
		"none":           "none",
		"default":        "default",
	}
	for networkMode, state := range networkModes {
		if networkMode.IsPrivate() != state[0] {
			c.Fatalf("NetworkMode.IsPrivate for %v should have been %v but was %v", networkMode, state[0], networkMode.IsPrivate())
		}
		if networkMode.IsBridge() != state[1] {
			c.Fatalf("NetworkMode.IsBridge for %v should have been %v but was %v", networkMode, state[1], networkMode.IsBridge())
		}
		if networkMode.IsHost() != state[2] {
			c.Fatalf("NetworkMode.IsHost for %v should have been %v but was %v", networkMode, state[2], networkMode.IsHost())
		}
		if networkMode.IsContainer() != state[3] {
			c.Fatalf("NetworkMode.IsContainer for %v should have been %v but was %v", networkMode, state[3], networkMode.IsContainer())
		}
		if networkMode.IsNone() != state[4] {
			c.Fatalf("NetworkMode.IsNone for %v should have been %v but was %v", networkMode, state[4], networkMode.IsNone())
		}
		if networkMode.IsDefault() != state[5] {
			c.Fatalf("NetworkMode.IsDefault for %v should have been %v but was %v", networkMode, state[5], networkMode.IsDefault())
		}
		if networkMode.NetworkName() != networkModeNames[networkMode] {
			c.Fatalf("Expected name %v, got %v", networkModeNames[networkMode], networkMode.NetworkName())
		}
	}
}

func (s *DockerSuite) TestIpcModeTest(c *check.C) {
	ipcModes := map[container.IpcMode][]bool{
		// private, host, container, valid
		"":                         {true, false, false, true},
		"something:weird":          {true, false, false, false},
		":weird":                   {true, false, false, true},
		"host":                     {false, true, false, true},
		"container:name":           {false, false, true, true},
		"container:name:something": {false, false, true, false},
		"container:":               {false, false, true, false},
	}
	for ipcMode, state := range ipcModes {
		if ipcMode.IsPrivate() != state[0] {
			c.Fatalf("IpcMode.IsPrivate for %v should have been %v but was %v", ipcMode, state[0], ipcMode.IsPrivate())
		}
		if ipcMode.IsHost() != state[1] {
			c.Fatalf("IpcMode.IsHost for %v should have been %v but was %v", ipcMode, state[1], ipcMode.IsHost())
		}
		if ipcMode.IsContainer() != state[2] {
			c.Fatalf("IpcMode.IsContainer for %v should have been %v but was %v", ipcMode, state[2], ipcMode.IsContainer())
		}
		if ipcMode.Valid() != state[3] {
			c.Fatalf("IpcMode.Valid for %v should have been %v but was %v", ipcMode, state[3], ipcMode.Valid())
		}
	}
	containerIpcModes := map[container.IpcMode]string{
		"":                      "",
		"something":             "",
		"something:weird":       "weird",
		"container":             "",
		"container:":            "",
		"container:name":        "name",
		"container:name1:name2": "name1:name2",
	}
	for ipcMode, container := range containerIpcModes {
		if ipcMode.Container() != container {
			c.Fatalf("Expected %v for %v but was %v", container, ipcMode, ipcMode.Container())
		}
	}
}

func (s *DockerSuite) TestUTSModeTest(c *check.C) {
	utsModes := map[container.UTSMode][]bool{
		// private, host, valid
		"":                {true, false, true},
		"something:weird": {true, false, false},
		"host":            {false, true, true},
		"host:name":       {true, false, true},
	}
	for utsMode, state := range utsModes {
		if utsMode.IsPrivate() != state[0] {
			c.Fatalf("UtsMode.IsPrivate for %v should have been %v but was %v", utsMode, state[0], utsMode.IsPrivate())
		}
		if utsMode.IsHost() != state[1] {
			c.Fatalf("UtsMode.IsHost for %v should have been %v but was %v", utsMode, state[1], utsMode.IsHost())
		}
		if utsMode.Valid() != state[2] {
			c.Fatalf("UtsMode.Valid for %v should have been %v but was %v", utsMode, state[2], utsMode.Valid())
		}
	}
}

func (s *DockerSuite) TestUsernsModeTest(c *check.C) {
	usrensMode := map[container.UsernsMode][]bool{
		// private, host, valid
		"":                {true, false, true},
		"something:weird": {true, false, false},
		"host":            {false, true, true},
		"host:name":       {true, false, true},
	}
	for usernsMode, state := range usrensMode {
		if usernsMode.IsPrivate() != state[0] {
			c.Fatalf("UsernsMode.IsPrivate for %v should have been %v but was %v", usernsMode, state[0], usernsMode.IsPrivate())
		}
		if usernsMode.IsHost() != state[1] {
			c.Fatalf("UsernsMode.IsHost for %v should have been %v but was %v", usernsMode, state[1], usernsMode.IsHost())
		}
		if usernsMode.Valid() != state[2] {
			c.Fatalf("UsernsMode.Valid for %v should have been %v but was %v", usernsMode, state[2], usernsMode.Valid())
		}
	}
}

func (s *DockerSuite) TestPidModeTest(c *check.C) {
	pidModes := map[container.PidMode][]bool{
		// private, host, valid
		"":                {true, false, true},
		"something:weird": {true, false, false},
		"host":            {false, true, true},
		"host:name":       {true, false, true},
	}
	for pidMode, state := range pidModes {
		if pidMode.IsPrivate() != state[0] {
			c.Fatalf("PidMode.IsPrivate for %v should have been %v but was %v", pidMode, state[0], pidMode.IsPrivate())
		}
		if pidMode.IsHost() != state[1] {
			c.Fatalf("PidMode.IsHost for %v should have been %v but was %v", pidMode, state[1], pidMode.IsHost())
		}
		if pidMode.Valid() != state[2] {
			c.Fatalf("PidMode.Valid for %v should have been %v but was %v", pidMode, state[2], pidMode.Valid())
		}
	}
}

func (s *DockerSuite) TestRestartPolicy(c *check.C) {
	restartPolicies := map[container.RestartPolicy][]bool{
		// none, always, failure
		container.RestartPolicy{}:                {true, false, false},
		container.RestartPolicy{"something", 0}:  {false, false, false},
		container.RestartPolicy{"no", 0}:         {true, false, false},
		container.RestartPolicy{"always", 0}:     {false, true, false},
		container.RestartPolicy{"on-failure", 0}: {false, false, true},
	}
	for restartPolicy, state := range restartPolicies {
		if restartPolicy.IsNone() != state[0] {
			c.Fatalf("RestartPolicy.IsNone for %v should have been %v but was %v", restartPolicy, state[0], restartPolicy.IsNone())
		}
		if restartPolicy.IsAlways() != state[1] {
			c.Fatalf("RestartPolicy.IsAlways for %v should have been %v but was %v", restartPolicy, state[1], restartPolicy.IsAlways())
		}
		if restartPolicy.IsOnFailure() != state[2] {
			c.Fatalf("RestartPolicy.IsOnFailure for %v should have been %v but was %v", restartPolicy, state[2], restartPolicy.IsOnFailure())
		}
	}
}
func (s *DockerSuite) TestDecodeHostConfig(c *check.C) {
	fixtures := []struct {
		file string
	}{
		{"fixtures/unix/container_hostconfig_1_14.json"},
		{"fixtures/unix/container_hostconfig_1_19.json"},
	}

	for _, f := range fixtures {
		b, err := ioutil.ReadFile(f.file)
		if err != nil {
			c.Fatal(err)
		}

		cc, err := DecodeHostConfig(bytes.NewReader(b))
		if err != nil {
			c.Fatal(fmt.Errorf("Error parsing %s: %v", f, err))
		}

		if cc.Privileged != false {
			c.Fatalf("Expected privileged false, found %v\n", cc.Privileged)
		}

		if l := len(cc.Binds); l != 1 {
			c.Fatalf("Expected 1 bind, found %d\n", l)
		}

		if len(cc.CapAdd) != 1 && cc.CapAdd[0] != "NET_ADMIN" {
			c.Fatalf("Expected CapAdd NET_ADMIN, got %v", cc.CapAdd)
		}

		if len(cc.CapDrop) != 1 && cc.CapDrop[0] != "NET_ADMIN" {
			c.Fatalf("Expected CapDrop MKNOD, got %v", cc.CapDrop)
		}
	}
}
