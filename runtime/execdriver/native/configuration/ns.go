package configuration

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"strings"
)

func parseNsOpt(container *libcontainer.Container, opts []string) error {
	var (
		value = strings.TrimSpace(opts[0])
		ns    = container.Namespaces.Get(value[1:])
	)
	if ns == nil {
		return fmt.Errorf("%s is not a valid namespace", value[1:])
	}
	switch value[0] {
	case '-':
		ns.Enabled = false
	case '+':
		ns.Enabled = true
	default:
		return fmt.Errorf("%c is not a valid modifier for namespaces", value[0])
	}
	return nil
}
