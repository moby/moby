// +build ignore

package continuity

import "os"

var (
	devNullResource = resource{
		kind:  chardev,
		path:  "/dev/null",
		major: 3,
		minor: 2,
		mode:  0666 | os.ModeDevice | os.ModeCharDevice,
	}

	devZeroResource = resource{
		kind:  chardev,
		path:  "/dev/zero",
		major: 3,
		minor: 3,
		mode:  0666 | os.ModeDevice | os.ModeCharDevice,
	}
)
