package main

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/appc/spec/discovery"
)

var (
	cmdDiscover = &Command{
		Name:        "discover",
		Description: "Discover the download URLs for an app",
		Summary:     "Discover the download URLs for one or more app container images",
		Usage:       "APP...",
		Run:         runDiscover,
	}
)

func init() {
	cmdDiscover.Flags.BoolVar(&transportFlags.Insecure, "insecure", false,
		"Allow insecure non-TLS downloads over http")
}

func runDiscover(args []string) (exit int) {
	if len(args) < 1 {
		stderr("discover: at least one name required")
	}

	for _, name := range args {
		app, err := discovery.NewAppFromString(name)
		if app.Labels["os"] == "" {
			app.Labels["os"] = runtime.GOOS
		}
		if app.Labels["arch"] == "" {
			app.Labels["arch"] = runtime.GOARCH
		}
		if err != nil {
			stderr("%s: %s", name, err)
			return 1
		}
		eps, attempts, err := discovery.DiscoverEndpoints(*app, transportFlags.Insecure)
		if err != nil {
			stderr("error fetching %s: %s", name, err)
			return 1
		}
		for _, a := range attempts {
			fmt.Printf("discover walk: prefix: %s error: %v\n", a.Prefix, a.Error)
		}
		for _, aciEndpoint := range eps.ACIEndpoints {
			fmt.Printf("ACI: %s, ASC: %s\n", aciEndpoint.ACI, aciEndpoint.ASC)
		}
		if len(eps.Keys) > 0 {
			fmt.Println("Keys: " + strings.Join(eps.Keys, ","))
		}
	}

	return
}
