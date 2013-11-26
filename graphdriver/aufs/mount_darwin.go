package aufs

import "errors"

func mount(source string, target string, fstype string, flags uintptr, data string) (err error) {
	return errors.New("mount is not implemented on darwin")
}
