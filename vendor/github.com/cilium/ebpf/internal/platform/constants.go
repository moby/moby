package platform

import "fmt"

// Values used to tag platform specific constants.
//
// The value for Linux is zero so that existing constants do not change.
const (
	LinuxTag = uint32(iota) << platformShift
	WindowsTag
)

const (
	platformMax   = 1<<3 - 1 // most not exceed 3 bits to avoid setting the high bit
	platformShift = 28
	platformMask  = platformMax << platformShift
)

func tagForPlatform(platform string) (uint32, error) {
	switch platform {
	case Linux:
		return LinuxTag, nil
	case Windows:
		return WindowsTag, nil
	default:
		return 0, fmt.Errorf("unrecognized platform: %s", platform)
	}
}

func platformForConstant(c uint32) string {
	tag := uint32(c & platformMask)
	switch tag {
	case LinuxTag:
		return Linux
	case WindowsTag:
		return Windows
	default:
		return ""
	}
}

// Encode a platform and a value into a tagged constant.
//
// Returns an error if platform is unknown or c is out of bounds.
func EncodeConstant[T ~uint32](platform string, c uint32) (T, error) {
	if c>>platformShift > 0 {
		return 0, fmt.Errorf("invalid constant 0x%x", c)
	}

	tag, err := tagForPlatform(platform)
	if err != nil {
		return 0, err
	}

	return T(tag | c), nil
}

// Decode a platform and a value from a tagged constant.
func DecodeConstant[T ~uint32](c T) (string, uint32) {
	v := uint32(c) & ^uint32(platformMask)
	return platformForConstant(uint32(c)), v
}
