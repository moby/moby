package security

import (
	"strings"
)

// Option contains the name and options of a security option
type Option struct {
	Name    string
	Options []KeyValue
}

// KeyValue holds a key/value pair.
type KeyValue struct {
	Key, Value string
}

// DecodeOptions decodes a security options string slice to a
// type-safe [Option].
func DecodeOptions(opts []string) []Option {
	so := make([]Option, 0, len(opts))
	for _, opt := range opts {
		secopt := Option{}
		for _, s := range strings.Split(opt, ",") {
			k, v, _ := strings.Cut(s, "=")
			if k == "" {
				continue
			}
			if k == "name" {
				secopt.Name = v
				continue
			}
			secopt.Options = append(secopt.Options, KeyValue{Key: k, Value: v})
		}
		if secopt.Name != "" {
			so = append(so, secopt)
		}
	}
	return so
}
