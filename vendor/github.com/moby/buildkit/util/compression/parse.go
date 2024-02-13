//go:build !nydus
// +build !nydus

package compression

func Parse(t string) (Type, error) {
	return parse(t)
}

func FromMediaType(mediaType string) (Type, error) {
	return fromMediaType(mediaType)
}
