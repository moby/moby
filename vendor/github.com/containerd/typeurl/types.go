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

package typeurl

import (
	"encoding/json"
	"path"
	"reflect"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
)

var (
	mu       sync.RWMutex
	registry = make(map[reflect.Type]string)
)

// Definitions of common error types used throughout typeurl.
//
// These error types are used with errors.Wrap and errors.Wrapf to add context
// to an error.
//
// To detect an error class, use errors.Is() functions to tell whether an
// error is of this type.
var (
	ErrNotFound = errors.New("not found")
)

// Register a type with a base URL for JSON marshaling. When the MarshalAny and
// UnmarshalAny functions are called they will treat the Any type value as JSON.
// To use protocol buffers for handling the Any value the proto.Register
// function should be used instead of this function.
func Register(v interface{}, args ...string) {
	var (
		t = tryDereference(v)
		p = path.Join(args...)
	)
	mu.Lock()
	defer mu.Unlock()
	if et, ok := registry[t]; ok {
		if et != p {
			panic(errors.Errorf("type registered with alternate path %q != %q", et, p))
		}
		return
	}
	registry[t] = p
}

// TypeURL returns the type url for a registered type.
func TypeURL(v interface{}) (string, error) {
	mu.RLock()
	u, ok := registry[tryDereference(v)]
	mu.RUnlock()
	if !ok {
		// fallback to the proto registry if it is a proto message
		pb, ok := v.(proto.Message)
		if !ok {
			return "", errors.Wrapf(ErrNotFound, "type %s", reflect.TypeOf(v))
		}
		return proto.MessageName(pb), nil
	}
	return u, nil
}

// Is returns true if the type of the Any is the same as v.
func Is(any *types.Any, v interface{}) bool {
	// call to check that v is a pointer
	tryDereference(v)
	url, err := TypeURL(v)
	if err != nil {
		return false
	}
	return any.TypeUrl == url
}

// MarshalAny marshals the value v into an any with the correct TypeUrl.
// If the provided object is already a proto.Any message, then it will be
// returned verbatim. If it is of type proto.Message, it will be marshaled as a
// protocol buffer. Otherwise, the object will be marshaled to json.
func MarshalAny(v interface{}) (*types.Any, error) {
	var marshal func(v interface{}) ([]byte, error)
	switch t := v.(type) {
	case *types.Any:
		// avoid reserializing the type if we have an any.
		return t, nil
	case proto.Message:
		marshal = func(v interface{}) ([]byte, error) {
			return proto.Marshal(t)
		}
	default:
		marshal = json.Marshal
	}

	url, err := TypeURL(v)
	if err != nil {
		return nil, err
	}

	data, err := marshal(v)
	if err != nil {
		return nil, err
	}
	return &types.Any{
		TypeUrl: url,
		Value:   data,
	}, nil
}

// UnmarshalAny unmarshals the any type into a concrete type.
func UnmarshalAny(any *types.Any) (interface{}, error) {
	return UnmarshalByTypeURL(any.TypeUrl, any.Value)
}

// UnmarshalByTypeURL unmarshals the given type and value to into a concrete type.
func UnmarshalByTypeURL(typeURL string, value []byte) (interface{}, error) {
	return unmarshal(typeURL, value, nil)
}

// UnmarshalTo unmarshals the any type into a concrete type passed in the out
// argument. It is identical to UnmarshalAny, but lets clients provide a
// destination type through the out argument.
func UnmarshalTo(any *types.Any, out interface{}) error {
	return UnmarshalToByTypeURL(any.TypeUrl, any.Value, out)
}

// UnmarshalTo unmarshals the given type and value into a concrete type passed
// in the out argument. It is identical to UnmarshalByTypeURL, but lets clients
// provide a destination type through the out argument.
func UnmarshalToByTypeURL(typeURL string, value []byte, out interface{}) error {
	_, err := unmarshal(typeURL, value, out)
	return err
}

func unmarshal(typeURL string, value []byte, v interface{}) (interface{}, error) {
	t, err := getTypeByUrl(typeURL)
	if err != nil {
		return nil, err
	}

	if v == nil {
		v = reflect.New(t.t).Interface()
	} else {
		// Validate interface type provided by client
		vURL, err := TypeURL(v)
		if err != nil {
			return nil, err
		}
		if typeURL != vURL {
			return nil, errors.Errorf("can't unmarshal type %q to output %q", typeURL, vURL)
		}
	}

	if t.isProto {
		err = proto.Unmarshal(value, v.(proto.Message))
	} else {
		err = json.Unmarshal(value, v)
	}

	return v, err
}

type urlType struct {
	t       reflect.Type
	isProto bool
}

func getTypeByUrl(url string) (urlType, error) {
	mu.RLock()
	for t, u := range registry {
		if u == url {
			mu.RUnlock()
			return urlType{
				t: t,
			}, nil
		}
	}
	mu.RUnlock()
	// fallback to proto registry
	t := proto.MessageType(url)
	if t != nil {
		return urlType{
			// get the underlying Elem because proto returns a pointer to the type
			t:       t.Elem(),
			isProto: true,
		}, nil
	}
	return urlType{}, errors.Wrapf(ErrNotFound, "type with url %s", url)
}

func tryDereference(v interface{}) reflect.Type {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		// require check of pointer but dereference to register
		return t.Elem()
	}
	panic("v is not a pointer to a type")
}
