package units

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var sizeRegex *regexp.Regexp

func init() {
	var err error
	if sizeRegex, err = regexp.Compile("^(\\d+)([kKmMgGtTpP])?[bB]?$"); err != nil {
		panic("Failed to compile the 'size' regular expression")
	}
}

// HumanSize returns a human-readable approximation of a size
// using SI standard (eg. "44kB", "17MB")
func HumanSize(size int64) string {
	units := []string{"B", "kB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"}
	i := 0
	sizef := float64(size)
	for sizef >= 1000.0 {
		sizef = sizef / 1000.0
		i++
	}
	return fmt.Sprintf("%.4g %s", sizef, units[i])
}

// FromHumanSize returns an integer from a human-readable specification of a size
// using SI standard (eg. "44kB", "17MB")
func FromHumanSize(size string) (int64, error) {
	matches := sizeRegex.FindStringSubmatch(size)

	if len(matches) != 3 {
		return -1, fmt.Errorf("Invalid size: '%s'", size)
	}

	theSize, err := strconv.ParseInt(matches[1], 10, 0)
	if err != nil {
		return -1, err
	}

	unit := strings.ToLower(matches[2])

	switch unit {
	case "k":
		theSize *= 1000
	case "m":
		theSize *= 1000 * 1000
	case "g":
		theSize *= 1000 * 1000 * 1000
	case "t":
		theSize *= 1000 * 1000 * 1000 * 1000
	case "p":
		theSize *= 1000 * 1000 * 1000 * 1000 * 1000
	}

	return theSize, nil
}

// Parses a human-readable string representing an amount of RAM
// in bytes, kibibytes, mebibytes, gibibytes, or tebibytes and
// returns the number of bytes, or -1 if the string is unparseable.
// Units are case-insensitive, and the 'b' suffix is optional.
func RAMInBytes(size string) (int64, error) {
	matches := sizeRegex.FindStringSubmatch(size)

	if len(matches) != 3 {
		return -1, fmt.Errorf("Invalid size: '%s'", size)
	}

	memLimit, err := strconv.ParseInt(matches[1], 10, 0)
	if err != nil {
		return -1, err
	}

	unit := strings.ToLower(matches[2])

	switch unit {
	case "k":
		memLimit *= 1024
	case "m":
		memLimit *= 1024 * 1024
	case "g":
		memLimit *= 1024 * 1024 * 1024
	case "t":
		memLimit *= 1024 * 1024 * 1024 * 1024
	case "p":
		memLimit *= 1024 * 1024 * 1024 * 1024 * 1024
	}

	return memLimit, nil
}
