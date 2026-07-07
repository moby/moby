//go:build linux

package libnetwork

import (
	"fmt"
	"io"
	"net"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestCloseIngressPortsProxyAfterBindFailure(t *testing.T) {
	ingressMu.Lock()
	defer ingressMu.Unlock()

	origTbl := ingressProxyTbl
	t.Cleanup(func() {
		ingressProxyTbl = origTbl
	})
	ingressProxyTbl = make(map[string]io.Closer)

	blocker, err := net.ListenUDP("udp", &net.UDPAddr{Port: 0})
	assert.NilError(t, err)
	defer blocker.Close()

	port := uint32(blocker.LocalAddr().(*net.UDPAddr).Port)
	ingressPort := &PortConfig{
		Protocol:      ProtocolUDP,
		PublishedPort: port,
	}

	plumbIngressPortsProxy([]*PortConfig{ingressPort})

	portSpec := fmt.Sprintf("%d/%s", ingressPort.PublishedPort, strings.ToLower(ingressPort.Protocol.String()))
	if _, ok := ingressProxyTbl[portSpec]; ok {
		t.Fatal("bind failure must not store a listener in ingressProxyTbl")
	}

	closeIngressPortsProxy([]*PortConfig{ingressPort})
}
