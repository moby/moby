package runtime

import (
	"fmt"

	"github.com/moby/moby/api/types/swarm"
)

// PluginSpec defines the base payload which clients can specify for creating
// a service with the plugin runtime.
type PluginSpec = swarm.RuntimeSpec

// PluginPrivilege describes a permission the user has to accept
// upon installing a plugin.
type PluginPrivilege = swarm.RuntimePrivilege

var (
	ErrInvalidLengthPlugin        = fmt.Errorf("proto: negative length found during unmarshaling") // Deprecated: this error was only used internally and is no longer used.
	ErrIntOverflowPlugin          = fmt.Errorf("proto: integer overflow")                          // Deprecated: this error was only used internally and is no longer used.
	ErrUnexpectedEndOfGroupPlugin = fmt.Errorf("proto: unexpected end of group")                   // Deprecated: this error was only used internally and is no longer used.
)
