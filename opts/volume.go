package opts

import (
	"fmt"
	"regexp"

	mounttypes "github.com/docker/docker/api/types/mount"
)

// VolumeType denotes the syntax type of a volume string
type VolumeType int

const (
	// InvalidVolume should never be used
	InvalidVolume VolumeType = iota
	// LongVolume is the modern syntax that covers all the mount types
	LongVolume
	// ShortUnsplitVolume is the traditional syntax for volumes
	ShortUnsplitVolume
	// ShortSplitVolume is the traditional syntax for bind-mounts
	ShortSplitVolume
)

// InferVolumeType infers the VolumeType
func InferVolumeType(s string) VolumeType {
	// NOTE: we can't use `(,\\w+=\\w+)*` (originally suggested in #28527),
	// because `\w` lacks '/' and non-ASCII chars
	matchedLong, err := regexp.MatchString(".+=.+", s)
	if err != nil {
		// should not happen
		return InvalidVolume
	}
	if matchedLong {
		return LongVolume
	}
	if len(volumeSplitN(s, 2)) > 1 {
		return ShortSplitVolume
	}
	return ShortUnsplitVolume
}

// VolumeOpt is a Value type for parsing volumes
type VolumeOpt struct {
	raws          []string
	longs         []mounttypes.Mount
	shortUnsplits map[string]struct{} // for Config.
	shortSplits   []string            // for HostConfig.Binds
}

// Set a new mount value
func (v *VolumeOpt) Set(value string) error {
	switch InferVolumeType(value) {
	case LongVolume:
		mount, err := parseLongVolume(value)
		if err != nil {
			return err
		}
		v.longs = append(v.longs, mount)
	case ShortUnsplitVolume:
		if v.shortUnsplits == nil {
			v.shortUnsplits = make(map[string]struct{})
		}
		v.shortUnsplits[value] = struct{}{}
	case ShortSplitVolume:
		v.shortSplits = append(v.shortSplits, value)
	default:
		return fmt.Errorf("unknown volume type for %q", value)
	}
	v.raws = append(v.raws, value)
	return nil
}

// Type returns the type of this option
func (v *VolumeOpt) Type() string {
	return "volume"
}

// String returns a string repr of this option.
func (v *VolumeOpt) String() string {
	return fmt.Sprintf("%v", v.raws)
}

// LongValue returns the longs
func (v *VolumeOpt) LongValue() []mounttypes.Mount {
	return v.longs
}

// ShortUnsplitValue returns the short unsplits
func (v *VolumeOpt) ShortUnsplitValue() map[string]struct{} {
	return v.shortUnsplits
}

// ShortSplitValue returns the short splits
func (v *VolumeOpt) ShortSplitValue() []string {
	return v.shortSplits
}
