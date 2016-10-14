// Copyright 2015 LinkedIn Corp. Licensed under the Apache License,
// Version 2.0 (the "License"); you may not use this file except in
// compliance with the License.  You may obtain a copy of the License
// at http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.Copyright [201X] LinkedIn Corp. Licensed under the Apache
// License, Version 2.0 (the "License"); you may not use this file
// except in compliance with the License.  You may obtain a copy of
// the License at http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.

package goavro

import (
	"fmt"
	"strings"
)

const (
	nullNamespace = ""
)

type name struct {
	n   string // name
	ns  string // namespace
	ens string // enclosing namespace
}

type nameSetter func(*name) error

func newName(setters ...nameSetter) (*name, error) {
	var err error
	n := &name{}
	for _, setter := range setters {
		if err = setter(n); err != nil {
			return nil, err
		}
	}
	// if name contains dot, then ignore namespace and enclosing namespace
	if !strings.ContainsRune(n.n, '.') {
		if n.ns != "" {
			n.n = n.ns + "." + n.n
		} else if n.ens != "" {
			n.n = n.ens + "." + n.n
		}
	}
	return n, nil
}

func nameSchema(schema map[string]interface{}) nameSetter {
	return func(n *name) error {
		val, ok := schema["name"]
		if !ok {
			return fmt.Errorf("ought to have name key")
		}
		n.n, ok = val.(string)
		if !ok || len(n.n) == 0 {
			return fmt.Errorf("name ought to be non-empty string: %T", n)
		}
		if val, ok := schema["namespace"]; ok {
			n.ns, ok = val.(string)
			if !ok {
				return fmt.Errorf("namespace ought to be a string: %T", n)
			}
		}
		return nil
	}
}

// ErrInvalidName is returned when a Codec cannot be created due to
// invalid name format.
type ErrInvalidName struct {
	Message string
}

func (e ErrInvalidName) Error() string {
	return "The name portion of a fullname, record field names, and enum symbols must " + e.Message
}

func isRuneInvalidForFirstCharacter(r rune) bool {
	if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_' || r == '.' {
		return false
	}
	return true
}

func isRuneInvalidForOtherCharacters(r rune) bool {
	if r >= '0' && r <= '9' {
		return false
	}
	return isRuneInvalidForFirstCharacter(r)
}

func checkName(s string) error {
	if len(s) == 0 {
		return &ErrInvalidName{"not be empty"}
	}
	if strings.IndexFunc(s[:1], isRuneInvalidForFirstCharacter) != -1 {
		return &ErrInvalidName{"start with [A-Za-z_]"}
	}
	if strings.IndexFunc(s[1:], isRuneInvalidForOtherCharacters) != -1 {
		return &ErrInvalidName{"have second and remaining characters contain only [A-Za-z0-9_]"}
	}
	return nil
}

func nameName(someName string) nameSetter {
	return func(n *name) (err error) {
		if err = checkName(someName); err == nil {
			n.n = someName
		}
		return
	}
}

func nameEnclosingNamespace(someNamespace string) nameSetter {
	return func(n *name) error {
		n.ens = someNamespace
		return nil
	}
}

func nameNamespace(someNamespace string) nameSetter {
	return func(n *name) error {
		n.ns = someNamespace
		return nil
	}
}

func (n *name) equals(b *name) bool {
	if n.n == b.n {
		return true
	}
	return false
}

func (n name) namespace() string {
	li := strings.LastIndex(n.n, ".")
	if li == -1 {
		return ""
	}
	return n.n[:li]
}

func (n name) GoString() string {
	return n.n
}

func (n name) String() string {
	return n.n
}
