package main

import (
	"fmt"
	"log"
	"net"

	"github.com/docker/libnetwork"
	_ "github.com/docker/libnetwork/drivers/bridge"
)

func main() {
	ip, net, _ := net.ParseCIDR("192.168.100.1/24")
	net.IP = ip

	options := libnetwork.DriverParams{"AddressIPv4": net}
	netw, err := libnetwork.NewNetwork("simplebridge", "dummy", options)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Network=%#v\n", netw)
}
