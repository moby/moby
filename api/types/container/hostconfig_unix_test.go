//go:build !windows
// +build !windows

package container

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestCgroupnsModeTest(t *testing.T) {
	cgroupNsModes := map[CgroupnsMode][]bool{
		// private, host, empty, valid
		"":                {false, false, true, true},
		"something:weird": {false, false, false, false},
		"host":            {false, true, false, true},
		"host:name":       {false, false, false, false},
		"private":         {true, false, false, true},
		"private:name":    {false, false, false, false},
	}
	for cgroupNsMode, state := range cgroupNsModes {
		if cgroupNsMode.IsPrivate() != state[0] {
			t.Fatalf("CgroupnsMode.IsPrivate for %v should have been %v but was %v", cgroupNsMode, state[0], cgroupNsMode.IsPrivate())
		}
		if cgroupNsMode.IsHost() != state[1] {
			t.Fatalf("CgroupnsMode.IsHost for %v should have been %v but was %v", cgroupNsMode, state[1], cgroupNsMode.IsHost())
		}
		if cgroupNsMode.IsEmpty() != state[2] {
			t.Fatalf("CgroupnsMode.Valid for %v should have been %v but was %v", cgroupNsMode, state[2], cgroupNsMode.Valid())
		}
		if cgroupNsMode.Valid() != state[3] {
			t.Fatalf("CgroupnsMode.Valid for %v should have been %v but was %v", cgroupNsMode, state[2], cgroupNsMode.Valid())
		}
	}
}

// TODO Windows: This will need addressing for a Windows daemon.
func TestNetworkModeTest(t *testing.T) {
	networkModes := map[NetworkMode][]bool{
		// private, bridge, host, container, none, default
		"":                {true, false, false, false, false, false},
		"something:weird": {true, false, false, false, false, false},
		"bridge":          {true, true, false, false, false, false},
		"host":            {false, false, true, false, false, false},
		"container:name":  {false, false, false, true, false, false},
		"none":            {true, false, false, false, true, false},
		"default":         {true, false, false, false, false, true},
	}
	networkModeNames := map[NetworkMode]string{
		"":                "",
		"something:weird": "something:weird",
		"bridge":          "bridge",
		"host":            "host",
		"container:name":  "container",
		"none":            "none",
		"default":         "default",
	}
	for networkMode, state := range networkModes {
		if networkMode.IsPrivate() != state[0] {
			t.Fatalf("NetworkMode.IsPrivate for %v should have been %v but was %v", networkMode, state[0], networkMode.IsPrivate())
		}
		if networkMode.IsBridge() != state[1] {
			t.Fatalf("NetworkMode.IsBridge for %v should have been %v but was %v", networkMode, state[1], networkMode.IsBridge())
		}
		if networkMode.IsHost() != state[2] {
			t.Fatalf("NetworkMode.IsHost for %v should have been %v but was %v", networkMode, state[2], networkMode.IsHost())
		}
		if networkMode.IsContainer() != state[3] {
			t.Fatalf("NetworkMode.IsContainer for %v should have been %v but was %v", networkMode, state[3], networkMode.IsContainer())
		}
		if networkMode.IsNone() != state[4] {
			t.Fatalf("NetworkMode.IsNone for %v should have been %v but was %v", networkMode, state[4], networkMode.IsNone())
		}
		if networkMode.IsDefault() != state[5] {
			t.Fatalf("NetworkMode.IsDefault for %v should have been %v but was %v", networkMode, state[5], networkMode.IsDefault())
		}
		if networkMode.NetworkName() != networkModeNames[networkMode] {
			t.Fatalf("Expected name %v, got %v", networkModeNames[networkMode], networkMode.NetworkName())
		}
	}
}

func TestIpcModeTest(t *testing.T) {
	ipcModes := map[IpcMode]struct {
		private   bool
		host      bool
		container bool
		shareable bool
		valid     bool
		ctrName   string
	}{
		"":                      {valid: true},
		"private":               {private: true, valid: true},
		"something:weird":       {},
		":weird":                {},
		"host":                  {host: true, valid: true},
		"container":             {},
		"container:":            {container: true, valid: true, ctrName: ""},
		"container:name":        {container: true, valid: true, ctrName: "name"},
		"container:name1:name2": {container: true, valid: true, ctrName: "name1:name2"},
		"shareable":             {shareable: true, valid: true},
	}

	for ipcMode, state := range ipcModes {
		assert.Check(t, is.Equal(state.private, ipcMode.IsPrivate()), "IpcMode.IsPrivate() parsing failed for %q", ipcMode)
		assert.Check(t, is.Equal(state.host, ipcMode.IsHost()), "IpcMode.IsHost()  parsing failed for %q", ipcMode)
		assert.Check(t, is.Equal(state.container, ipcMode.IsContainer()), "IpcMode.IsContainer()  parsing failed for %q", ipcMode)
		assert.Check(t, is.Equal(state.shareable, ipcMode.IsShareable()), "IpcMode.IsShareable()  parsing failed for %q", ipcMode)
		assert.Check(t, is.Equal(state.valid, ipcMode.Valid()), "IpcMode.Valid()  parsing failed for %q", ipcMode)
		assert.Check(t, is.Equal(state.ctrName, ipcMode.Container()), "IpcMode.Container() parsing failed for %q", ipcMode)
	}
}

