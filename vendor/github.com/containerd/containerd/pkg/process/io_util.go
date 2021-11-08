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

package process

import (
	"net/url"
	"os"

	exec "golang.org/x/sys/execabs"
)

// NewBinaryCmd returns a Cmd to be used to start a logging binary.
// The Cmd is generated from the provided uri, and the container ID and
// namespace are appended to the Cmd environment.
func NewBinaryCmd(binaryURI *url.URL, id, ns string) *exec.Cmd {
	var args []string
	for k, vs := range binaryURI.Query() {
		args = append(args, k)
		if len(vs) > 0 {
			args = append(args, vs[0])
		}
	}

	cmd := exec.Command(binaryURI.Path, args...)

	cmd.Env = append(cmd.Env,
		"CONTAINER_ID="+id,
		"CONTAINER_NAMESPACE="+ns,
	)

	return cmd
}

// CloseFiles closes any files passed in.
// It it used for cleanup in the event of unexpected errors.
func CloseFiles(files ...*os.File) {
	for _, file := range files {
		file.Close()
	}
}
