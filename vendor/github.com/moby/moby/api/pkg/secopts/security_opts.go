package secopts

import (
	"errors"
	"fmt"
	"strings"

	"github.com/moby/moby/api/types/system"
)

// Decode decodes a security options string slice to a
// type-safe [system.SecurityOpt].
func Decode(opts []string) ([]system.SecurityOpt, error) {
	so := []system.SecurityOpt{}
	for _, opt := range opts {
		// support output from a < 1.13 docker daemon
		if !strings.Contains(opt, "=") {
			so = append(so, system.SecurityOpt{Name: opt})
			continue
		}
		secopt := system.SecurityOpt{}
		for _, s := range strings.Split(opt, ",") {
			k, v, ok := strings.Cut(s, "=")
			if !ok {
				return nil, fmt.Errorf("invalid security option %q", s)
			}
			if k == "" || v == "" {
				return nil, errors.New("invalid empty security option")
			}
			if k == "name" {
				secopt.Name = v
				continue
			}
			secopt.Options = append(secopt.Options, system.KeyValue{Key: k, Value: v})
		}
		so = append(so, secopt)
	}
	return so, nil
}
