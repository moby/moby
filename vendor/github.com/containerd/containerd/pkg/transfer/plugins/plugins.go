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

package plugins

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/containerd/errdefs"
	"github.com/containerd/typeurl/v2"
)

var register = struct {
	sync.RWMutex
	r map[string]reflect.Type
}{}

func Register(apiObject, transferObject interface{}) {
	url, err := typeurl.TypeURL(apiObject)
	if err != nil {
		panic(err)
	}
	// Lock
	register.Lock()
	defer register.Unlock()
	if register.r == nil {
		register.r = map[string]reflect.Type{}
	}
	if _, ok := register.r[url]; ok {
		panic(fmt.Sprintf("url already registered: %v", url))
	}
	t := reflect.TypeOf(transferObject)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	register.r[url] = t
}

func ResolveType(any typeurl.Any) (interface{}, error) {
	register.RLock()
	defer register.RUnlock()
	if register.r != nil {
		if t, ok := register.r[any.GetTypeUrl()]; ok {
			return reflect.New(t).Interface(), nil
		}
	}
	return nil, fmt.Errorf("%v not registered: %w", any.GetTypeUrl(), errdefs.ErrNotFound)
}
