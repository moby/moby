package docker

import (
	"fmt"
	"testing"
)

var (
	privatePortSortFunc By = func(port1, port2 *APIPort) bool {
		return port1.PrivatePort < port2.PrivatePort
	}
	publicPortSortFunc By = func(port1, port2 *APIPort) bool {
		return port1.PublicPort < port2.PublicPort
	}
)

func createTestAPIPorts() []APIPort {
	return []APIPort{
		APIPort{
			PublicPort:  2000,
			PrivatePort: 3000,
			Type:        "tcp",
            IP:          "1.2.3.4",
		},
		APIPort{
			PublicPort:  1,
			PrivatePort: 2,
			Type:        "tcp",
            IP:          "1.2.3.4",
		},
		APIPort{
			PublicPort:  2,
			PrivatePort: 1,
			Type:        "tcp",
            IP:          "1.2.3.4",
		},
		APIPort{
			PublicPort:  3,
			PrivatePort: 7,
			Type:        "tcp",
            IP:          "1.2.3.4",
		},
	}
}

func TestAPIPortSliceSortMethodBy(t *testing.T) {
	ports := createTestAPIPorts()

	privatePort := func(port1, port2 *APIPort) bool {
		return port1.PrivatePort < port2.PrivatePort
	}

	By(privatePort).Sort(ports)

	if ports[0].PrivatePort != 1 {
		t.Error(printSortErrorMessage("By", "Private", 1, ports[0].PrivatePort))
	}

	publicPort := func(port1, port2 *APIPort) bool {
		return port1.PublicPort < port2.PublicPort
	}

	By(publicPort).Sort(ports)

	if ports[0].PublicPort != 1 {
		t.Error(printSortErrorMessage("By", "Public", 1, ports[0].PublicPort))
	}
}

func TestAPIPortSliceSortMethodByWithEmptySlice(t *testing.T) {
	// No assertions because test's point is to make sure error is not thrown
	ports := []APIPort{}
	By(publicPortSortFunc).Sort(ports)
	By(privatePortSortFunc).Sort(ports)
}

func TestAPIPortSliceSortByPrivatePort(t *testing.T) {
	ports := createTestAPIPorts()

	apiPortSlice := APIPortSlice(ports)
	ports = apiPortSlice.sortByPrivatePort()

	if ports[0].PrivatePort != 1 {
		t.Error(printSortErrorMessage("TestAPIPortSliceSortByPrivatePort", "Private",
			1, ports[0].PrivatePort))
	}
}

func TestAPIPortSliceSortByPublicPort(t *testing.T) {
	ports := createTestAPIPorts()

	apiPortSlice := APIPortSlice(ports)
	ports = apiPortSlice.sortByPublicPort()

	if ports[0].PublicPort != 1 {
		t.Error(printSortErrorMessage("TestAPIPortSliceSortByPublicPort", "Public",
			1, ports[0].PublicPort))
	}
}

func printSortErrorMessage(methodName, APIPortType string, expected, actual int64) string {
	return fmt.Sprintf("%s function failed. "+
		"The expected first %s port is %d and the actual was %d",
		methodName, APIPortType, expected, actual)
}

func TestSortUniquePorts(t *testing.T) {
	ports := []Port{
		Port("6379/tcp"),
		Port("22/tcp"),
	}

	sortPorts(ports, func(ip, jp Port) bool {
		return ip.Int() < jp.Int() || (ip.Int() == jp.Int() && ip.Proto() == "tcp")
	})

	first := ports[0]
	if fmt.Sprint(first) != "22/tcp" {
		t.Log(fmt.Sprint(first))
		t.Fail()
	}
}

func TestSortSamePortWithDifferentProto(t *testing.T) {
	ports := []Port{
		Port("8888/tcp"),
		Port("8888/udp"),
		Port("6379/tcp"),
		Port("6379/udp"),
	}

	sortPorts(ports, func(ip, jp Port) bool {
		return ip.Int() < jp.Int() || (ip.Int() == jp.Int() && ip.Proto() == "tcp")
	})

	first := ports[0]
	if fmt.Sprint(first) != "6379/tcp" {
		t.Fail()
	}
}

func TestDisplayablePortsOutputString(t *testing.T) {
    expected := "1.2.3.4:2->1/tcp, 1.2.3.4:1->2/tcp, 1.2.3.4:3->7/tcp, 1.2.3.4:2000->3000/tcp"
	actual := displayablePorts(createTestAPIPorts())
	if expected != actual {
		t.Error(fmt.Sprintf("TestDisplayablePortsOutputString fail. "+
			"Expected was %s and Actual was %s", expected, actual))
	}
}