func TestUTSModeTest(t *testing.T) {
	utsModes := map[UTSMode][]bool{
		// private, host, valid
		"":                {true, false, true},
		"something:weird": {true, false, false},
		"host":            {false, true, true},
		"host:name":       {true, false, true},
	}
	for utsMode, state := range utsModes {
		if utsMode.IsPrivate() != state[0] {
			t.Fatalf("UtsMode.IsPrivate for %v should have been %v but was %v", utsMode, state[0], utsMode.IsPrivate())
		}
		if utsMode.IsHost() != state[1] {
			t.Fatalf("UtsMode.IsHost for %v should have been %v but was %v", utsMode, state[1], utsMode.IsHost())
		}
		if utsMode.Valid() != state[2] {
			t.Fatalf("UtsMode.Valid for %v should have been %v but was %v", utsMode, state[2], utsMode.Valid())
		}
	}
}

func TestUsernsModeTest(t *testing.T) {
	usrensMode := map[UsernsMode][]bool{
		// private, host, valid
		"":                {true, false, true},
		"something:weird": {true, false, false},
		"host":            {false, true, true},
		"host:name":       {true, false, true},
	}
	for usernsMode, state := range usrensMode {
		if usernsMode.IsPrivate() != state[0] {
			t.Fatalf("UsernsMode.IsPrivate for %v should have been %v but was %v", usernsMode, state[0], usernsMode.IsPrivate())
		}
		if usernsMode.IsHost() != state[1] {
			t.Fatalf("UsernsMode.IsHost for %v should have been %v but was %v", usernsMode, state[1], usernsMode.IsHost())
		}
		if usernsMode.Valid() != state[2] {
			t.Fatalf("UsernsMode.Valid for %v should have been %v but was %v", usernsMode, state[2], usernsMode.Valid())
		}
	}
}

func TestPidModeTest(t *testing.T) {
	pidModes := map[PidMode][]bool{
		// private, host, valid
		"":                {true, false, true},
		"something:weird": {true, false, false},
		"host":            {false, true, true},
		"host:name":       {true, false, true},
	}
	for pidMode, state := range pidModes {
		if pidMode.IsPrivate() != state[0] {
			t.Fatalf("PidMode.IsPrivate for %v should have been %v but was %v", pidMode, state[0], pidMode.IsPrivate())
		}
		if pidMode.IsHost() != state[1] {
			t.Fatalf("PidMode.IsHost for %v should have been %v but was %v", pidMode, state[1], pidMode.IsHost())
		}
		if pidMode.Valid() != state[2] {
			t.Fatalf("PidMode.Valid for %v should have been %v but was %v", pidMode, state[2], pidMode.Valid())
		}
	}
}

func TestRestartPolicy(t *testing.T) {
	restartPolicies := map[RestartPolicy][]bool{
		// none, always, failure
		{}: {true, false, false},
		{Name: "something", MaximumRetryCount: 0}:  {false, false, false},
		{Name: "no", MaximumRetryCount: 0}:         {true, false, false},
		{Name: "always", MaximumRetryCount: 0}:     {false, true, false},
		{Name: "on-failure", MaximumRetryCount: 0}: {false, false, true},
	}
	for restartPolicy, state := range restartPolicies {
		if restartPolicy.IsNone() != state[0] {
			t.Fatalf("RestartPolicy.IsNone for %v should have been %v but was %v", restartPolicy, state[0], restartPolicy.IsNone())
		}
		if restartPolicy.IsAlways() != state[1] {
			t.Fatalf("RestartPolicy.IsAlways for %v should have been %v but was %v", restartPolicy, state[1], restartPolicy.IsAlways())
		}
		if restartPolicy.IsOnFailure() != state[2] {
			t.Fatalf("RestartPolicy.IsOnFailure for %v should have been %v but was %v", restartPolicy, state[2], restartPolicy.IsOnFailure())
		}
	}
}
