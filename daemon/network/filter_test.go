//go:build !windows

package network

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"

	"github.com/moby/moby/api/types/filters"
)

type mockFilterNetwork struct {
	FilterNetwork
	name, driver, scope string
	containers          int
}

func (n mockFilterNetwork) Name() string {
	return n.name
}

func (n mockFilterNetwork) Driver() string {
	return n.driver
}

func (n mockFilterNetwork) Scope() string {
	return n.scope
}

func (n mockFilterNetwork) ContainerAttachments() int {
	return n.containers
}

func (n mockFilterNetwork) ServiceAttachments() int {
	return 0
}

func TestFilter(t *testing.T) {
	networks := []mockFilterNetwork{
		{
			name:   "host",
			driver: "host",
			scope:  "local",
		},
		{
			name:   "bridge",
			driver: "bridge",
			scope:  "local",
		},
		{
			name:   "none",
			driver: "null",
			scope:  "local",
		},
		{
			name:   "myoverlay",
			driver: "overlay",
			scope:  "swarm",
		},
		{
			name:   "mydrivernet",
			driver: "mydriver",
			scope:  "local",
		},
		{
			name:   "mykvnet",
			driver: "mykvdriver",
			scope:  "global",
		},
		{
			name:       "networkwithcontainer",
			driver:     "nwc",
			scope:      "local",
			containers: 1,
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
			flt, err := NewFilter(testCase.filter)
			if testCase.err != "" {
				if err == nil {
					t.Fatalf("expect error '%s', got no error", testCase.err)
				} else if !strings.Contains(err.Error(), testCase.err) {
					t.Fatalf("expect error '%s', got '%s'", testCase.err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expect no error, got error '%s'", err)
			}
			got := map[string]bool{}
			for _, nw := range networks {
				if flt.Matches(nw) {
					got[nw.Name()] = true
				}
			}

			assert.Check(t, is.Len(got, testCase.resultCount))
			if len(testCase.results) > 0 {
				want := map[string]bool{}
				for _, r := range testCase.results {
					want[r] = true
				}
				assert.Check(t, is.DeepEqual(got, want))
			}
		})
	}
}
