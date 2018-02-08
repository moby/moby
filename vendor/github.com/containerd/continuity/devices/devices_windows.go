package devices

import (
	"os"

	"github.com/pkg/errors"
)

func DeviceInfo(fi os.FileInfo) (uint64, uint64, error) {
	return 0, 0, errors.Wrap(ErrNotSupported, "cannot get device info on windows")
}
