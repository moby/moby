/*
 * ZLint Copyright 2023 Regents of the University of Michigan
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not
 * use this file except in compliance with the License. You may obtain a copy
 * of the License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
 * implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

package lint

import (
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/pelletier/go-toml"
)

// Configuration is a ZLint configuration which serves as a target
// to hold the full TOML tree that is a physical ZLint configuration./
type Configuration struct {
	tree *toml.Tree
}

// MaybeConfigure is a thin wrapper over Configure.
//
// If the provided lint object does not implement the Configurable interface
// then this function is a noop and nil is always returned.
//
// Otherwise, configuration of the provided lint is attempted.
func (c Configuration) MaybeConfigure(lint interface{}, namespace string) error {
	configurable, ok := lint.(Configurable)
	if !ok {
		return nil
	}
	return c.Configure(configurable.Configure(), namespace)
}

// Configure attempts to deserialize the provided namespace into the provided empty interface.
//
// For example, let's say that the name of your lint is MyLint, then the configuration
// file might look something like the following...
//
// ```
//
//	[MyLint]
//	A = 1
//	B = 2
//
// ```
//
// Given this, our target struct may look like the following...
//
// ```
//
//	type MytLint struct {
//		A int
//		B uint
//	}
//
// ```
//
// So deserializing into this struct would look like...
//
// ```
// configuration.Configure(&myLint, myLint.Name())
// ```
func (c Configuration) Configure(lint interface{}, namespace string) error {
	err := c.deserializeConfigInto(lint, namespace)
	if err != nil {
		details := fmt.Sprintf(
			"A fatal error occurred while attempting to configure %s. Please visit the [%s] section of "+
				"your provided configuration and compare it with the output of `zlint -exampleConfig`. Error: %s",
			namespace,
			namespace,
			err.Error())
		err = errors.New(details)
	}
	return err
}

// NewConfig attempts to instantiate a configuration by consuming the contents of the provided reader.
//
// The contents of the provided reader MUST be in a valid TOML format. The caller of this function
// is responsible for closing the reader, if appropriate.
func NewConfig(r io.Reader) (Configuration, error) {
	tree, err := toml.LoadReader(r)
	if err != nil {
		return Configuration{}, err
	}
	return Configuration{tree}, nil
}

// NewConfigFromFile attempts to instantiate a configuration from the provided filesystem path.
//
// The file pointed to by `path` MUST be valid TOML file. If `path` is the empty string then
// an empty configuration is returned.
func NewConfigFromFile(path string) (Configuration, error) {
	if path == "" {
		return NewEmptyConfig(), nil
	}
	f, err := os.Open(path)
	if err != nil {
		return Configuration{}, fmt.Errorf("failed to open the provided configuration at %s. Error: %s", path, err.Error())
	}
	defer f.Close()
	return NewConfig(f)
}

// NewConfigFromString attempts to instantiate a configuration from the provided string.
//
// The provided string MUST be in a valid TOML format.
func NewConfigFromString(config string) (Configuration, error) {
	return NewConfig(strings.NewReader(config))
}

// NewEmptyConfig returns a configuration that is backed by an entirely empty TOML tree.
//
// This is useful if no particular configuration is set at all by the user of ZLint as
// any attempt to resolve a namespace in `deserializeConfigInto` fails and thus results
// in all defaults for all lints being maintained.
func NewEmptyConfig() Configuration {
	cfg, _ := NewConfigFromString("")
	return cfg
}

// deserializeConfigInto deserializes the section labeled by the provided `namespace`
// into the provided target `interface{}`.
//
// For example, given the following configuration...
//
// ```
// [e_some_lint]
// field = 1
// flag = false
//
// [w_some_other_lint]
// is_web_pki = true
// ```
//
// And the following struct definition...
//
// ```
//
//	type SomeOtherLint {
//			IsWebPKI bool `toml:"is_web_pki"`
//	}
//
// ```
//
// Then the invocation of this function should be...
//
// ```
// lint := &SomeOtherLint{}
// deserializeConfigInto(lint, "w_some_other_lint")
// ```
//
// If there is no such namespace found in this configuration then provided the namespace specific data encoded
// within `target` is left unmodified. However, configuration of higher scoped fields will still be attempted.
func (c Configuration) deserializeConfigInto(target interface{}, namespace string) error {
	if tree := c.tree.Get(namespace); tree != nil {
		err := tree.(*toml.Tree).Unmarshal(target)
		if err != nil {
			return err
		}
	}
	return c.resolveHigherScopedReferences(target)
}

// resolveHigherScopeReferences takes in an interface{} value and attempts to
// find any field within its inner value that is either a struct or a pointer
// to a struct that is one of our global configurable types. If such a field
// exists then that higher scoped configuration will be copied into the value
// held by the provided interface{}.
//
// This procedure is recursive.
func (c Configuration) resolveHigherScopedReferences(i interface{}) error {
	value := reflect.Indirect(reflect.ValueOf(i))
	if value.Kind() != reflect.Struct {
		// Our target higher scoped configurations are either structs
		// or are fields of structs. Any other Kind simply cannot
		// be a target for deserialization here. For example, an interface
		// does not make sense since an interface cannot have fields nor
		// are any of our higher scoped configurations interfaces themselves.
		//
		// For a comprehensive list of Kinds, please see `type.go` in the `reflect` package.
		return nil
	}
	// Iterate through every field within the struct held by the provided interface{}.
	// If the field is either one of our higher scoped configurations (or a pointer to one)
	// then deserialize that higher scoped configuration into that field. If the field
	// is not one of our higher scoped configurations then recursively pass it to this function
	// in an attempt to resolve it.
	for field := 0; field < value.NumField(); field++ {
		field := value.Field(field)
		if !field.CanSet() {
			// This skips fields that are either not addressable or are private data members.
			continue
		}
		if _, ok := field.Interface().(GlobalConfiguration); ok {
			// It's one of our higher level configurations, so we need to pull out a different
			// subtree from our TOML document and inject it int othis struct.
			config := initializePtr(field).Interface().(GlobalConfiguration)
			err := c.deserializeConfigInto(config, config.namespace())
			if err != nil {
				return err
			}
			field.Set(reflect.ValueOf(config))
		} else {
			// This is just another member of some kind that is not one of our higher level configurations.
			err := c.resolveHigherScopedReferences(field.Addr().Interface())
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// stripGlobalsFromExample takes in an interface{} and returns a mapping that is
// the provided struct but with all references to higher scoped configurations scrubbed.
//
// This is intended only for use when constructing an example configuration file via the
// `-exampleConfig` flag. This is to avoid visually redundant, and possibly incorrect,
// examples such as the following...
//
// ```
// [Global]
// something = false
// something_else = ""
//
// [e_some_lint]
// my_data = 0
// my_flag = false
// globals = { something = false, something_else = "" }
// ```
//
// Notice how the above has Global effectively listed twice - once externally and once internally, which
// defeats the whole point of having globals to begin with.
func stripGlobalsFromExample(i interface{}) interface{} {
	value := reflect.Indirect(reflect.ValueOf(i))
	if value.Kind() != reflect.Struct {
		return i
	}
	m := map[string]interface{}{}
	for field := 0; field < value.NumField(); field++ {
		name := value.Type().Field(field).Name
		field := value.Field(field)
		if !field.CanInterface() {
			continue
		}
		if _, ok := field.Interface().(GlobalConfiguration); ok {
			continue
		}
		field = initializePtr(field)
		m[name] = stripGlobalsFromExample(field.Interface())
	}
	return m
}

// initializePtr checks whether the provided reflect.Value is a pointer type and is nil. If so, it returns
// a new reflect.Value that has an initialized pointer.
//
// If the provided reflect.Value is not a nil pointer, then the original reflect.Value is returned.
func initializePtr(value reflect.Value) reflect.Value {
	if value.Kind() == reflect.Ptr && value.IsZero() {
		return reflect.New(value.Type().Elem())
	}
	return value
}
