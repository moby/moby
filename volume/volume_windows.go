package volume

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Sirupsen/logrus"
	derr "github.com/docker/docker/errors"
)

// read-write modes
var rwModes = map[string]bool{
	"rw": true,
}

// read-only modes
var roModes = map[string]bool{
	"ro": true,
}

const (
	// Spec should be in the format [source:]destination[:mode]
	//
	// Examples: c:\foo bar:d:rw
	//           c:\foo:d:\bar
	//           myname:d:
	//           d:\
	//
	// Explanation of this regex! Thanks @thaJeztah on IRC and gist for help. See
	// https://gist.github.com/thaJeztah/6185659e4978789fb2b2. A good place to
	// test is https://regex-golang.appspot.com/assets/html/index.html
	//
	// Useful link for referencing named capturing groups:
	// http://stackoverflow.com/questions/20750843/using-named-matches-from-go-regex
	//
	// There are three match groups: source, destination and mode.
	//

	// RXHostDir is the first option of a source
	RXHostDir = `[a-z]:\\(?:[^\\/:*?"<>|\r\n]+\\?)*`
	// RXName is the second option of a source
	RXName = `[^\\/:*?"<>|\r\n]+`
	// RXReservedNames are reserved names not possible on Windows
	RXReservedNames = `(con)|(prn)|(nul)|(aux)|(com[1-9])|(lpt[1-9])`

	// RXSource is the combined possiblities for a source
	RXSource = `((?P<source>((` + RXHostDir + `)|(` + RXName + `))):)?`

	// Source. Can be either a host directory, a name, or omitted:
	//  HostDir:
	//    -  Essentially using the folder solution from
	//       https://www.safaribooksonline.com/library/view/regular-expressions-cookbook/9781449327453/ch08s18.html
	//       but adding case insensitivity.
	//    -  Must be an absolute path such as c:\path
	//    -  Can include spaces such as `c:\program files`
	//    -  And then followed by a colon which is not in the capture group
	//    -  And can be optional
	//  Name:
	//    -  Must not contain invalid NTFS filename characters (https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx)
	//    -  And then followed by a colon which is not in the capture group
	//    -  And can be optional

	// RXDestination is the regex expression for the mount destination
	RXDestination = `(?P<destination>([a-z]):((?:\\[^\\/:*?"<>\r\n]+)*\\?))`
	// Destination (aka container path):
	//    -  Variation on hostdir but can be a drive followed by colon as well
	//    -  If a path, must be absolute. Can include spaces
	//    -  Drive cannot be c: (explicitly checked in code, not RegEx)
	//

	// RXMode is the regex expression for the mode of the mount
	RXMode = `(:(?P<mode>(?i)rw))?`
	// Temporarily for TP4, disabling the use of ro as it's not supported yet
	// in the platform. TODO Windows: `(:(?P<mode>(?i)ro|rw))?`
	// mode (optional)
	//    -  Hopefully self explanatory in comparison to above.
	//    -  Colon is not in the capture group
	//
)

// BackwardsCompatible decides whether this mount point can be
// used in old versions of Docker or not.
// Windows volumes are never backwards compatible.
func (m *MountPoint) BackwardsCompatible() bool {
	return false
}

// ParseMountSpec validates the configuration of mount information is valid.
func ParseMountSpec(spec string, volumeDriver string) (*MountPoint, error) {
	var specExp = regexp.MustCompile(`^` + RXSource + RXDestination + RXMode + `$`)

	// Ensure in platform semantics for matching. The CLI will send in Unix semantics.
	match := specExp.FindStringSubmatch(filepath.FromSlash(strings.ToLower(spec)))

	// Must have something back
	if len(match) == 0 {
		return nil, derr.ErrorCodeVolumeInvalid.WithArgs(spec)
	}

	// Pull out the sub expressions from the named capture groups
	matchgroups := make(map[string]string)
	for i, name := range specExp.SubexpNames() {
		matchgroups[name] = strings.ToLower(match[i])
	}

	mp := &MountPoint{
		Source:      matchgroups["source"],
		Destination: matchgroups["destination"],
		RW:          true,
	}
	if strings.ToLower(matchgroups["mode"]) == "ro" {
		mp.RW = false
	}

	// Volumes cannot include an explicitly supplied mode eg c:\path:rw
	if mp.Source == "" && mp.Destination != "" && matchgroups["mode"] != "" {
		return nil, derr.ErrorCodeVolumeInvalid.WithArgs(spec)
	}

	// Note: No need to check if destination is absolute as it must be by
	// definition of matching the regex.

	if filepath.VolumeName(mp.Destination) == mp.Destination {
		// Ensure the destination path, if a drive letter, is not the c drive
		if strings.ToLower(mp.Destination) == "c:" {
			return nil, derr.ErrorCodeVolumeDestIsC.WithArgs(spec)
		}
	} else {
		// So we know the destination is a path, not drive letter. Clean it up.
		mp.Destination = filepath.Clean(mp.Destination)
		// Ensure the destination path, if a path, is not the c root directory
		if strings.ToLower(mp.Destination) == `c:\` {
			return nil, derr.ErrorCodeVolumeDestIsCRoot.WithArgs(spec)
		}
	}

	// See if the source is a name instead of a host directory
	if len(mp.Source) > 0 {
		validName, err := IsVolumeNameValid(mp.Source)
		if err != nil {
			return nil, err
		}
		if validName {
			// OK, so the source is a name.
			mp.Name = mp.Source
			mp.Source = ""

			// Set the driver accordingly
			mp.Driver = volumeDriver
			if len(mp.Driver) == 0 {
				mp.Driver = DefaultDriverName
			}
		} else {
			// OK, so the source must be a host directory. Make sure it's clean.
			mp.Source = filepath.Clean(mp.Source)
		}
	}

	// Ensure the host path source, if supplied, exists and is a directory
	if len(mp.Source) > 0 {
		var fi os.FileInfo
		var err error
		if fi, err = os.Stat(mp.Source); err != nil {
			return nil, derr.ErrorCodeVolumeSourceNotFound.WithArgs(mp.Source, err)
		}
		if !fi.IsDir() {
			return nil, derr.ErrorCodeVolumeSourceNotDirectory.WithArgs(mp.Source)
		}
	}

	logrus.Debugf("MP: Source '%s', Dest '%s', RW %t, Name '%s', Driver '%s'", mp.Source, mp.Destination, mp.RW, mp.Name, mp.Driver)
	return mp, nil
}

// IsVolumeNameValid checks a volume name in a platform specific manner.
func IsVolumeNameValid(name string) (bool, error) {
	nameExp := regexp.MustCompile(`^` + RXName + `$`)
	if !nameExp.MatchString(name) {
		return false, nil
	}
	nameExp = regexp.MustCompile(`^` + RXReservedNames + `$`)
	if nameExp.MatchString(name) {
		return false, derr.ErrorCodeVolumeNameReservedWord.WithArgs(name)
	}
	return true, nil
}
