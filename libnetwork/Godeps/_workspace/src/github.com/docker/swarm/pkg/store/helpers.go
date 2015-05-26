package store

import (
	"strings"
)

// Creates a list of endpoints given the right scheme
func createEndpoints(addrs []string, scheme string) (entries []string) {
	for _, addr := range addrs {
		entries = append(entries, scheme+"://"+addr)
	}
	return entries
}

// Normalize the key for each store to the form:
//
//     /path/to/key
//
func normalize(key string) string {
	return "/" + join(splitKey(key))
}

// Get the full directory part of the key to the form:
//
//     /path/to/
//
func getDirectory(key string) string {
	parts := splitKey(key)
	parts = parts[:len(parts)-1]
	return "/" + join(parts)
}

// SplitKey splits the key to extract path informations
func splitKey(key string) (path []string) {
	if strings.Contains(key, "/") {
		path = strings.Split(key, "/")
	} else {
		path = []string{key}
	}
	return path
}

// Join the path parts with '/'
func join(parts []string) string {
	return strings.Join(parts, "/")
}
