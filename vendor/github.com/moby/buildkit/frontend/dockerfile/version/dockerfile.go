package version

import (
	"strings"

	buildkitversion "github.com/moby/buildkit/version"
	"github.com/pkg/errors"
	"golang.org/x/mod/semver"
)

func Version() string {
	v, err := normalizeDockerfileVersion(version, buildkitversion.Version)
	if err != nil {
		return ""
	}
	return v
}

func normalizeDockerfileVersion(v, buildkitVersion string) (string, error) {
	v = strings.TrimSpace(v)
	parts := strings.Split(v, ".")
	if len(parts) != 2 && len(parts) != 3 {
		return "", errors.Errorf("invalid dockerfile frontend version %q", v)
	}
	for _, part := range parts {
		if part == "" {
			return "", errors.Errorf("invalid dockerfile frontend version %q", v)
		}
		for _, ch := range part {
			if ch < '0' || ch > '9' {
				return "", errors.Errorf("invalid dockerfile frontend version %q", v)
			}
		}
	}
	if len(parts) == 2 {
		v += ".0"
	}
	if !semver.IsValid(buildkitVersion) {
		return v + "-dev", nil
	}
	if prerelease := semver.Prerelease(buildkitVersion); prerelease != "" {
		return v + prerelease, nil
	}
	if semver.Build(buildkitVersion) != "" {
		return v + "-dev", nil
	}
	return v, nil
}
