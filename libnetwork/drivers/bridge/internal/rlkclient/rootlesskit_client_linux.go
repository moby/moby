// RootlessKit integration - if required by RootlessKit's port driver, let it know
// about port mappings as they're added and removed.
//
// This is based on / copied from rootlesskit-docker-proxy, which was previously
// installed as a proxy for docker-proxy:
// https://github.com/rootless-containers/rootlesskit/blob/4fb2e2cb80bf13eb28b7f2a4317b63406b89ad32/cmd/rootlesskit-docker-proxy/main.go

package rlkclient

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rootless-containers/rootlesskit/v2/pkg/api/client"
	"github.com/rootless-containers/rootlesskit/v2/pkg/port"
)

type PortDriverClient struct {
	client         client.Client
	portDriverName string
	protos         map[string]struct{}
	childIP        netip.Addr
}

func NewPortDriverClient(ctx context.Context) (*PortDriverClient, error) {
	stateDir := os.Getenv("ROOTLESSKIT_STATE_DIR")
	if stateDir == "" {
		return nil, errors.New("$ROOTLESSKIT_STATE_DIR needs to be set")
	}
	socketPath := filepath.Join(stateDir, "api.sock")
	c, err := client.New(socketPath)
	if err != nil {
		return nil, fmt.Errorf("error while connecting to RootlessKit API socket: %w", err)
	}

	info, err := c.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to call info API, probably RootlessKit binary is too old (needs to be v0.14.0 or later): %w", err)
	}

	// info.PortDriver is currently nil for "none" and "implicit", but this may change in future
	if info.PortDriver == nil || info.PortDriver.Driver == "none" || info.PortDriver.Driver == "implicit" {
		return nil, nil
	}

	pdc := &PortDriverClient{
		client:         c,
		portDriverName: info.PortDriver.Driver,
	}

	if info.PortDriver.DisallowLoopbackChildIP {
		// i.e., port-driver="slirp4netns"
		if info.NetworkDriver.ChildIP == nil {
			return nil, fmt.Errorf("RootlessKit port driver (%q) does not allow loopback child IP, but network driver (%q) has no non-loopback IP",
				info.PortDriver.Driver, info.NetworkDriver.Driver)
		}
		childIP, ok := netip.AddrFromSlice(info.NetworkDriver.ChildIP)
		if !ok {
			return nil, fmt.Errorf("unable to use child IP %s from network driver (%q)",
				info.NetworkDriver.ChildIP, info.NetworkDriver.Driver)
		}
		pdc.childIP = childIP
	}

	pdc.protos = make(map[string]struct{}, len(info.PortDriver.Protos))
	for _, p := range info.PortDriver.Protos {
		pdc.protos[p] = struct{}{}
	}

	return pdc, nil
}

// ChildHostIP returns the address that must be used in the child network
// namespace in place of hostIP, a host IP address. In particular, port
// mappings from host IP addresses, and DNAT rules, must use this child
// address in place of the real host address.
func (c *PortDriverClient) ChildHostIP(hostIP netip.Addr) netip.Addr {
	if c == nil {
		return hostIP
	}
	if c.childIP.IsValid() {
		return c.childIP
	}
	if hostIP.Is6() {
		return netip.IPv6Loopback()
	}
	return netip.MustParseAddr("127.0.0.1")
}

// AddPort makes a request to RootlessKit asking it to set up a port
// mapping between a host IP address and a child host IP address.
func (c *PortDriverClient) AddPort(
	ctx context.Context,
	proto string,
	hostIP netip.Addr,
	childIP netip.Addr,
	hostPort int,
) (func() error, error) {
	if c == nil {
		return func() error { return nil }, nil
	}
	// proto is like "tcp", but we need to convert it to "tcp4" or "tcp6" explicitly
	// for libnetwork >= 20201216
	//
	// See https://github.com/moby/libnetwork/pull/2604/files#diff-8fa48beed55dd033bf8e4f8c40b31cf69d0b2cc5d4bb53cde8594670ea6c938aR20
	// See also https://github.com/rootless-containers/rootlesskit/issues/231
	apiProto := proto
	if !strings.HasSuffix(apiProto, "4") && !strings.HasSuffix(apiProto, "6") {
		if hostIP.Is6() {
			apiProto += "6"
		} else {
			apiProto += "4"
		}
	}

	if _, ok := c.protos[apiProto]; !ok {
		// This happens when apiProto="tcp6", portDriverName="slirp4netns",
		// because "slirp4netns" port driver does not support listening on IPv6 yet.
		//
		// Note that "slirp4netns" port driver is not used by default,
		// even when network driver is set to "slirp4netns".
		//
		// Most users are using "builtin" port driver and will not see this warning.
		return nil, fmt.Errorf("protocol %q is not supported by the RootlessKit port driver %q, discarding request for %q",
			proto,
			c.portDriverName,
			net.JoinHostPort(hostIP.String(), strconv.Itoa(hostPort)))
	}

	pm := c.client.PortManager()
	p := port.Spec{
		Proto:      apiProto,
		ParentIP:   hostIP.String(),
		ParentPort: hostPort,
		ChildIP:    childIP.String(),
		ChildPort:  hostPort,
	}
	st, err := pm.AddPort(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("error while calling RootlessKit PortManager.AddPort(): %w", err)
	}
	deferFunc := func() error {
		if dErr := pm.RemovePort(context.WithoutCancel(ctx), st.ID); dErr != nil {
			return fmt.Errorf("error while calling RootlessKit PortManager.RemovePort(): %w", dErr)
		}
		return nil
	}
	return deferFunc, nil
}
