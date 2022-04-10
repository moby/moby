package urlutil // import "github.com/docker/docker/pkg/urlutil"

import "github.com/docker/docker/builder/remotecontext/urlutil"

// IsURL returns true if the provided str is an HTTP(S) URL.
//
// Deprecated: use github.com/docker/docker/builder/remotecontext/urlutil.IsURL
// to detect build-context type, or use strings.HasPrefix() to check if the
// string has a https:// or http:// prefix.
func IsURL(str string) bool {
	// TODO(thaJeztah) when removing this alias, remove the exception from hack/validate/pkg-imports and hack/make.ps1 (Validate-PkgImports)
	return urlutil.IsURL(str)
}

// IsGitURL returns true if the provided str is a git repository URL.
//
// Deprecated: use github.com/docker/docker/builder/remotecontext/urlutil.IsGitURL
func IsGitURL(str string) bool {
	// TODO(thaJeztah) when removing this alias, remove the exception from hack/validate/pkg-imports and hack/make.ps1 (Validate-PkgImports)
	return urlutil.IsGitURL(str)
}
