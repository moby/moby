//go:build !windows
// +build !windows

package network // import "github.com/docker/docker/daemon/network"

import (
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
)

func TestFilterNetworks(t *testing.T) {
	networks := []types.NetworkResource{
		{
			Name:   "host",
			Driver: "host",
			Scope:  "local",
		},
		{
			Name:   "bridge",
			Driver: "bridge",
			Scope:  "local",
		},
		{
			Name:   "none",
			Driver: "null",
			Scope:  "local",
		},
		{
			Name:   "myoverlay",
			Driver: "overlay",
			Scope:  "swarm",
		},
		{
			Name:   "mydrivernet",
			Driver: "mydriver",
			Scope:  "local",
		},
		{
			Name:   "mykvnet",
			Driver: "mykvdriver",
			Scope:  "global",
		},
		{
			Name:   "networkwithcontainer",
			Driver: "nwc",
			Scope:  "local",
			Containers: map[string]types.EndpointResource{
				"customcontainer": {
					Name: "customendpoint",
				},
			},
		},
	}

	testCases := []struct {
		filter      filters.Args
		resultCount int
		err         string
		name        string
		results     []string
	}{
		{
			filter:      filters.NewArgs(filters.Arg("driver", "bridge")),
			resultCount: 1,
			err:         "",
			name:        "bridge driver filters",
		},
		{
			filter:      filters.NewArgs(filters.Arg("driver", "overlay")),
			resultCount: 1,
			err:         "",
			name:        "overlay driver filters",
		},
		{
			filter:      filters.NewArgs(filters.Arg("driver", "noname")),
			resultCount: 0,
			err:         "",
			name:        "no name driver filters",
		},
		{
			filter:      filters.NewArgs(filters.Arg("type", "custom")),
			resultCount: 4,
			err:         "",
			name:        "custom driver filters",
		},
		{
			filter:      filters.NewArgs(filters.Arg("type", "builtin")),
			resultCount: 3,
			err:         "",
			name:        "builtin driver filters",
		},
		{
			filter:      filters.NewArgs(filters.Arg("type", "invalid")),
			resultCount: 0,
			err:         "invalid filter: 'type'='invalid'",
			name:        "invalid driver filters",
		},
		{
			filter:      filters.NewArgs(filters.Arg("scope", "local")),
			resultCount: 5,
			err:         "",
			name:        "local scope filters",
		},
		{
			filter:      filters.NewArgs(filters.Arg("scope", "swarm")),
			resultCount: 1,
			err:         "",
			name:        "swarm scope filters",
		},
		{
			filter:      filters.NewArgs(filters.Arg("scope", "global")),
			resultCount: 1,
			err:         "",
			name:        "global scope filters",
		},
		{
			filter:      filters.NewArgs(filters.Arg("dangling", "true")),
			resultCount: 3,
			err:         "",
			name:        "dangling filter is 'True'",
			results:     []string{"myoverlay", "mydrivernet", "mykvnet"},
		},
		{
			filter:      filters.NewArgs(filters.Arg("dangling", "false")),
			resultCount: 4,
			err:         "",
			name:        "dangling filter is 'False'",
			results:     []string{"host", "bridge", "none", "networkwithcontainer"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ls := make([]types.NetworkResource, 0, len(networks))
			ls = append(ls, networks...)
			result, err := FilterNetworks(ls, testCase.filter)
			if testCase.err != "" {
				if err == nil {
					t.Fatalf("expect error '%s', got no error", testCase.err)
				} else if !strings.Contains(err.Error(), testCase.err) {
					t.Fatalf("expect error '%s', got '%s'", testCase.err, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expect no error, got error '%s'", err)
				}
				// Make sure result is not nil
				if result == nil {
					t.Fatal("filterNetworks should not return nil")
				}

				if len(result) != testCase.resultCount {
					t.Fatalf("expect '%d' networks, got '%d' networks", testCase.resultCount, len(result))
				}

				if len(testCase.results) > 0 {
					resultMap := make(map[string]bool)
					for _, r := range result {
						resultMap[r.Name] = true
					}
					for _, r := range testCase.results {
						if _, ok := resultMap[r]; !ok {
							t.Fatalf("expected result: '%s' not found", r)
						}
					}
				}
			}
		})
	}
}
