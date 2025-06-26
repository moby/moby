//go:generate go-winres make --arch=386,amd64,arm,arm64 --in=./winresources/winres.json --out=./resource

package main

import _ "github.com/docker/docker/cmd/dockerd/winresources"
