package main

import (
	"fmt"
	"net"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/options"
)

func main() {
	log.SetLevel(log.DebugLevel)
	controller, err := libnetwork.New()
	if err != nil {
		log.Fatal(err)
	}

	netType := "null"
	ip, net, _ := net.ParseCIDR("192.168.100.1/24")
	net.IP = ip
	options := options.Generic{"AddressIPv4": net}

	err = controller.ConfigureNetworkDriver(netType, options)
	for i := 0; i < 10; i++ {
		netw, err := controller.NewNetwork(netType, fmt.Sprintf("Gordon-%d", i))
		if err != nil {
			if _, ok := err.(libnetwork.NetworkNameError); !ok {
				log.Fatal(err)
			}
		} else {
			fmt.Println("Network Created Successfully :", netw)
		}
		netw, _ = controller.NetworkByName(fmt.Sprintf("Gordon-%d", i))
		_, err = netw.CreateEndpoint(fmt.Sprintf("Gordon-Ep-%d", i), nil)
		if err != nil {
			log.Fatalf("Error creating endpoint 1 %v", err)
		}

		_, err = netw.CreateEndpoint(fmt.Sprintf("Gordon-Ep2-%d", i), nil)
		if err != nil {
			log.Fatalf("Error creating endpoint 2 %v", err)
		}

		time.Sleep(2 * time.Second)
	}
}
