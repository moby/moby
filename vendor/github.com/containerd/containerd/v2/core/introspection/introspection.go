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

package introspection

import (
	context "context"

	api "github.com/containerd/containerd/api/services/introspection/v1"
)

// Service defines the introspection service interface
type Service interface {
	Plugins(context.Context, ...string) (*api.PluginsResponse, error)
	Server(context.Context) (*api.ServerResponse, error)
	PluginInfo(context.Context, string, string, any) (*api.PluginInfoResponse, error)
}
