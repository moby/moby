/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package api

// DupStringSlice creates a copy of a string slice.
func DupStringSlice(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

// DupStringMap creates a copy of a map with string keys and values.
func DupStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

// IsMarkedForRemoval checks if a key is marked for removal.
//
// The key can be an annotation name, a mount container path, a device path,
// a namespace type, or an environment variable name.
// These are all marked for removal in adjustments by preceding
// their corresponding key with a '-'.
func IsMarkedForRemoval(key string) (string, bool) {
	if key == "" {
		return "", false
	}
	if key[0] != '-' {
		return key, false
	}
	return key[1:], true
}

// MarkForRemoval returns a key marked for removal.
func MarkForRemoval(key string) string {
	return "-" + key
}

// ClearRemovalMarker returns a key cleared from any removal marker.
func ClearRemovalMarker(key string) string {
	if key == "" {
		return ""
	}
	if key[0] == '-' {
		return key[1:]
	}
	return key
}
