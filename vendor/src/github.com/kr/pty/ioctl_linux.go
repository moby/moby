package pty

// from <asm-generic/ioctl.h>
const (
	_IOC_NRBITS   = 8
	_IOC_TYPEBITS = 8

	_IOC_SIZEBITS = 14
	_IOC_DIRBITS  = 2

	_IOC_NRSHIFT   = 0
	_IOC_TYPESHIFT = _IOC_NRSHIFT + _IOC_NRBITS
	_IOC_SIZESHIFT = _IOC_TYPESHIFT + _IOC_TYPEBITS
	_IOC_DIRSHIFT  = _IOC_SIZESHIFT + _IOC_SIZEBITS

	_IOC_NONE  uint = 0
	_IOC_WRITE uint = 1
	_IOC_READ  uint = 2
)

func _IOC(dir uint, ioctl_type byte, nr byte, size uintptr) uintptr {
	return (uintptr(dir)<<_IOC_DIRSHIFT |
		uintptr(ioctl_type)<<_IOC_TYPESHIFT |
		uintptr(nr)<<_IOC_NRSHIFT |
		size<<_IOC_SIZESHIFT)
}

func _IO(ioctl_type byte, nr byte) uintptr {
	return _IOC(_IOC_NONE, ioctl_type, nr, 0)
}

func _IOR(ioctl_type byte, nr byte, size uintptr) uintptr {
	return _IOC(_IOC_READ, ioctl_type, nr, size)
}

func _IOW(ioctl_type byte, nr byte, size uintptr) uintptr {
	return _IOC(_IOC_WRITE, ioctl_type, nr, size)
}

func _IOWR(ioctl_type byte, nr byte, size uintptr) uintptr {
	return _IOC(_IOC_READ|_IOC_WRITE, ioctl_type, nr, size)
}
