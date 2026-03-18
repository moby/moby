package contentutil

import (
	"net/url"
	"slices"
	"strings"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/pkg/reference"
)

func HasSource(info content.Info, refspec reference.Spec) (bool, error) {
	u, err := url.Parse("dummy://" + refspec.Locator)
	if err != nil {
		return false, err
	}

	if info.Labels == nil {
		return false, nil
	}

	source, target := u.Hostname(), strings.TrimPrefix(u.Path, "/")
	repoLabel, ok := info.Labels["containerd.io/distribution.source."+source]
	if !ok || repoLabel == "" {
		return false, nil
	}

	if slices.Contains(strings.Split(repoLabel, ","), target) {
		return true, nil
	}
	return false, nil
}
