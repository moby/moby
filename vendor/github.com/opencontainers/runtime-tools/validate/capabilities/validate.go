package capabilities

import (
	"fmt"
	"strings"
	"sync"

	"github.com/moby/sys/capability"
)

// CapValid checks whether a capability is valid. If hostSpecific is set,
// it also checks that the capability is supported on the current host.
func CapValid(c string, hostSpecific bool) error {
	if !strings.HasPrefix(c, "CAP_") {
		return fmt.Errorf("capability %s must start with CAP_", c)
	}

	if _, ok := knownCaps()[c]; !ok {
		return fmt.Errorf("invalid capability: %s", c)
	}
	if !hostSpecific {
		return nil
	}
	if _, ok := supportedCaps()[c]; !ok {
		return fmt.Errorf("%s is not supported on the current host", c)
	}
	return nil
}

func capSet(list []capability.Cap) map[string]struct{} {
	m := make(map[string]struct{}, len(list))
	for _, c := range list {
		m["CAP_"+strings.ToUpper(c.String())] = struct{}{}
	}
	return m
}

var knownCaps = sync.OnceValue(func() map[string]struct{} {
	return capSet(capability.ListKnown())
})

var supportedCaps = sync.OnceValue(func() map[string]struct{} {
	list, _ := capability.ListSupported()
	return capSet(list)
})
