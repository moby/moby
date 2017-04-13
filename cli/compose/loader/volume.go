package loader

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/cli/compose/types"
	"github.com/pkg/errors"
)

func parseVolume(spec string) (types.ServiceVolumeConfig, error) {
	volume := types.ServiceVolumeConfig{}

	switch len(spec) {
	case 0:
		return volume, errors.New("invalid empty volume spec")
	case 1, 2:
		volume.Target = spec
		volume.Type = string(mount.TypeVolume)
		return volume, nil
	}

	buffer := []rune{}
	for _, char := range spec {
		switch {
		case isWindowsDrive(char, buffer, volume):
			buffer = append(buffer, char)
		case char == ':':
			if err := populateFieldFromBuffer(char, buffer, &volume); err != nil {
				return volume, errors.Wrapf(err, "invalid spec: %s", spec)
			}
			buffer = []rune{}
		default:
			buffer = append(buffer, char)
		}
	}

	if err := populateFieldFromBuffer(rune(0), buffer, &volume); err != nil {
		return volume, errors.Wrapf(err, "invalid spec: %s", spec)
	}
	populateType(&volume)
	return volume, nil
}

func isWindowsDrive(char rune, buffer []rune, volume types.ServiceVolumeConfig) bool {
	return char == ':' && len(buffer) == 1 && unicode.IsLetter(buffer[0])
}

func populateFieldFromBuffer(char rune, buffer []rune, volume *types.ServiceVolumeConfig) error {
	strBuffer := string(buffer)
	switch {
	case len(buffer) == 0:
		return errors.New("empty section between colons")
	// Anonymous volume
	case volume.Source == "" && char == rune(0):
		volume.Target = strBuffer
		return nil
	case volume.Source == "":
		volume.Source = strBuffer
		return nil
	case volume.Target == "":
		volume.Target = strBuffer
		return nil
	case char == ':':
		return errors.New("too many colons")
	}
	for _, option := range strings.Split(strBuffer, ",") {
		switch option {
		case "ro":
			volume.ReadOnly = true
		case "rw":
			volume.ReadOnly = false
		case "nocopy":
			volume.Volume = &types.ServiceVolumeVolume{NoCopy: true}
		default:
			if isBindOption(option) {
				volume.Bind = &types.ServiceVolumeBind{Propagation: option}
			} else {
				return errors.Errorf("unknown option: %s", option)
			}
		}
	}
	return nil
}

func isBindOption(option string) bool {
	for _, propagation := range mount.Propagations {
		if mount.Propagation(option) == propagation {
			return true
		}
	}
	return false
}

func populateType(volume *types.ServiceVolumeConfig) {
	switch {
	// Anonymous volume
	case volume.Source == "":
		volume.Type = string(mount.TypeVolume)
	case isFilePath(volume.Source):
		volume.Type = string(mount.TypeBind)
	default:
		volume.Type = string(mount.TypeVolume)
	}
}

func isFilePath(source string) bool {
	switch source[0] {
	case '.', '/', '~':
		return true
	}

	// Windows absolute path
	first, next := utf8.DecodeRuneInString(source)
	if unicode.IsLetter(first) && source[next] == ':' {
		return true
	}
	return false
}
