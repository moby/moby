//go:build linux

package bridge

import (
	"net"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/types"
	"gotest.tools/v3/assert"
)

func TestPortBindingMatchesAny(t *testing.T) {
	gwIP := net.ParseIP("172.18.0.2")
	// A representative operational binding for a published ingress port.
	pb := types.PortBinding{
		Proto:    types.TCP,
		IP:       gwIP,
		Port:     8080,
		HostIP:   net.IPv4zero,
		HostPort: 8080,
	}

	tests := []struct {
		name string
		reqs []types.PortBinding
		want bool
	}{
		{
			name: "exact match",
			reqs: []types.PortBinding{{Proto: types.TCP, IP: gwIP, Port: 8080, HostPort: 8080}},
			want: true,
		},
		{
			name: "protocol differs",
			reqs: []types.PortBinding{{Proto: types.UDP, IP: gwIP, Port: 8080, HostPort: 8080}},
			want: false,
		},
		{
			name: "container port differs",
			reqs: []types.PortBinding{{Proto: types.TCP, IP: gwIP, Port: 9090, HostPort: 8080}},
			want: false,
		},
		{
			name: "host port differs",
			reqs: []types.PortBinding{{Proto: types.TCP, IP: gwIP, Port: 8080, HostPort: 9090}},
			want: false,
		},
		{
			name: "unspecified host port matches any",
			reqs: []types.PortBinding{{Proto: types.TCP, IP: gwIP, Port: 8080}},
			want: true,
		},
		{
			name: "nil container IP matches any",
			reqs: []types.PortBinding{{Proto: types.TCP, Port: 8080, HostPort: 8080}},
			want: true,
		},
		{
			name: "container IP differs",
			reqs: []types.PortBinding{{Proto: types.TCP, IP: net.ParseIP("172.18.0.3"), Port: 8080, HostPort: 8080}},
			want: false,
		},
		{
			name: "unspecified host IP in request matches",
			reqs: []types.PortBinding{{Proto: types.TCP, IP: gwIP, Port: 8080, HostPort: 8080, HostIP: net.IPv4zero}},
			want: true,
		},
		{
			name: "specific host IP differs",
			reqs: []types.PortBinding{{Proto: types.TCP, IP: gwIP, Port: 8080, HostPort: 8080, HostIP: net.ParseIP("10.0.0.1")}},
			want: false,
		},
		{
			name: "matches one of several requests",
			reqs: []types.PortBinding{
				{Proto: types.UDP, IP: gwIP, Port: 8080, HostPort: 8080},
				{Proto: types.TCP, IP: gwIP, Port: 8080, HostPort: 8080},
			},
			want: true,
		},
		{
			name: "no requests",
			reqs: nil,
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, portBindingMatchesAny(pb, tc.reqs), tc.want)
		})
	}
}
