package main

import (
	"fmt"
	"net"
	"os"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/options"
)

func main() {
	log.SetLevel(log.DebugLevel)
	os.Setenv("LIBNETWORK_CFG", "libnetwork.toml")
	controller, err := libnetwork.New("libnetwork.toml")
	if err != nil {
		log.Fatal(err)
	}

	netType := "null"
	ip, net, _ := net.ParseCIDR("192.168.100.1/24")
	net.IP = ip
	options := options.Generic{"AddressIPv4": net}

	err = controller.ConfigureNetworkDriver(netType, options)
	for i := 0; i < 100; i++ {
		netw, err := controller.NewNetwork(netType, fmt.Sprintf("Gordon-%d", i))
		if err != nil {
			if _, ok := err.(libnetwork.NetworkNameError); !ok {
				log.Fatal(err)
			}
		} else {
			fmt.Println("Network Created Successfully :", netw)
		}
		time.Sleep(10 * time.Second)
	}
}
