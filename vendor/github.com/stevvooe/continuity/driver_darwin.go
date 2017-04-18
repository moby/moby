package continuity

import "os"

func (d *driver) DeviceInfo(fi os.FileInfo) (maj uint64, min uint64, err error) {
	return deviceInfo(fi)
}
