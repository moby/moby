// DO NOT EDIT
// This file has been auto-generated with go generate.

package v17_06_1

import specs "github.com/opencontainers/runtime-spec/specs-go" // a45ba0989fc26c695fe166a49c45bb8b7618ab36 https://github.com/docker/runtime-spec

type ProcessState struct {
	Terminal        bool                `json:"terminal,omitempty"`
	ConsoleSize     specs.Box           `json:"consoleSize,omitempty"`
	User            specs.User          `json:"user"`
	Args            []string            `json:"args"`
	Env             []string            `json:"env,omitempty"`
	Cwd             string              `json:"cwd"`
	Capabilities    linuxCapabilities   `json:"capabilities,omitempty" platform:"linux"`
	Rlimits         []specs.LinuxRlimit `json:"rlimits,omitempty" platform:"linux"`
	NoNewPrivileges bool                `json:"noNewPrivileges,omitempty" platform:"linux"`
	ApparmorProfile string              `json:"apparmorProfile,omitempty" platform:"linux"`
	SelinuxLabel    string              `json:"selinuxLabel,omitempty" platform:"linux"`
	Exec            bool                `json:"exec"`
	Stdin           string              `json:"containerdStdin"`
	Stdout          string              `json:"containerdStdout"`
	Stderr          string              `json:"containerdStderr"`
	RuntimeArgs     []string            `json:"runtimeArgs"`
	NoPivotRoot     bool                `json:"noPivotRoot"`
	Checkpoint      string              `json:"checkpoint"`
	RootUID         int                 `json:"rootUID"`
	RootGID         int                 `json:"rootGID"`
}
