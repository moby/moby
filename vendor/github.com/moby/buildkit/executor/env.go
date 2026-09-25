package executor

import "strings"

// ReplaceEnv removes entries whose names are present in replacement, then
// appends replacement in order.
func ReplaceEnv(env, replacement []string) []string {
	names := make(map[string]struct{}, len(replacement))
	for _, entry := range replacement {
		name, _, _ := strings.Cut(entry, "=")
		names[name] = struct{}{}
	}
	out := make([]string, 0, len(env)+len(replacement))
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		if _, ok := names[name]; !ok {
			out = append(out, entry)
		}
	}
	return append(out, replacement...)
}
