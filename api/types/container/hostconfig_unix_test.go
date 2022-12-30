//go:build !windows
// +build !windows

package container

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestCgroupnsMode(t *testing.T) {
	modes := map[CgroupnsMode]struct{ private, host, empty, valid bool }{
		"":                {private: false, host: false, empty: true, valid: true},
		"something:weird": {private: false, host: false, empty: false, valid: false},
		"host":            {private: false, host: true, empty: false, valid: true},
		"host:":           {valid: false},
		"host:name":       {valid: false},
		":name":           {valid: false},
		":":               {valid: false},
		"private":         {private: true, host: false, empty: false, valid: true},
		"private:name":    {private: false, host: false, empty: false, valid: false},
	}
	for mode, expected := range modes {
		t.Run("mode="+string(mode), func(t *testing.T) {
			assert.Check(t, is.Equal(mode.IsPrivate(), expected.private))
			assert.Check(t, is.Equal(mode.IsHost(), expected.host))
			assert.Check(t, is.Equal(mode.IsEmpty(), expected.empty))
			assert.Check(t, is.Equal(mode.Valid(), expected.valid))
		})
	}
}

// TODO Windows: This will need addressing for a Windows daemon.
func TestNetworkMode(t *testing.T) {
	modes := map[NetworkMode]struct {
		private, bridge, host, container, none, isDefault bool
		name                                              string
	}{
		"":                {private: true, bridge: false, host: false, container: false, none: false, isDefault: false, name: ""},
		"something:weird": {private: true, bridge: false, host: false, container: false, none: false, isDefault: false, name: "something:weird"},
		"bridge":          {private: true, bridge: true, host: false, container: false, none: false, isDefault: false, name: "bridge"},
		"host":            {private: false, bridge: false, host: true, container: false, none: false, isDefault: false, name: "host"},
		"container:name":  {private: false, bridge: false, host: false, container: true, none: false, isDefault: false, name: "container"},
		"none":            {private: true, bridge: false, host: false, container: false, none: true, isDefault: false, name: "none"},
		"default":         {private: true, bridge: false, host: false, container: false, none: false, isDefault: true, name: "default"},
	}
	for mode, expected := range modes {
		t.Run("mode="+string(mode), func(t *testing.T) {
			assert.Check(t, is.Equal(mode.IsPrivate(), expected.private))
			assert.Check(t, is.Equal(mode.IsBridge(), expected.bridge))
			assert.Check(t, is.Equal(mode.IsHost(), expected.host))
			assert.Check(t, is.Equal(mode.IsContainer(), expected.container))
			assert.Check(t, is.Equal(mode.IsNone(), expected.none))
			assert.Check(t, is.Equal(mode.IsDefault(), expected.isDefault))
			assert.Check(t, is.Equal(mode.NetworkName(), expected.name))
		})
	}
}

func TestIpcMode(t *testing.T) {
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
		"host:":                 {valid: false},
		"host:name":             {valid: false},
		":name":                 {valid: false},
		":":                     {valid: false},
		"container":             {},
		"container:":            {container: true, valid: true, ctrName: ""},
		"container:name":        {container: true, valid: true, ctrName: "name"},
		"container:name1:name2": {container: true, valid: true, ctrName: "name1:name2"},
		"shareable":             {shareable: true, valid: true},
	}

	for mode, expected := range ipcModes {
		t.Run("mode="+string(mode), func(t *testing.T) {
			assert.Check(t, is.Equal(mode.IsPrivate(), expected.private))
			assert.Check(t, is.Equal(mode.IsHost(), expected.host))
			assert.Check(t, is.Equal(mode.IsContainer(), expected.container))
			assert.Check(t, is.Equal(mode.IsShareable(), expected.shareable))
			assert.Check(t, is.Equal(mode.Valid(), expected.valid))
			assert.Check(t, is.Equal(mode.Container(), expected.ctrName))
		})
	}
}

func TestUTSMode(t *testing.T) {
	modes := map[UTSMode]struct{ private, host, valid bool }{
		"":                {private: true, host: false, valid: true},
		"something:weird": {private: true, host: false, valid: false},
		"host":            {private: false, host: true, valid: true},
		"host:":           {private: true, valid: false},
		"host:name":       {private: true, valid: false},
		":name":           {private: true, valid: false},
		":":               {private: true, valid: false},
	}
	for mode, expected := range modes {
		t.Run("mode="+string(mode), func(t *testing.T) {
			assert.Check(t, is.Equal(mode.IsPrivate(), expected.private))
			assert.Check(t, is.Equal(mode.IsHost(), expected.host))
			assert.Check(t, is.Equal(mode.Valid(), expected.valid))
		})

	}
}

func TestUsernsMode(t *testing.T) {
	modes := map[UsernsMode]struct{ private, host, valid bool }{
		"":                {private: true, host: false, valid: true},
		"something:weird": {private: true, host: false, valid: false},
		"host":            {private: false, host: true, valid: true},
		"host:":           {private: true, valid: false},
		"host:name":       {private: true, valid: false},
		":name":           {private: true, valid: false},
		":":               {private: true, valid: false},
	}
	for mode, expected := range modes {
		t.Run("mode="+string(mode), func(t *testing.T) {
			assert.Check(t, is.Equal(mode.IsPrivate(), expected.private))
			assert.Check(t, is.Equal(mode.IsHost(), expected.host))
			assert.Check(t, is.Equal(mode.Valid(), expected.valid))
		})
	}
}

func TestPidMode(t *testing.T) {
	modes := map[PidMode]struct{ private, host, valid bool }{
		"":                {private: true, host: false, valid: true},
		"something:weird": {private: true, host: false, valid: false},
		"host":            {private: false, host: true, valid: true},
		"host:":           {private: true, valid: false},
		"host:name":       {private: true, valid: false},
		":name":           {private: true, valid: false},
		":":               {private: true, valid: false},
	}
	for mode, expected := range modes {
		t.Run("mode="+string(mode), func(t *testing.T) {
			assert.Check(t, is.Equal(mode.IsPrivate(), expected.private))
			assert.Check(t, is.Equal(mode.IsHost(), expected.host))
			assert.Check(t, is.Equal(mode.Valid(), expected.valid))
		})
	}
}

func TestRestartPolicy(t *testing.T) {
	policies := map[RestartPolicy]struct{ none, always, onFailure bool }{
		{Name: "", MaximumRetryCount: 0}:           {none: true, always: false, onFailure: false},
		{Name: "something", MaximumRetryCount: 0}:  {none: false, always: false, onFailure: false},
		{Name: "no", MaximumRetryCount: 0}:         {none: true, always: false, onFailure: false},
		{Name: "always", MaximumRetryCount: 0}:     {none: false, always: true, onFailure: false},
		{Name: "on-failure", MaximumRetryCount: 0}: {none: false, always: false, onFailure: true},
	}
	for policy, expected := range policies {
		t.Run("policy="+policy.Name, func(t *testing.T) {
			assert.Check(t, is.Equal(policy.IsNone(), expected.none))
			assert.Check(t, is.Equal(policy.IsAlways(), expected.always))
			assert.Check(t, is.Equal(policy.IsOnFailure(), expected.onFailure))
		})
	}
}
