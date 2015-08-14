// +build linux

package proxy

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/parsers"
	"net/rpc"
	"strings"
)

func init() {
	graphdriver.Register("proxy", Init)
}

type Driver struct {
	client *rpc.Client
	home   string
}

func Init(home string, input_options []string) (graphdriver.Driver, error) {
	var options []string = make([]string, 0)
	var protoAddrParts []string = make([]string, 2)
	var graphDriver string

	foundProxyServer := false
	foundGraphDriver := false
	for _, option := range input_options {
		key, val, err := parsers.ParseKeyValueOpt(option)
		if err != nil {
			options = append(options, option)
			continue
		}
		key = strings.ToLower(key)
		switch key {
		case "proxyserver":
			protoAddrParts = strings.SplitN(val, "://", 2)
			foundProxyServer = true
		case "graphdriver":
			graphDriver = val
			foundGraphDriver = true
		default:
			options = append(options, option)
		}
	}

	if !foundProxyServer {
		return nil, fmt.Errorf("proxyserver not specified")
	}

	if !foundGraphDriver {
		logrus.Infof("Init: graphdriver not specified, using devicemapper")
		graphDriver = "devicemapper"
	}

	client, err := rpc.DialHTTP(protoAddrParts[0], protoAddrParts[1])
	if err != nil {
		logrus.Errorf("dialing: %s", err)
		return nil, err
	}

	args := &graphdriver.InitArgs{graphDriver, home, options}
	var reply graphdriver.InitReply

	err = client.Call("ProxyAPI.Init", &args, &reply)
	if err != nil {
		logrus.Errorf("ProxyAPI.Init: %s", err)
		return nil, err
	}

	d := &Driver{
		client: client,
		home:   home,
	}

	return graphdriver.NaiveDiffDriver(d), nil
}

func (d *Driver) String() string {
	return "proxy"
}

func (d *Driver) Status() [][2]string {
	args := &graphdriver.StatusArgs{}
	var reply graphdriver.StatusReply

	err := d.client.Call("ProxyAPI.Status", &args, &reply)
	if err != nil {
		logrus.Errorf("ProxyAPI.Status: %s", err)
		return nil
	}
	return reply.Status
}

func (d *Driver) Cleanup() error {
	args := &graphdriver.CleanupArgs{}
	var reply graphdriver.CleanupReply

	err := d.client.Call("ProxyAPI.Cleanup", &args, &reply)
	if err != nil {
		logrus.Errorf("ProxyAPI.Cleanup: %s", err)
	}
	return err
}

func (d *Driver) Create(id, parent string) error {
	args := &graphdriver.CreateArgs{id, parent}
	var reply graphdriver.CreateReply

	err := d.client.Call("ProxyAPI.Create", &args, &reply)
	if err != nil {
		logrus.Errorf("ProxyAPI.Create: %s", err)
	}
	return err
}

func (d *Driver) Remove(id string) error {
	args := &graphdriver.RemoveArgs{id}
	var reply graphdriver.RemoveReply

	err := d.client.Call("ProxyAPI.Remove", &args, &reply)
	if err != nil {
		logrus.Errorf("ProxyAPI.Remove: %s", err)
	}
	return err
}

func (d *Driver) Get(id, mountLabel string) (string, error) {
	args := &graphdriver.GetArgs{id, mountLabel}
	var reply graphdriver.GetReply

	err := d.client.Call("ProxyAPI.Get", &args, &reply)
	if err != nil {
		logrus.Errorf("ProxyAPI.Get: %s", err)
	}
	return reply.Dir, err
}

func (d *Driver) Put(id string) error {
	args := &graphdriver.PutArgs{id}
	var reply graphdriver.PutReply

	err := d.client.Call("ProxyAPI.Put", &args, &reply)
	if err != nil {
		logrus.Errorf("ProxyAPI.Put: %s", err)
	}
	return err
}

func (d *Driver) Exists(id string) bool {
	args := &graphdriver.ExistsArgs{id}
	var reply graphdriver.ExistsReply

	err := d.client.Call("ProxyAPI.Exists", &args, &reply)
	if err != nil {
		logrus.Errorf("ProxyAPI.Exists: %s", err)
	}
	return reply.Exists
}

func (d *Driver) GetMetadata(id string) (map[string]string, error) {
	args := &graphdriver.GetMetadataArgs{id}
	var reply graphdriver.GetMetadataReply

	err := d.client.Call("ProxyAPI.GetMetadata", &args, &reply)
	if err != nil {
		logrus.Errorf("ProxyAPI.GetMetadata: %s", err)
	}
	return reply.MInfo, err
}
