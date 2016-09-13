package assert

import (
	"reflect"
	"strings"
)

// CompareMultipleValues accepts two strings which contain comma-separated
// key=value pairs. Parses them into maps, then uses reflect.DeepEquals to
// assert they contain the same pairs (which may be in any order).
func CompareMultipleValues(t TestingT, value, expected string) {
	entriesMap := make(map[string]string)
	expMap := make(map[string]string)
	entries := strings.Split(value, ",")
	expectedEntries := strings.Split(expected, ",")
	for _, entry := range entries {
		keyval := strings.Split(entry, "=")
		entriesMap[keyval[0]] = keyval[1]
	}
	for _, expected := range expectedEntries {
		keyval := strings.Split(expected, "=")
		expMap[keyval[0]] = keyval[1]
	}
	if !reflect.DeepEqual(expMap, entriesMap) {
		fatal(t, "Expected entries: %v, got: %v", expected, value)
	}
}
