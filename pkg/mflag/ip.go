package mflag

import (
	"fmt"
	"net"
)

func IPVar(value *net.IP, names []string, defaultValue, usage string) {
	ip := (*IP)(value)
	ip.Set(defaultValue)
	Var(ip, names, usage)
}

type IP net.IP

func (ip *IP) Set(val string) error {
	(*ip) = IP(net.ParseIP(val))
	if (*ip) == nil {
		return fmt.Errorf("%s is not an ip address", val)
	}
	return nil
}

func (ip *IP) String() string {
	return (*ip).String()
}
