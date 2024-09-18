package contentutil

import (
	"net/url"
	"strings"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/reference"
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

	for _, repo := range strings.Split(repoLabel, ",") {
		// the target repo is not a candidate
		if repo == target {
			return true, nil
		}
	}
	return false, nil
}
