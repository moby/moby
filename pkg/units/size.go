package units

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// HumanSize returns a human-readable approximation of a size
// using SI standard (eg. "44kB", "17MB")
func HumanSize(size int64) string {
	i := 0
	var sizef float64
	sizef = float64(size)
	units := []string{"B", "kB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"}
	for sizef >= 1000.0 {
		sizef = sizef / 1000.0
		i++
	}
	return fmt.Sprintf("%.4g %s", sizef, units[i])
}

// FromHumanSize returns an integer from a human-readable specification of a size
// using SI standard (eg. "44kB", "17MB")
func FromHumanSize(size string) (int64, error) {
	re, error := regexp.Compile("^(\\d+)([kKmMgGtTpP])?[bB]?$")
	if error != nil {
		return -1, fmt.Errorf("%s does not specify not a size", size)
	}

	matches := re.FindStringSubmatch(size)

	if len(matches) != 3 {
		return -1, fmt.Errorf("Invalid size: '%s'", size)
	}

	theSize, error := strconv.ParseInt(matches[1], 10, 0)
	if error != nil {
		return -1, error
	}

	unit := strings.ToLower(matches[2])

	if unit == "k" {
		theSize *= 1000
	} else if unit == "m" {
		theSize *= 1000 * 1000
	} else if unit == "g" {
		theSize *= 1000 * 1000 * 1000
	} else if unit == "t" {
		theSize *= 1000 * 1000 * 1000 * 1000
	} else if unit == "p" {
		theSize *= 1000 * 1000 * 1000 * 1000 * 1000
	}

	return theSize, nil
}

// Parses a human-readable string representing an amount of RAM
// in bytes, kibibytes, mebibytes or gibibytes, and returns the
// number of bytes, or -1 if the string is unparseable.
// Units are case-insensitive, and the 'b' suffix is optional.
func RAMInBytes(size string) (bytes int64, err error) {
	re, error := regexp.Compile("^(\\d+)([kKmMgG])?[bB]?$")
	if error != nil {
		return -1, error
	}

	matches := re.FindStringSubmatch(size)

	if len(matches) != 3 {
		return -1, fmt.Errorf("Invalid size: '%s'", size)
	}

	memLimit, error := strconv.ParseInt(matches[1], 10, 0)
	if error != nil {
		return -1, error
	}

	unit := strings.ToLower(matches[2])

	if unit == "k" {
		memLimit *= 1024
	} else if unit == "m" {
		memLimit *= 1024 * 1024
	} else if unit == "g" {
		memLimit *= 1024 * 1024 * 1024
	}

	return memLimit, nil
}
