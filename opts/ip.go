package opts

import (
	"net"
)

type IpOpt struct {
	*net.IP
}

func NewIpOpt(ref *net.IP, defaultVal string) *IpOpt {
	o := &IpOpt{
		IP: ref,
	}
	o.Set(defaultVal)
	return o
}

func (o *IpOpt) Set(val string) error {
	// FIXME: return a parse error if the value is not a valid IP?
	// We are not changing this now to preserve behavior while refactoring.
	(*o.IP) = net.ParseIP(val)
	return nil
}

func (o *IpOpt) String() string {
	return (*o.IP).String()
}
