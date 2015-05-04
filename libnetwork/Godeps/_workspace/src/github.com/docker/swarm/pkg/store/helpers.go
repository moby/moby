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

// Formats the key
func format(key string) string {
	return fullpath(splitKey(key))
}

// Formats the key partially (omits the first '/')
func partialFormat(key string) string {
	return partialpath(splitKey(key))
}

// Get the full directory part of the key
func getDir(key string) string {
	parts := splitKey(key)
	parts = parts[:len(parts)-1]
	return fullpath(parts)
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

// Get the full correct path representation of a splitted key/directory
func fullpath(path []string) string {
	return "/" + strings.Join(path, "/")
}

// Get the partial correct path representation of a splitted key/directory
// Omits the first '/'
func partialpath(path []string) string {
	return strings.Join(path, "/")
}
