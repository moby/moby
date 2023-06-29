package dbserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/libnetwork/cmd/networkdb-test/dummyclient"
	"github.com/docker/docker/libnetwork/diagnostic"
	"github.com/docker/docker/libnetwork/networkdb"
)

var (
	nDB    *networkdb.NetworkDB
	server *diagnostic.Server
	ipAddr string
)

var testerPaths2Func = map[string]diagnostic.HTTPHandlerFunc{
	"/myip": ipaddress,
}

func ipaddress(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "%s\n", ipAddr)
}

// Server starts the server
func Server(args []string) {
	log.G(context.TODO()).Infof("[SERVER] Starting with arguments %v", args)
	if len(args) < 1 {
		log.G(context.TODO()).Fatal("Port number is a mandatory argument, aborting...")
	}
	port, _ := strconv.Atoi(args[0])
	var localNodeName string
	var ok bool
	if localNodeName, ok = os.LookupEnv("TASK_ID"); !ok {
		log.G(context.TODO()).Fatal("TASK_ID environment variable not set, aborting...")
	}
	log.G(context.TODO()).Infof("[SERVER] Starting node %s on port %d", localNodeName, port)

	ip, err := getIPInterface("eth0")
	if err != nil {
		log.G(context.TODO()).Errorf("%s There was a problem with the IP %s\n", localNodeName, err)
		return
	}
	ipAddr = ip
	log.G(context.TODO()).Infof("%s uses IP %s\n", localNodeName, ipAddr)

	server = diagnostic.New()
	server.Init()
	conf := networkdb.DefaultConfig()
	conf.Hostname = localNodeName
	conf.AdvertiseAddr = ipAddr
	conf.BindAddr = ipAddr
	nDB, err = networkdb.New(conf)
	if err != nil {
		log.G(context.TODO()).Infof("%s error in the DB init %s\n", localNodeName, err)
		return
	}

	// Register network db handlers
	server.RegisterHandler(nDB, networkdb.NetDbPaths2Func)
	server.RegisterHandler(nil, testerPaths2Func)
	server.RegisterHandler(nDB, dummyclient.DummyClientPaths2Func)
	server.EnableDiagnostic("", port)
	// block here
	select {}
}

func getIPInterface(name string) (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Name != name {
			continue // not the name specified
		}

		if iface.Flags&net.FlagUp == 0 {
			return "", errors.New("Interfaces is down")
		}

		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue
			}
			return ip.String(), nil
		}
		return "", errors.New("Interfaces does not have a valid IPv4")
	}
	return "", errors.New("Interface not found")
}
