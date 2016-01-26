package data

import "github.com/docker/go/canonical/json"

// Serializer is an interface that can marshal and unmarshal TUF data.  This
// is expected to be a canonical JSON marshaller
type serializer interface {
	MarshalCanonical(from interface{}) ([]byte, error)
	Marshal(from interface{}) ([]byte, error)
	Unmarshal(from []byte, to interface{}) error
}

// CanonicalJSON marshals to and from canonical JSON
type canonicalJSON struct{}

// MarshalCanonical returns the canonical JSON form of a thing
func (c canonicalJSON) MarshalCanonical(from interface{}) ([]byte, error) {
	return json.MarshalCanonical(from)
}

// Marshal returns the regular non-canonical JSON form of a thing
func (c canonicalJSON) Marshal(from interface{}) ([]byte, error) {
	return json.Marshal(from)
}

// Unmarshal unmarshals some JSON bytes
func (c canonicalJSON) Unmarshal(from []byte, to interface{}) error {
	return json.Unmarshal(from, to)
}

// defaultSerializer is a canonical JSON serializer
var defaultSerializer serializer = canonicalJSON{}

func setDefaultSerializer(s serializer) {
	defaultSerializer = s
}
