package oci

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/libnetwork/resolvconf"
	"github.com/moby/buildkit/util/flightcontrol"
)

var g flightcontrol.Group
var notFirstRun bool
var lastNotEmpty bool

func GetResolvConf(ctx context.Context, stateDir string) (string, error) {
	p := filepath.Join(stateDir, "resolv.conf")
	_, err := g.Do(ctx, stateDir, func(ctx context.Context) (interface{}, error) {
		generate := !notFirstRun
		notFirstRun = true

		if !generate {
			fi, err := os.Stat(p)
			if err != nil {
				if !os.IsNotExist(err) {
					return "", err
				}
				generate = true
			}
			if !generate {
				fiMain, err := os.Stat(resolvconf.Path())
				if err != nil {
					if !os.IsNotExist(err) {
						return nil, err
					}
					if lastNotEmpty {
						generate = true
						lastNotEmpty = false
					}
				} else {
					if fi.ModTime().Before(fiMain.ModTime()) {
						generate = true
					}
				}
			}
		}

		if !generate {
			return "", nil
		}

		var dt []byte
		f, err := resolvconf.Get()
		if err != nil {
			if !os.IsNotExist(err) {
				return "", err
			}
		} else {
			dt = f.Content
		}

		f, err = resolvconf.FilterResolvDNS(dt, true)
		if err != nil {
			return "", err
		}

		if err := ioutil.WriteFile(p+".tmp", f.Content, 0644); err != nil {
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
