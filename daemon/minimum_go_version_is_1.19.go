//go:build !go1.19
package daemon
// We don't have a go.mod in this version, so this is a hacky way to prevent
// building this package with pre-1.19 Go version.
//
// If you really, really don't care about support and need to build using pre-1.19 version, please make sure to revendor the vendored dependencies (./hack/vendor.sh) and delete this file.
Purposeful_build_error_Minimum_supported_Go_version_is_1_19

