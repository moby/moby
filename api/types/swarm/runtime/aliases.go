package runtime

import (
	swarmruntime "github.com/moby/moby/api/types/swarm/runtime"
)

type PluginSpec = swarmruntime.PluginSpec

type PluginPrivilege = swarmruntime.PluginPrivilege

var (
	ErrInvalidLengthPlugin        = swarmruntime.ErrInvalidLengthPlugin
	ErrIntOverflowPlugin          = swarmruntime.ErrIntOverflowPlugin
	ErrUnexpectedEndOfGroupPlugin = swarmruntime.ErrUnexpectedEndOfGroupPlugin
)
