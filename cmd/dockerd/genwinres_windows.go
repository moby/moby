//go:generate go-winres make --arch=386,amd64,arm,arm64 --in=../../cli/winresources/dockerd/winres.json --out=../../cli/winresources/dockerd/resource

package main

import _ "github.com/docker/docker/cli/winresources/dockerd"
