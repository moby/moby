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

package archive

import (
	"strings"

	"github.com/containerd/cri/pkg/util"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

// FilterRefPrefix restricts references to having the given image
// prefix. Tag-only references will have the prefix prepended.
func FilterRefPrefix(image string) func(string) string {
	return refTranslator(image, true)
}

// AddRefPrefix prepends the given image prefix to tag-only references,
// while leaving returning full references unmodified.
func AddRefPrefix(image string) func(string) string {
	return refTranslator(image, false)
}

// refTranslator creates a reference which only has a tag or verifies
// a full reference.
func refTranslator(image string, checkPrefix bool) func(string) string {
	return func(ref string) string {
		// Check if ref is full reference
		if strings.ContainsAny(ref, "/:@") {
			// If not prefixed, don't include image
			if checkPrefix && !isImagePrefix(ref, image) {
				return ""
			}
			return ref
		}
		return image + ":" + ref
	}
}

func isImagePrefix(s, prefix string) bool {
	if !strings.HasPrefix(s, prefix) {
		return false
	}
	if len(s) > len(prefix) {
		switch s[len(prefix)] {
		case '/', ':', '@':
			// Prevent matching partial namespaces
		default:
			return false
		}
	}
	return true
}

func normalizeReference(ref string) (string, error) {
	// TODO: Replace this function to not depend on reference package
	normalized, err := util.NormalizeImageRef(ref)
	if err != nil {
		return "", errors.Wrapf(err, "normalize image ref %q", ref)
	}

	return normalized.String(), nil
}

// DigestTranslator creates a digest reference by adding the
// digest to an image name
func DigestTranslator(prefix string) func(digest.Digest) string {
	return func(dgst digest.Digest) string {
		return prefix + "@" + dgst.String()
	}
}
