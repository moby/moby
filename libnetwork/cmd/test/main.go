package main

import (
	"fmt"
	"log"
	"net"

	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/pkg/options"
)

func main() {
	ip, net, _ := net.ParseCIDR("192.168.100.1/24")
	net.IP = ip

	options := options.Generic{"AddressIPv4": net}
	controller := libnetwork.New()
	netType := "bridge"
	err := controller.ConfigureNetworkDriver(netType, options)
	netw, err := controller.NewNetwork(netType, "dummy", "")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Network=%#v\n", netw)
}
