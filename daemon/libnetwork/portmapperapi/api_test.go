package portmapperapi

import (
	"net"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/types"
	"gotest.tools/v3/assert"
)

func TestPortBindingReqsCompare(t *testing.T) {
	pb := PortBindingReq{
		PortBinding: types.PortBinding{
			Proto:       types.TCP,
			IP:          net.ParseIP("172.17.0.2"),
			Port:        80,
			HostIP:      net.ParseIP("192.168.1.2"),
			HostPort:    8080,
			HostPortEnd: 8080,
		},
	}
	var pbA, pbB PortBindingReq

	assert.Check(t, pb.Compare(pb) == 0) //nolint:gocritic // ignore "dupArg: suspicious method call with the same argument and receiver (gocritic)"

	pbA, pbB = pb, pb
	pbB.Mapper = "routed"
	assert.Check(t, pbA.Compare(pbB) < 0)
	assert.Check(t, pbB.Compare(pbA) > 0)

	pbA, pbB = pb, pb
	pbA.Port = 22
	assert.Check(t, pbA.Compare(pbB) < 0)
	assert.Check(t, pbB.Compare(pbA) > 0)

	pbA, pbB = pb, pb
	pbB.Proto = types.UDP
	assert.Check(t, pbA.Compare(pbB) < 0)
	assert.Check(t, pbB.Compare(pbA) > 0)

	pbA, pbB = pb, pb
	pbA.Port = 22
	pbA.Proto = types.UDP
	assert.Check(t, pbA.Compare(pbB) < 0)
	assert.Check(t, pbB.Compare(pbA) > 0)

	pbA, pbB = pb, pb
	pbB.HostPort = 8081
	assert.Check(t, pbA.Compare(pbB) < 0)
	assert.Check(t, pbB.Compare(pbA) > 0)

	pbA, pbB = pb, pb
	pbB.HostPort, pbB.HostPortEnd = 0, 0
	assert.Check(t, pbA.Compare(pbB) < 0)
	assert.Check(t, pbB.Compare(pbA) > 0)

	pbA, pbB = pb, pb
	pbB.HostPortEnd = 8081
	assert.Check(t, pbA.Compare(pbB) < 0)
	assert.Check(t, pbB.Compare(pbA) > 0)

	pbA, pbB = pb, pb
	pbA.HostPortEnd = 8080
	pbB.HostPortEnd = 8081
	assert.Check(t, pbA.Compare(pbB) < 0)
	assert.Check(t, pbB.Compare(pbA) > 0)
}
