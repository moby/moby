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

package registry

import (
	"sync"

	"github.com/containerd/plugin"
)

var register = struct {
	sync.RWMutex
	r plugin.Registry
}{}

// Register allows plugins to register
func Register(r *plugin.Registration) {
	register.Lock()
	defer register.Unlock()
	register.r = register.r.Register(r)
}

// Reset removes all global registrations
func Reset() {
	register.Lock()
	defer register.Unlock()
	register.r = nil
}

// Graph returns an ordered list of registered plugins for initialization.
// Plugins in disableList specified by id will be disabled.
func Graph(filter plugin.DisableFilter) []plugin.Registration {
	register.RLock()
	defer register.RUnlock()
	return register.r.Graph(filter)
}
