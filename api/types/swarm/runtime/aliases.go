package runtime

import (
	"fmt"

	"github.com/moby/moby/api/types/swarm"
)

type PluginSpec = swarm.RuntimeSpec

type PluginPrivilege = swarm.RuntimePrivilege

var (
	ErrInvalidLengthPlugin        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowPlugin          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupPlugin = fmt.Errorf("proto: unexpected end of group")
)
