package opts

import (
	"net"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestIpOptString(c *check.C) {
	addresses := []string{"", "0.0.0.0"}
	var ip net.IP

	for _, address := range addresses {
		stringAddress := NewIPOpt(&ip, address).String()
		if stringAddress != address {
			c.Fatalf("IpOpt string should be `%s`, not `%s`", address, stringAddress)
		}
	}
}

func (s *DockerSuite) TestNewIpOptInvalidDefaultVal(c *check.C) {
	ip := net.IPv4(127, 0, 0, 1)
	defaultVal := "Not an ip"

	ipOpt := NewIPOpt(&ip, defaultVal)

	expected := "127.0.0.1"
	if ipOpt.String() != expected {
		c.Fatalf("Expected [%v], got [%v]", expected, ipOpt.String())
	}
}

func (s *DockerSuite) TestNewIpOptValidDefaultVal(c *check.C) {
	ip := net.IPv4(127, 0, 0, 1)
	defaultVal := "192.168.1.1"

	ipOpt := NewIPOpt(&ip, defaultVal)

	expected := "192.168.1.1"
	if ipOpt.String() != expected {
		c.Fatalf("Expected [%v], got [%v]", expected, ipOpt.String())
	}
}

func (s *DockerSuite) TestIpOptSetInvalidVal(c *check.C) {
	ip := net.IPv4(127, 0, 0, 1)
	ipOpt := &IPOpt{IP: &ip}

	invalidIP := "invalid ip"
	expectedError := "invalid ip is not an ip address"
	err := ipOpt.Set(invalidIP)
	if err == nil || err.Error() != expectedError {
		c.Fatalf("Expected an Error with [%v], got [%v]", expectedError, err.Error())
	}
}
