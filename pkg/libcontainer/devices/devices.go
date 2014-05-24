package devices

import (
	"fmt"
	"os"
	"syscall"
)

const (
	Wildcard = -1
)

type Device struct {
	Type              rune        `json:"type"`
	Path              string      `json:"path"`               // It is fine if this is an empty string in the case that you are using Wildcards
	MajorNumber       int64       `json:"major_number"`       // Use the wildcard constant for wildcards.
	MinorNumber       int64       `json:"minor_number"`       // Use the wildcard constant for wildcards.
	CgroupPermissions string      `json:"cgroup_permissions"` // Typically just "rwm"
	FileMode          os.FileMode `json:"file_mode"`          // The permission bits of the file's mode
}

func GetDeviceNumberString(deviceNumber int64) string {
	if deviceNumber == Wildcard {
		return "*"
	} else {
		return fmt.Sprintf("%d", deviceNumber)
	}
}

func GetDevice(path string, cgroupPermissions string) (Device, error) { // Given the path to a device and it's cgroup_permissions(which cannot be easilly queried) look up the information about a linux device and return that information as a Device struct.
	stat, err := os.Stat(path)
	if err != nil {
		return Device{}, err
	}

	devType := 'c'
	mode := stat.Mode()
	fileModePermissionBits := os.FileMode.Perm(mode)
	switch {
	case (mode & os.ModeDevice) == 0:
		return Device{}, fmt.Errorf("%s is not a device", path)
	case (mode & os.ModeCharDevice) != 0:
		fileModePermissionBits |= syscall.S_IFCHR
	default:
		fileModePermissionBits |= syscall.S_IFBLK
		devType = 'b'
	}

	devNumber := int(0)
	sys, ok := stat.Sys().(*syscall.Stat_t)
	if ok {
		devNumber = int(sys.Rdev)
	} else {
		return Device{}, fmt.Errorf("Cannot determine the device major and minor numbers")
	}

	device := Device{
		Type:              devType,
		Path:              path,
		MajorNumber:       Major(devNumber),
		MinorNumber:       Minor(devNumber),
		CgroupPermissions: cgroupPermissions,
		FileMode:          fileModePermissionBits,
	}
	return device, nil
}

var (
	DefaultAllowedDevices = []Device{
		// allow mknod for any device
		{
			Type:              'c',
			MajorNumber:       Wildcard,
			MinorNumber:       Wildcard,
			CgroupPermissions: "m",
		},
		{
			Type:              'b',
			MajorNumber:       Wildcard,
			MinorNumber:       Wildcard,
			CgroupPermissions: "m",
		},

		// /dev/null and zero
		{
			Path:              "/dev/null",
			Type:              'c',
			MajorNumber:       1,
			MinorNumber:       3,
			CgroupPermissions: "rwm",
		},
		{
			Path:              "/dev/zero",
			Type:              'c',
			MajorNumber:       1,
			MinorNumber:       5,
			CgroupPermissions: "rwm",
		},
		{
			Path:              "/dev/full",
			Type:              'c',
			MajorNumber:       1,
			MinorNumber:       7,
			CgroupPermissions: "rwm",
		},
		// consoles and ttys
		{
			Path:              "/dev/tty",
			Type:              'c',
			MajorNumber:       5,
			MinorNumber:       0,
			CgroupPermissions: "rwm",
		},
		{
			Path:              "/dev/console",
			Type:              'c',
			MajorNumber:       5,
			MinorNumber:       1,
			CgroupPermissions: "rwm",
		},
		{
			Path:              "/dev/tty0",
			Type:              'c',
			MajorNumber:       4,
			MinorNumber:       0,
			CgroupPermissions: "rwm",
		},
		{
			Path:              "/dev/tty1",
			Type:              'c',
			MajorNumber:       4,
			MinorNumber:       1,
			CgroupPermissions: "rwm",
		},

		// /dev/urandom,/dev/random
		{
			Path:              "/dev/urandom",
			Type:              'c',
			MajorNumber:       1,
			MinorNumber:       9,
			CgroupPermissions: "rwm",
		},
		{
			Path:              "/dev/random",
			Type:              'c',
			MajorNumber:       1,
			MinorNumber:       8,
			CgroupPermissions: "rwm",
		},

		// /dev/pts/ - pts namespaces are "coming soon"
		{
			Path:              "",
			Type:              'c',
			MajorNumber:       136,
			MinorNumber:       Wildcard,
			CgroupPermissions: "rwm",
		},
		{
			Path:              "",
			Type:              'c',
			MajorNumber:       5,
			MinorNumber:       2,
			CgroupPermissions: "rwm",
		},

		// tuntap
		{
			Path:              "",
			Type:              'c',
			MajorNumber:       10,
			MinorNumber:       200,
			CgroupPermissions: "rwm",
		},

		/*// fuse
		   {
		    Path: "",
		    Type: 'c',
		    MajorNumber: 10,
		    MinorNumber: 229,
		    CgroupPermissions: "rwm",
		   },

		// rtc
		   {
		    Path: "",
		    Type: 'c',
		    MajorNumber: 254,
		    MinorNumber: 0,
		    CgroupPermissions: "rwm",
		   },
		*/
	}

	DefaultAutoCreatedDevices = []Device{
		// /dev/null and zero
		{
			Path:              "/dev/null",
			Type:              'c',
			MajorNumber:       1,
			MinorNumber:       3,
			CgroupPermissions: "rwm",
			FileMode:          0666,
		},
		{
			Path:              "/dev/zero",
			Type:              'c',
			MajorNumber:       1,
			MinorNumber:       5,
			CgroupPermissions: "rwm",
			FileMode:          0666,
		},

		{
			Path:              "/dev/full",
			Type:              'c',
			MajorNumber:       1,
			MinorNumber:       7,
			CgroupPermissions: "rwm",
			FileMode:          0666,
		},

		// consoles and ttys
		{
			Path:              "/dev/tty",
			Type:              'c',
			MajorNumber:       5,
			MinorNumber:       0,
			CgroupPermissions: "rwm",
			FileMode:          0666,
		},

		// /dev/urandom,/dev/random
		{
			Path:              "/dev/urandom",
			Type:              'c',
			MajorNumber:       1,
			MinorNumber:       9,
			CgroupPermissions: "rwm",
			FileMode:          0666,
		},
		{
			Path:              "/dev/random",
			Type:              'c',
			MajorNumber:       1,
			MinorNumber:       8,
			CgroupPermissions: "rwm",
			FileMode:          0666,
		},
	}
)
