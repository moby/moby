//go:build !windows
// +build !windows

package container

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestCgroupnsMode(t *testing.T) {
	modes := map[CgroupnsMode]struct{ valid, private, host, empty bool }{
		"":                {valid: true, private: false, host: false, empty: true},
		"something:weird": {valid: false, private: false, host: false, empty: false},
		"host":            {valid: true, private: false, host: true, empty: false},
		"host:":           {valid: false},
		"host:name":       {valid: false},
		":name":           {valid: false},
		":":               {valid: false},
		"private":         {valid: true, private: true, host: false, empty: false},
		"private:name":    {valid: false, private: false, host: false, empty: false},
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
		valid     bool
		private   bool
		host      bool
		container bool
		shareable bool
		ctrName   string
	}{
		"":                      {valid: true},
		"private":               {valid: true, private: true},
		"something:weird":       {valid: false},
		":weird":                {valid: false},
		"host":                  {valid: true, host: true},
		"host:":                 {valid: false},
		"host:name":             {valid: false},
		":name":                 {valid: false},
		":":                     {valid: false},
		"container":             {valid: false},
		"container:":            {valid: true, container: true, ctrName: ""},
		"container:name":        {valid: true, container: true, ctrName: "name"},
		"container:name1:name2": {valid: true, container: true, ctrName: "name1:name2"},
		"shareable":             {valid: true, shareable: true},
	}

	for mode, expected := range ipcModes {
		t.Run("mode="+string(mode), func(t *testing.T) {
			assert.Check(t, is.Equal(mode.Valid(), expected.valid))
			assert.Check(t, is.Equal(mode.IsPrivate(), expected.private))
			assert.Check(t, is.Equal(mode.IsHost(), expected.host))
			assert.Check(t, is.Equal(mode.IsContainer(), expected.container))
			assert.Check(t, is.Equal(mode.IsShareable(), expected.shareable))
			assert.Check(t, is.Equal(mode.Container(), expected.ctrName))
		})
	}
}

func TestUTSMode(t *testing.T) {
	modes := map[UTSMode]struct{ valid, private, host bool }{
		"":                {valid: true, private: true, host: false},
		"something:weird": {valid: false, private: true, host: false},
		"host":            {valid: true, private: false, host: true},
		"host:":           {valid: false, private: true},
		"host:name":       {valid: false, private: true},
		":name":           {valid: false, private: true},
		":":               {valid: false, private: true},
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
	modes := map[UsernsMode]struct{ valid, private, host bool }{
		"":                {valid: true, private: true, host: false},
		"something:weird": {valid: false, private: true, host: false},
		"host":            {valid: true, private: false, host: true},
		"host:":           {valid: false, private: true},
		"host:name":       {valid: false, private: true},
		":name":           {valid: false, private: true},
		":":               {valid: false, private: true},
	}
	for mode, expected := range modes {
		t.Run("mode="+string(mode), func(t *testing.T) {
			assert.Check(t, is.Equal(mode.Valid(), expected.valid))
			assert.Check(t, is.Equal(mode.IsPrivate(), expected.private))
			assert.Check(t, is.Equal(mode.IsHost(), expected.host))
		})
	}
}

func TestPidMode(t *testing.T) {
	modes := map[PidMode]struct{ valid, private, host bool }{
		"":                {valid: true, private: true, host: false},
		"something:weird": {valid: false, private: true, host: false},
		"host":            {valid: true, private: false, host: true},
		"host:":           {valid: false, private: true},
		"host:name":       {valid: false, private: true},
		":name":           {valid: false, private: true},
		":":               {valid: false, private: true},
	}
	for mode, expected := range modes {
		t.Run("mode="+string(mode), func(t *testing.T) {
			assert.Check(t, is.Equal(mode.Valid(), expected.valid))
			assert.Check(t, is.Equal(mode.IsPrivate(), expected.private))
			assert.Check(t, is.Equal(mode.IsHost(), expected.host))
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
