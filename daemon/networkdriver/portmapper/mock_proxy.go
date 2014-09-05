package portmapper

import "net"

func NewMockProxyCommand(proto string, hostIP net.IP, hostPort int, containerIP net.IP, containerPort int) UserlandProxy {
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
