// SPDX-License-Identifier: Apache-2.0
/*
 * Copyright (C) 2024-2025 Aleksa Sarai <cyphar@cyphar.com>
 * Copyright (C) 2024-2025 SUSE LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package pathrs

import (
	"strings"
)

// IsLexicallyInRoot is shorthand for strings.HasPrefix(path+"/", root+"/"),
// but properly handling the case where path or root have a "/" suffix.
//
// NOTE: The return value only make sense if the path is already mostly cleaned
// (i.e., doesn't contain "..", ".", nor unneeded "/"s).
func IsLexicallyInRoot(root, path string) bool {
	root = strings.TrimRight(root, "/")
	path = strings.TrimRight(path, "/")
	return strings.HasPrefix(path+"/", root+"/")
}
