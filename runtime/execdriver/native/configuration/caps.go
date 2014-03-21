package configuration

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"strings"
)

// i.e: cap +MKNOD cap -NET_ADMIN
func parseCapOpt(container *libcontainer.Container, opts []string) error {
	var (
		value = strings.TrimSpace(opts[0])
		c     = container.CapabilitiesMask.Get(value[1:])
	)
	if c == nil {
		return fmt.Errorf("%s is not a valid capability", value[1:])
	}
	switch value[0] {
	case '-':
		c.Enabled = false
	case '+':
		c.Enabled = true
	default:
		return fmt.Errorf("%c is not a valid modifier for capabilities", value[0])
	}
	return nil
}
