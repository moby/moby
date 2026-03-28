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

// RunInDetachedNetNS runs f in the detached network namespace if one is
// configured, otherwise runs f directly.
func RunInDetachedNetNS(f func() error) error {
	return f()
}

// MarkInSandboxNS is a no-op on non-Linux platforms.
func MarkInSandboxNS() {}

// UnmarkInSandboxNS is a no-op on non-Linux platforms.
func UnmarkInSandboxNS() {}

// InSandboxNS always returns false on non-Linux platforms.
func InSandboxNS() bool { return false }
