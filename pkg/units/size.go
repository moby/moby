package units

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	decimalKUnit = 1000
	binaryKUnit  = 1024
)

var sizeRegex *regexp.Regexp

func init() {
	sizeRegex = regexp.MustCompile("^(\\d+)([kKmMgGtTpP])?[bB]?$")
}

var bytePrefixes = [...]string{"B", "kB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"}

// HumanSize returns a human-readable approximation of a size
// using SI standard (eg. "44kB", "17MB")
func HumanSize(size int64) string {
	i := 0
	sizef := float64(size)
	for sizef >= 1000.0 {
		sizef = sizef / 1000.0
		i++
	}
	return fmt.Sprintf("%.4g %s", sizef, bytePrefixes[i])
}

// FromHumanSize returns an integer from a human-readable specification of a
// size using SI standard (eg. "44kB", "17MB")
func FromHumanSize(size string) (int64, error) {
	return parseSize(size, decimalKUnit)
}

// Parses a human-readable string representing an amount of RAM
// in bytes, kibibytes, mebibytes, gibibytes, or tebibytes and
// returns the number of bytes, or -1 if the string is unparseable.
// Units are case-insensitive, and the 'b' suffix is optional.
func RAMInBytes(size string) (int64, error) {
	return parseSize(size, binaryKUnit)
}

// Parses the human-readable size string into the amount it represents given
// the desired kilo unit [decimalKiloUnit=1000|binaryKiloUnit=1024]
func parseSize(size string, kUnit int64) (int64, error) {
	matches := sizeRegex.FindStringSubmatch(size)

	if len(matches) != 3 {
		return -1, fmt.Errorf("Invalid size: '%s'", size)
	}

	theSize, err := strconv.ParseInt(matches[1], 10, 0)
	if err != nil {
		return -1, err
	}

	unitPrefix := strings.ToLower(matches[2])

	switch unitPrefix {
	case "k":
		theSize *= kUnit
	case "m":
		theSize *= kUnit * kUnit
	case "g":
		theSize *= kUnit * kUnit * kUnit
	case "t":
		theSize *= kUnit * kUnit * kUnit * kUnit
	case "p":
		theSize *= kUnit * kUnit * kUnit * kUnit * kUnit
	}

	return theSize, nil
}
