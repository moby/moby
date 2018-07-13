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
	"time"

	"github.com/pkg/errors"
)

// as at MacOS 10.12 there is apparently no way to set timestamps
// with nanosecond precision. We could fall back to utimes/lutimes
// and lose the precision as a temporary workaround.
func chtimes(path string, atime, mtime time.Time) error {
	return errors.New("OSX missing UtimesNanoAt")
}
