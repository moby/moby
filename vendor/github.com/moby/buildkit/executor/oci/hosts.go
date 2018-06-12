package oci

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
)

const hostsContent = `
127.0.0.1	localhost
::1	localhost ip6-localhost ip6-loopback
`

func GetHostsFile(ctx context.Context, stateDir string) (string, error) {
	p := filepath.Join(stateDir, "hosts")
	_, err := g.Do(ctx, stateDir, func(ctx context.Context) (interface{}, error) {
		_, err := os.Stat(p)
		if err == nil {
			return "", nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		if err := ioutil.WriteFile(p+".tmp", []byte(hostsContent), 0644); err != nil {
			return "", err
		}

		if err := os.Rename(p+".tmp", p); err != nil {
			return "", err
		}
		return "", nil
	})
	if err != nil {
		return "", err
	}
	return p, nil
}
