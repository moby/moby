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

package reference

import (
	"errors"
	"net/url"
	"path"
	"regexp"
	"strings"

	digest "github.com/opencontainers/go-digest"
)

var (
	// ErrInvalid is returned when there is an invalid reference
	ErrInvalid = errors.New("invalid reference")
	// ErrObjectRequired is returned when the object is required
	ErrObjectRequired = errors.New("object required")
	// ErrHostnameRequired is returned when the hostname is required
	ErrHostnameRequired = errors.New("hostname required")
)

// Spec defines the main components of a reference specification.
//
// A reference specification is a schema-less URI parsed into common
// components. The two main components, locator and object, are required to be
// supported by remotes. It represents a superset of the naming define in
// docker's reference schema. It aims to be compatible but not prescriptive.
//
// While the interpretation of the components, locator and object, are up to
// the remote, we define a few common parts, accessible via helper methods.
//
// The first is the hostname, which is part of the locator. This doesn't need
// to map to a physical resource, but it must parse as a hostname. We refer to
// this as the namespace.
//
// The other component made accessible by helper method is the digest. This is
// part of the object identifier, always prefixed with an '@'. If present, the
// remote may use the digest portion directly or resolve it against a prefix.
// If the object does not include the `@` symbol, the return value for `Digest`
// will be empty.
type Spec struct {
	// Locator is the host and path portion of the specification. The host
	// portion may refer to an actual host or just a namespace of related
	// images.
	//
	// Typically, the locator may used to resolve the remote to fetch specific
	// resources.
	Locator string

	// Object contains the identifier for the remote resource. Classically,
	// this is a tag but can refer to anything in a remote. By convention, any
	// portion that may be a partial or whole digest will be preceded by an
	// `@`. Anything preceding the `@` will be referred to as the "tag".
	//
	// In practice, we will see this broken down into the following formats:
	//
	// 1. <tag>
	// 2. <tag>@<digest spec>
	// 3. @<digest spec>
	//
	// We define the tag to be anything except '@' and ':'. <digest spec> may
	// be a full valid digest or shortened version, possibly with elided
	// algorithm.
	Object string
}

var splitRe = regexp.MustCompile(`[:@]`)

// Parse parses the string into a structured ref.
func Parse(s string) (Spec, error) {
	if strings.Contains(s, "://") {
		return Spec{}, ErrInvalid
	}

	u, err := url.Parse("dummy://" + s)
	if err != nil {
		return Spec{}, err
	}

	if u.Scheme != "dummy" {
		return Spec{}, ErrInvalid
	}

	if u.Host == "" {
		return Spec{}, ErrHostnameRequired
	}

	var object string

	if idx := splitRe.FindStringIndex(u.Path); idx != nil {
		// This allows us to retain the @ to signify digests or shortened digests in
		// the object.
		object = u.Path[idx[0]:]
		if object[:1] == ":" {
			object = object[1:]
		}
		u.Path = u.Path[:idx[0]]
	}

	return Spec{
		Locator: path.Join(u.Host, u.Path),
		Object:  object,
	}, nil
}

// Hostname returns the hostname portion of the locator.
//
// Remotes are not required to directly access the resources at this host. This
// method is provided for convenience.
func (r Spec) Hostname() string {
	i := strings.Index(r.Locator, "/")

	if i < 0 {
		return r.Locator
	}
	return r.Locator[:i]
}

// Digest returns the digest portion of the reference spec. This may be a
// partial or invalid digest, which may be used to lookup a complete digest.
func (r Spec) Digest() digest.Digest {
	i := strings.Index(r.Object, "@")

	if i < 0 {
		return ""
	}
	return digest.Digest(r.Object[i+1:])
}

// String returns the normalized string for the ref.
func (r Spec) String() string {
	if r.Object == "" {
		return r.Locator
	}
	if r.Object[:1] == "@" {
		return r.Locator + r.Object
	}

	return r.Locator + ":" + r.Object
}

// SplitObject provides two parts of the object spec, delimited by an "@"
// symbol. It does not perform any validation on correctness of the values
// returned, and it's the callers' responsibility to validate the result.
//
// If an "@" delimiter is found, it returns the part *including* the "@"
// delimiter as "tag", and the part after the "@" as digest.
//
// The example below produces "docker.io/library/ubuntu:latest@" and
// "sha256:deadbeef";
//
//	t, d := SplitObject("docker.io/library/ubuntu:latest@sha256:deadbeef")
//	fmt.Println(t) // docker.io/library/ubuntu:latest@
//	fmt.Println(d) // sha256:deadbeef
//
// Deprecated: use [Parse] and [Spec.Digest] instead.
func SplitObject(obj string) (tag string, dgst digest.Digest) {
	if i := strings.Index(obj, "@"); i >= 0 {
		// Offset by one so preserve the "@" in the tag returned.
		return obj[:i+1], digest.Digest(obj[i+1:])
	}
	return obj, ""
}
