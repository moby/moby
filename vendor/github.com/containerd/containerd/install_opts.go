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

package containerd

// InstallOpts configures binary installs
type InstallOpts func(*InstallConfig)

// InstallConfig sets the binary install configuration
type InstallConfig struct {
	// Libs installs libs from the image
	Libs bool
	// Replace will overwrite existing binaries or libs in the opt directory
	Replace bool
}

// WithInstallLibs installs libs from the image
func WithInstallLibs(c *InstallConfig) {
	c.Libs = true
}

// WithInstallReplace will replace existing files
func WithInstallReplace(c *InstallConfig) {
	c.Replace = true
}
