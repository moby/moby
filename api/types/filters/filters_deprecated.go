package filters

import (
	"encoding/json"

	"github.com/docker/docker/api/types/versions"
	"github.com/moby/moby/api/types/filters"
)

// ToParamWithVersion encodes Args as a JSON string. If version is less than 1.22
// then the encoded format will use an older legacy format where the values are a
// list of strings, instead of a set.
//
// Deprecated: do not use in any new code; use ToJSON instead
func ToParamWithVersion(version string, a filters.Args) (string, error) {
	if a.Len() == 0 {
		return "", nil
	}
	if version != "" && versions.LessThan(version, "1.22") {
		buf, err := json.Marshal(convertArgsToSlice(a))
		return string(buf), err
	}
	return ToJSON(a)
}

func convertArgsToSlice(f filters.Args) map[string][]string {
	m := map[string][]string{}
	for _, key := range f.Keys() {
		m[key] = f.Get(key)
	}
	return m
}
