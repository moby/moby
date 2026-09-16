package local

import "strings"

// PlatformIDToPath maps an exporter platform ID to a single path component.
func PlatformIDToPath(id string) string {
	id = strings.NewReplacer("/", "_", `\`, "_", ":", "_").Replace(id)
	switch id {
	case ".", "..":
		return strings.Repeat("_", len(id))
	default:
		return id
	}
}
