package oci

import specs "github.com/opencontainers/runtime-spec/specs-go"

// RemoveNamespace removes the `nsType` namespace from OCI spec `s`
func RemoveNamespace(s *specs.Spec, nsType specs.NamespaceType) {
	idx := -1
	for i, n := range s.Linux.Namespaces {
		if n.Type == nsType {
			idx = i
		}
	}
	if idx >= 0 {
		s.Linux.Namespaces = append(s.Linux.Namespaces[:idx], s.Linux.Namespaces[idx+1:]...)
	}
}
