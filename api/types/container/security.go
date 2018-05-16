package container

import (
	"errors"
	"fmt"
	"strings"
)

// KeyValue holds a key/value pair
type KeyValue struct {
	Key, Value string
}

// SecurityOpt contains the name and options of a security option
type SecurityOpt struct {
	Name    string
	Options []KeyValue
}

// DecodeSecurityOptions decodes a security options string slice to a type safe
// SecurityOpt
func DecodeSecurityOptions(opts []string) ([]SecurityOpt, error) {
	so := []SecurityOpt{}
	for _, opt := range opts {
		// support output from a < 1.13 docker daemon
		if !strings.Contains(opt, "=") {
			so = append(so, SecurityOpt{Name: opt})
			continue
		}
		secopt := SecurityOpt{}
		split := strings.Split(opt, ",")
		for _, s := range split {
			kv := strings.SplitN(s, "=", 2)
			if len(kv) != 2 {
				return nil, fmt.Errorf("invalid security option %q", s)
			}
			if kv[0] == "" || kv[1] == "" {
				return nil, errors.New("invalid empty security option")
			}
			if kv[0] == "name" {
				secopt.Name = kv[1]
				continue
			}
			secopt.Options = append(secopt.Options, KeyValue{Key: kv[0], Value: kv[1]})
		}
		so = append(so, secopt)
	}
	return so, nil
}
