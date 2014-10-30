// +build linux

package mount

func MakePrivate(mountPoint string) error {
	mounted, err := Mounted(mountPoint)
	if err != nil {
		return err
	}

	if !mounted {
		if err := Mount(mountPoint, mountPoint, "none", "bind,rw"); err != nil {
			return err
		}
	}

	return ForceMount("", mountPoint, "none", "private")
}
