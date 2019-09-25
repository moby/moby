// +build linux

package ipvs

import (
	"net"
	"testing"
	"time"

	"github.com/docker/libnetwork/testutils"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

var (
	schedMethods = []string{
		RoundRobin,
		LeastConnection,
		DestinationHashing,
		SourceHashing,
		WeightedLeastConnection,
		WeightedRoundRobin,
	}

	protocols = []string{
		"TCP",
		"UDP",
		"FWM",
	}

	fwdMethods = []uint32{
		ConnectionFlagMasq,
		ConnectionFlagTunnel,
		ConnectionFlagDirectRoute,
	}

	fwdMethodStrings = []string{
		"Masq",
		"Tunnel",
		"Route",
	}
)

func lookupFwMethod(fwMethod uint32) string {

	switch fwMethod {
	case ConnectionFlagMasq:
		return fwdMethodStrings[0]
	case ConnectionFlagTunnel:
		return fwdMethodStrings[1]
	case ConnectionFlagDirectRoute:
		return fwdMethodStrings[2]
	}
	return ""
}

func checkDestination(t *testing.T, i *Handle, s *Service, d *Destination, checkPresent bool) {
	var dstFound bool

	dstArray, err := i.GetDestinations(s)
	assert.NilError(t, err)

	for _, dst := range dstArray {
		if dst.Address.Equal(d.Address) && dst.Port == d.Port && lookupFwMethod(dst.ConnectionFlags) == lookupFwMethod(d.ConnectionFlags) {
			dstFound = true
			break
		}
	}

	switch checkPresent {
	case true: //The test expects the service to be present
		if !dstFound {

			t.Fatalf("Did not find the service %s in ipvs output", d.Address.String())
		}
	case false: //The test expects that the service should not be present
		if dstFound {
			t.Fatalf("Did not find the destination %s fwdMethod %s in ipvs output", d.Address.String(), lookupFwMethod(d.ConnectionFlags))
		}
	}

}

func checkService(t *testing.T, i *Handle, s *Service, checkPresent bool) {

	svcArray, err := i.GetServices()
	assert.NilError(t, err)

	var svcFound bool

	for _, svc := range svcArray {

		if svc.Protocol == s.Protocol && svc.Address.String() == s.Address.String() && svc.Port == s.Port {
			svcFound = true
			break
		}
	}

	switch checkPresent {
	case true: //The test expects the service to be present
		if !svcFound {

			t.Fatalf("Did not find the service %s in ipvs output", s.Address.String())
		}
	case false: //The test expects that the service should not be present
		if svcFound {
			t.Fatalf("Did not expect the service %s in ipvs output", s.Address.String())
		}
	}

}

func TestGetFamily(t *testing.T) {
	if testutils.RunningOnCircleCI() {
		t.Skip("Skipping as not supported on CIRCLE CI kernel")
	}

	id, err := getIPVSFamily()
	assert.NilError(t, err)
	assert.Check(t, 0 != id)
}

func TestService(t *testing.T) {
	if testutils.RunningOnCircleCI() {
		t.Skip("Skipping as not supported on CIRCLE CI kernel")
	}

	defer testutils.SetupTestOSContext(t)()

	i, err := New("")
	assert.NilError(t, err)

	for _, protocol := range protocols {
		for _, schedMethod := range schedMethods {
			testDatas := []struct {
				AddressFamily uint16
				IP            string
				Netmask       uint32
			}{
				{
					AddressFamily: nl.FAMILY_V4,
					IP:            "1.2.3.4",
					Netmask:       0xFFFFFFFF,
				}, {
					AddressFamily: nl.FAMILY_V6,
					IP:            "2001:db8:3c4d:15::1a00",
					Netmask:       128,
				},
			}
			for _, td := range testDatas {
				s := Service{
					AddressFamily: td.AddressFamily,
					SchedName:     schedMethod,
				}

				switch protocol {
				case "FWM":
					s.FWMark = 1234
					s.Netmask = td.Netmask
				case "TCP":
					s.Protocol = unix.IPPROTO_TCP
					s.Port = 80
					s.Address = net.ParseIP(td.IP)
					s.Netmask = td.Netmask
				case "UDP":
					s.Protocol = unix.IPPROTO_UDP
					s.Port = 53
					s.Address = net.ParseIP(td.IP)
					s.Netmask = td.Netmask
				}

				err := i.NewService(&s)
				assert.NilError(t, err)
				checkService(t, i, &s, true)
				for _, updateSchedMethod := range schedMethods {
					if updateSchedMethod == schedMethod {
						continue
					}

					s.SchedName = updateSchedMethod
					err = i.UpdateService(&s)
					assert.NilError(t, err)
					checkService(t, i, &s, true)

					scopy, err := i.GetService(&s)
					assert.NilError(t, err)
					assert.Check(t, is.Equal((*scopy).Address.String(), s.Address.String()))
					assert.Check(t, is.Equal((*scopy).Port, s.Port))
					assert.Check(t, is.Equal((*scopy).Protocol, s.Protocol))
				}

				err = i.DelService(&s)
				assert.NilError(t, err)
				checkService(t, i, &s, false)
			}
		}
	}

	svcs := []Service{
		{
			AddressFamily: nl.FAMILY_V4,
			SchedName:     RoundRobin,
			Protocol:      unix.IPPROTO_TCP,
			Port:          80,
			Address:       net.ParseIP("10.20.30.40"),
			Netmask:       0xFFFFFFFF,
		},
		{
			AddressFamily: nl.FAMILY_V4,
			SchedName:     LeastConnection,
			Protocol:      unix.IPPROTO_UDP,
			Port:          8080,
			Address:       net.ParseIP("10.20.30.41"),
			Netmask:       0xFFFFFFFF,
		},
	}
	// Create services for testing flush
	for _, svc := range svcs {
		if !i.IsServicePresent(&svc) {
			err = i.NewService(&svc)
			assert.NilError(t, err)
			checkService(t, i, &svc, true)
		} else {
			t.Errorf("svc: %v exists", svc)
		}
	}
	err = i.Flush()
	assert.NilError(t, err)
	got, err := i.GetServices()
	assert.NilError(t, err)
	if len(got) != 0 {
		t.Errorf("Unexpected services after flush")
	}
}

func createDummyInterface(t *testing.T) {
	if testutils.RunningOnCircleCI() {
		t.Skip("Skipping as not supported on CIRCLE CI kernel")
	}

	dummy := &netlink.Dummy{
		LinkAttrs: netlink.LinkAttrs{
			Name: "dummy",
		},
	}

	err := netlink.LinkAdd(dummy)
	assert.NilError(t, err)

	dummyLink, err := netlink.LinkByName("dummy")
	assert.NilError(t, err)

	ip, ipNet, err := net.ParseCIDR("10.1.1.1/24")
	assert.NilError(t, err)

	ipNet.IP = ip

	ipAddr := &netlink.Addr{IPNet: ipNet, Label: ""}
	err = netlink.AddrAdd(dummyLink, ipAddr)
	assert.NilError(t, err)
}

func TestDestination(t *testing.T) {
	defer testutils.SetupTestOSContext(t)()

	createDummyInterface(t)
	i, err := New("")
	assert.NilError(t, err)

	for _, protocol := range protocols {
		testDatas := []struct {
			AddressFamily uint16
			IP            string
			Netmask       uint32
			Destinations  []string
		}{
			{
				AddressFamily: nl.FAMILY_V4,
				IP:            "1.2.3.4",
				Netmask:       0xFFFFFFFF,
				Destinations:  []string{"10.1.1.2", "10.1.1.3", "10.1.1.4"},
			}, {
				AddressFamily: nl.FAMILY_V6,
				IP:            "2001:db8:3c4d:15::1a00",
				Netmask:       128,
				Destinations:  []string{"2001:db8:3c4d:15::1a2b", "2001:db8:3c4d:15::1a2c", "2001:db8:3c4d:15::1a2d"},
			},
		}
		for _, td := range testDatas {
			s := Service{
				AddressFamily: td.AddressFamily,
				SchedName:     RoundRobin,
			}

			switch protocol {
			case "FWM":
				s.FWMark = 1234
				s.Netmask = td.Netmask
			case "TCP":
				s.Protocol = unix.IPPROTO_TCP
				s.Port = 80
				s.Address = net.ParseIP(td.IP)
				s.Netmask = td.Netmask
			case "UDP":
				s.Protocol = unix.IPPROTO_UDP
				s.Port = 53
				s.Address = net.ParseIP(td.IP)
				s.Netmask = td.Netmask
			}

			err := i.NewService(&s)
			assert.NilError(t, err)
			checkService(t, i, &s, true)

			s.SchedName = ""
			for _, fwdMethod := range fwdMethods {
				destinations := make([]Destination, 0)
				for _, ip := range td.Destinations {
					d := Destination{
						AddressFamily:   td.AddressFamily,
						Address:         net.ParseIP(ip),
						Port:            5000,
						Weight:          1,
						ConnectionFlags: fwdMethod,
					}
					destinations = append(destinations, d)
					err := i.NewDestination(&s, &d)
					assert.NilError(t, err)
					checkDestination(t, i, &s, &d, true)
				}

				for _, updateFwdMethod := range fwdMethods {
					if updateFwdMethod == fwdMethod {
						continue
					}
					for _, d := range destinations {
						d.ConnectionFlags = updateFwdMethod
						err = i.UpdateDestination(&s, &d)
						assert.NilError(t, err)
						checkDestination(t, i, &s, &d, true)
					}
				}
				for _, d := range destinations {
					err = i.DelDestination(&s, &d)
					assert.NilError(t, err)
					checkDestination(t, i, &s, &d, false)
				}
			}

		}
	}
}

func TestTimeouts(t *testing.T) {
	if testutils.RunningOnCircleCI() {
		t.Skip("Skipping as not supported on CIRCLE CI kernel")
	}
	defer testutils.SetupTestOSContext(t)()

	i, err := New("")
	assert.NilError(t, err)

	_, err = i.GetConfig()
	assert.NilError(t, err)

	cfg := Config{66 * time.Second, 66 * time.Second, 66 * time.Second}
	err = i.SetConfig(&cfg)
	assert.NilError(t, err)

	c2, err := i.GetConfig()
	assert.NilError(t, err)
	assert.DeepEqual(t, cfg, *c2)

	//  A timeout value 0 means that the current timeout value of the corresponding entry is preserved
	cfg = Config{77 * time.Second, 0 * time.Second, 77 * time.Second}
	err = i.SetConfig(&cfg)
	assert.NilError(t, err)

	c3, err := i.GetConfig()
	assert.NilError(t, err)
	assert.DeepEqual(t, *c3, Config{77 * time.Second, 66 * time.Second, 77 * time.Second})
}
