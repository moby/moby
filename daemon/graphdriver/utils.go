package graphdriver

import (
	"fmt"
	"strings"
)

// ParseStorageOptKeyValue parses and validates the specified string as a key/value
// pair (key=value).
func ParseStorageOptKeyValue(opt string) (key string, value string, err error) {
	k, v, ok := strings.Cut(opt, "=")
	if !ok {
		return "", "", fmt.Errorf("unable to parse storage-opt key/value: %s", opt)
	}
	return strings.TrimSpace(k), strings.TrimSpace(v), nil
}
