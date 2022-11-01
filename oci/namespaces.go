package oci // import "github.com/docker/docker/oci"

import specs "github.com/opencontainers/runtime-spec/specs-go"

// RemoveNamespace removes the `nsType` namespace from OCI spec `s`
func RemoveNamespace(s *specs.Spec, nsType specs.LinuxNamespaceType) {
	for i, n := range s.Linux.Namespaces {
		if n.Type == nsType {
			s.Linux.Namespaces = append(s.Linux.Namespaces[:i], s.Linux.Namespaces[i+1:]...)
			return
		}
	}
}

// NamespacePath returns the configured Path of the first namespace in
// s.Linux.Namespaces of type nsType.
func NamespacePath(s *specs.Spec, nsType specs.LinuxNamespaceType) (path string, ok bool) {
	for _, n := range s.Linux.Namespaces {
		if n.Type == nsType {
			return n.Path, true
		}
	}
	return "", false
}
