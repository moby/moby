//go:build !linux

// From https://github.com/containerd/nerdctl/pull/2723
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

package rootless

// DetachedNetNS returns non-empty netns path if RootlessKit is running with --detach-netns mode.
// Otherwise returns "" without an error.
func DetachedNetNS() (string, error) {
	return "", nil
}

// WithDetachedNetNSIfAny executes fn in [DetachedNetNS] if RootlessKit is running with --detach-netns mode.
// Otherwise it just executes fn in the current netns.
func WithDetachedNetNSIfAny(fn func() error) error {
	return fn()
}
