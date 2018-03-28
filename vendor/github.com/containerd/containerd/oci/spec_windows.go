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

package oci

import (
	"context"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func createDefaultSpec(ctx context.Context, id string) (*specs.Spec, error) {
	return &specs.Spec{
		Version: specs.Version,
		Root:    &specs.Root{},
		Process: &specs.Process{
			Cwd: `C:\`,
			ConsoleSize: &specs.Box{
				Width:  80,
				Height: 20,
			},
		},
		Windows: &specs.Windows{
			IgnoreFlushesDuringBoot: true,
			Network: &specs.WindowsNetwork{
				AllowUnqualifiedDNSQuery: true,
			},
		},
	}, nil
}
