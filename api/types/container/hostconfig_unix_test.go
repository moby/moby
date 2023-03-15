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
		"":                {valid: true, empty: true},
		":":               {valid: false},
		"something":       {valid: false},
		"something:":      {valid: false},
		"something:weird": {valid: false},
		":weird":          {valid: false},
		"host":            {valid: true, host: true},
		"host:":           {valid: false},
		"host:name":       {valid: false},
		"private":         {valid: true, private: true},
		"private:name":    {valid: false, private: false},
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

func TestCgroupSpec(t *testing.T) {
	modes := map[CgroupSpec]struct {
		valid     bool
		private   bool
		host      bool
		container bool
		shareable bool
		ctrName   string
	}{
		"":                      {valid: true},
		":":                     {valid: false},
		"something":             {valid: false},
		"something:":            {valid: false},
		"something:weird":       {valid: false},
		":weird":                {valid: false},
		"container":             {valid: false},
		"container:":            {valid: true, container: true, ctrName: ""},
		"container:name":        {valid: true, container: true, ctrName: "name"},
		"container:name1:name2": {valid: true, container: true, ctrName: "name1:name2"},
	}

	for mode, expected := range modes {
		t.Run("mode="+string(mode), func(t *testing.T) {
			assert.Check(t, is.Equal(mode.Valid(), expected.valid))
			assert.Check(t, is.Equal(mode.IsContainer(), expected.container))
			assert.Check(t, is.Equal(mode.Container(), expected.ctrName))
		})
	}
}

// TODO Windows: This will need addressing for a Windows daemon.
func TestNetworkMode(t *testing.T) {
	// TODO(thaJeztah): we should consider the cases with a colon (":") in the network name to be invalid.
	modes := map[NetworkMode]struct {
		private, bridge, host, container, none, isDefault bool
		name, ctrName                                     string
	}{
		"":                      {private: true, name: ""},
		":":                     {private: true, name: ":"},
		"something":             {private: true, name: "something"},
		"something:":            {private: true, name: "something:"},
		"something:weird":       {private: true, name: "something:weird"},
		":weird":                {private: true, name: ":weird"},
		"bridge":                {private: true, bridge: true, name: "bridge"},
		"host":                  {private: false, host: true, name: "host"},
		"none":                  {private: true, none: true, name: "none"},
		"default":               {private: true, isDefault: true, name: "default"},
		"container":             {private: true, container: false, name: "container", ctrName: ""},
		"container:":            {private: false, container: true, name: "container", ctrName: ""},
		"container:name":        {private: false, container: true, name: "container", ctrName: "name"},
		"container:name1:name2": {private: false, container: true, name: "container", ctrName: "name1:name2"},
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
			assert.Check(t, is.Equal(mode.ConnectedContainer(), expected.ctrName))
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
		":":                     {valid: false},
		"something":             {valid: false},
		"something:":            {valid: false},
		"something:weird":       {valid: false},
		":weird":                {valid: false},
		"private":               {valid: true, private: true},
		"host":                  {valid: true, host: true},
		"host:":                 {valid: false},
		"host:name":             {valid: false},
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
		"":                {valid: true, private: true},
		":":               {valid: false, private: true},
		"something":       {valid: false, private: true},
		"something:":      {valid: false, private: true},
		"something:weird": {valid: false, private: true},
		":weird":          {valid: false, private: true},
		"host":            {valid: true, private: false, host: true},
		"host:":           {valid: false, private: true},
		"host:name":       {valid: false, private: true},
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
		"":                {valid: true, private: true},
		":":               {valid: false, private: true},
		"something":       {valid: false, private: true},
		"something:":      {valid: false, private: true},
		"something:weird": {valid: false, private: true},
		":weird":          {valid: false, private: true},
		"host":            {valid: true, private: false, host: true},
		"host:":           {valid: false, private: true},
		"host:name":       {valid: false, private: true},
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
	modes := map[PidMode]struct {
		valid     bool
		private   bool
		host      bool
		container bool
		ctrName   string
	}{
		"":                      {valid: true, private: true},
		":":                     {valid: false, private: true},
		"something":             {valid: false, private: true},
		"something:":            {valid: false, private: true},
		"something:weird":       {valid: false, private: true},
		":weird":                {valid: false, private: true},
		"host":                  {valid: true, private: false, host: true},
		"host:":                 {valid: false, private: true},
		"host:name":             {valid: false, private: true},
		"container":             {valid: false, private: true},
		"container:":            {valid: true, private: false, container: true, ctrName: ""},
		"container:name":        {valid: true, private: false, container: true, ctrName: "name"},
		"container:name1:name2": {valid: true, private: false, container: true, ctrName: "name1:name2"},
	}
	for mode, expected := range modes {
		t.Run("mode="+string(mode), func(t *testing.T) {
			assert.Check(t, is.Equal(mode.Valid(), expected.valid))
			assert.Check(t, is.Equal(mode.IsPrivate(), expected.private))
			assert.Check(t, is.Equal(mode.IsHost(), expected.host))
			assert.Check(t, is.Equal(mode.IsContainer(), expected.container))
			assert.Check(t, is.Equal(mode.Container(), expected.ctrName))
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
