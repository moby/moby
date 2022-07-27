package overrides

import (
	"errors"

	"github.com/docker/distribution/reference"
)

// SanitizeRepoAndTags parses the raw names to a slice of repoAndTag.
// It removes duplicates and validates each repoName and tag to not contain a digest.
func SanitizeRepoAndTags(names []string) (repoAndTags []string, err error) {
	uniqNames := map[string]struct{}{}
	for _, repo := range names {
		if repo == "" {
			continue
		}

		ref, err := reference.ParseNormalizedNamed(repo)
		if err != nil {
			return nil, err
		}

		if _, ok := ref.(reference.Digested); ok {
			return nil, errors.New("build tag cannot contain a digest")
		}

		nameWithTag := reference.TagNameOnly(ref).String()
		if _, exists := uniqNames[nameWithTag]; !exists {
			uniqNames[nameWithTag] = struct{}{}
			repoAndTags = append(repoAndTags, nameWithTag)
		}
	}
	return repoAndTags, nil
}
