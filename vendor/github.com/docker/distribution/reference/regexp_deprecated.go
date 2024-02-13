package reference

import (
	"github.com/distribution/reference"
)

// DigestRegexp matches well-formed digests, including algorithm (e.g. "sha256:<encoded>").
//
// Deprecated: use [reference.DigestRegexp].
var DigestRegexp = reference.DigestRegexp

// DomainRegexp matches hostname or IP-addresses, optionally including a port
// number. It defines the structure of potential domain components that may be
// part of image names. This is purposely a subset of what is allowed by DNS to
// ensure backwards compatibility with Docker image names. It may be a subset of
// DNS domain name, an IPv4 address in decimal format, or an IPv6 address between
// square brackets (excluding zone identifiers as defined by [RFC 6874] or special
// addresses such as IPv4-Mapped).
//
// Deprecated: use [reference.DomainRegexp].
//
// [RFC 6874]: https://www.rfc-editor.org/rfc/rfc6874.
var DomainRegexp = reference.DigestRegexp

// IdentifierRegexp is the format for string identifier used as a
// content addressable identifier using sha256. These identifiers
// are like digests without the algorithm, since sha256 is used.
//
// Deprecated: use [reference.IdentifierRegexp].
var IdentifierRegexp = reference.IdentifierRegexp

// NameRegexp is the format for the name component of references, including
// an optional domain and port, but without tag or digest suffix.
//
// Deprecated: use [reference.NameRegexp].
var NameRegexp = reference.NameRegexp

// ReferenceRegexp is the full supported format of a reference. The regexp
// is anchored and has capturing groups for name, tag, and digest
// components.
//
// Deprecated: use [reference.ReferenceRegexp].
var ReferenceRegexp = reference.ReferenceRegexp

// TagRegexp matches valid tag names. From [docker/docker:graph/tags.go].
//
// Deprecated: use [reference.TagRegexp].
//
// [docker/docker:graph/tags.go]: https://github.com/moby/moby/blob/v1.6.0/graph/tags.go#L26-L28
var TagRegexp = reference.TagRegexp
