package volume // import "github.com/docker/docker/volume"

import (
	"fmt"

	"github.com/docker/docker/api/types/mount"
	"github.com/pkg/errors"
)

var errBindNotExist = errors.New("bind source path does not exist")

type errMountConfig struct {
	mount *mount.Mount
	err   error
}

func (e *errMountConfig) Error() string {
	return fmt.Sprintf("invalid mount config for type %q: %v", e.mount.Type, e.err.Error())
}

func errExtraField(name string) error {
	return errors.Errorf("field %s must not be specified", name)
}
func errMissingField(name string) error {
	return errors.Errorf("field %s must not be empty", name)
}
