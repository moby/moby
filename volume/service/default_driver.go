// +build linux windows

package service // import "github.com/moby/moby/volume/service"
import (
	"github.com/moby/moby/pkg/idtools"
	"github.com/moby/moby/volume"
	"github.com/moby/moby/volume/drivers"
	"github.com/moby/moby/volume/local"
	"github.com/pkg/errors"
)

func setupDefaultDriver(store *drivers.Store, root string, rootIDs idtools.Identity) error {
	d, err := local.New(root, rootIDs)
	if err != nil {
		return errors.Wrap(err, "error setting up default driver")
	}
	if !store.Register(d, volume.DefaultDriverName) {
		return errors.New("local volume driver could not be registered")
	}
	return nil
}
