package configuration

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"strings"
)

func parseFsOpts(container *libcontainer.Container, opts []string) error {
	opt := strings.TrimSpace(opts[0])

	switch opt {
	case "readonly":
		container.ReadonlyFs = true
	default:
		return fmt.Errorf("%s is not a valid filesystem option", opt)
	}
	return nil
}
