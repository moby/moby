package fs

import (
	"fmt"
	"strconv"
	"strings"
)

// Parses a cgroup param and returns as name, value
//  i.e. "io_service_bytes 1234" will return as io_service_bytes, 1234
func getCgroupParamKeyValue(t string) (string, float64, error) {
	parts := strings.Fields(t)
	switch len(parts) {
	case 2:
		name := parts[0]
		value, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return "", 0.0, fmt.Errorf("Unable to convert param value to float: %s", err)
		}
		return name, value, nil
	default:
		return "", 0.0, fmt.Errorf("Unable to parse cgroup param: not enough parts; expected 2")
	}
}
