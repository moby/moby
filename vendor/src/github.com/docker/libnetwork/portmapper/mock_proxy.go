package portmapper

import "net"

func newMockProxyCommand(proto string, hostIP net.IP, hostPort int, containerIP net.IP, containerPort int) userlandProxy {
	return &mockProxyCommand{}
}

type mockProxyCommand struct {
}

func (p *mockProxyCommand) Start() error {
	return nil
}

func (p *mockProxyCommand) Stop() error {
	return nil
}
