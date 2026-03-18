// Copyright 2026 RedHat, Inc.
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

package activation

import "os"

// FilesWithNames maps fd names to a set of os.File pointers.
func FilesWithNames() map[string][]*os.File {
	files := Files(true)
	filesWithNames := map[string][]*os.File{}

	for _, f := range files {
		filesWithNames[f.Name()] = append(filesWithNames[f.Name()], f)
	}

	return filesWithNames
}
