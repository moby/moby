package capabilities

import (
	"fmt"
	"strings"

	"github.com/syndtr/gocapability/capability"
)

// CapValid checks whether a capability is valid
func CapValid(c string, hostSpecific bool) error {
	isValid := false

	if !strings.HasPrefix(c, "CAP_") {
		return fmt.Errorf("capability %s must start with CAP_", c)
	}
	for _, cap := range capability.List() {
		if c == fmt.Sprintf("CAP_%s", strings.ToUpper(cap.String())) {
			if hostSpecific && cap > LastCap() {
				return fmt.Errorf("%s is not supported on the current host", c)
			}
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf("invalid capability: %s", c)
	}
	return nil
}
