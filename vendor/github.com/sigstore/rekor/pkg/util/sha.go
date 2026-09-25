// Copyright 2022 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"crypto"
	"fmt"
	"strings"
)

// PrefixSHA sets the prefix of a sha hash to match how it is stored based on the length.
func PrefixSHA(sha string) string {
	var prefix string
	var components = strings.Split(sha, ":")

	if len(components) == 2 {
		return sha
	}

	switch len(sha) {
	case 40:
		prefix = "sha1:"
	case 64:
		prefix = "sha256:"
	case 96:
		prefix = "sha384:"
	case 128:
		prefix = "sha512:"
	}

	return fmt.Sprintf("%v%v", prefix, sha)
}

func UnprefixSHA(sha string) (crypto.Hash, string) {
	components := strings.Split(sha, ":")

	if len(components) == 2 {
		prefix := components[0]
		sha = components[1]

		switch prefix {
		case "sha1":
			return crypto.SHA1, sha
		case "sha256":
			return crypto.SHA256, sha
		case "sha384":
			return crypto.SHA384, sha
		case "sha512":
			return crypto.SHA512, sha
		default:
			return crypto.Hash(0), ""
		}
	}

	switch len(sha) {
	case 40:
		return crypto.SHA1, sha
	case 64:
		return crypto.SHA256, sha
	case 96:
		return crypto.SHA384, sha
	case 128:
		return crypto.SHA512, sha
	}

	return crypto.Hash(0), ""
}
