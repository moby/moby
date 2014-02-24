package network

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/utils"
	"log"
)

type Veth struct {
}

func (v *Veth) Create(n *libcontainer.Network, nspid int) (libcontainer.Context, error) {
	log.Printf("creating veth network")
	var (
		bridge string
		prefix string
		exists bool
	)
	if bridge, exists = n.Context["bridge"]; !exists {
		return nil, fmt.Errorf("bridge does not exist in network context")
	}
	if prefix, exists = n.Context["prefix"]; !exists {
		return nil, fmt.Errorf("veth prefix does not exist in network context")
	}
	name1, name2, err := createVethPair(prefix)
	if err != nil {
		return nil, err
	}
	context := libcontainer.Context{
		"vethHost":  name1,
		"vethChild": name2,
	}
	log.Printf("veth pair created %s <> %s", name1, name2)
	if err := SetInterfaceMaster(name1, bridge); err != nil {
		return context, err
	}
	if err := SetMtu(name1, n.Mtu); err != nil {
		return context, err
	}
	if err := InterfaceUp(name1); err != nil {
		return context, err
	}
	log.Printf("setting %s inside %d namespace", name2, nspid)
	if err := SetInterfaceInNamespacePid(name2, nspid); err != nil {
		return context, err
	}
	return context, nil
}

func (v *Veth) Initialize(config *libcontainer.Network, context libcontainer.Context) error {
	var (
		vethChild string
		exists    bool
	)
	if vethChild, exists = context["vethChild"]; !exists {
		return fmt.Errorf("vethChild does not exist in network context")
	}
	if err := InterfaceDown(vethChild); err != nil {
		return fmt.Errorf("interface down %s %s", vethChild, err)
	}
	if err := ChangeInterfaceName(vethChild, "eth0"); err != nil {
		return fmt.Errorf("change %s to eth0 %s", vethChild, err)
	}
	if err := SetInterfaceIp("eth0", config.Address); err != nil {
		return fmt.Errorf("set eth0 ip %s", err)
	}
	if err := SetMtu("eth0", config.Mtu); err != nil {
		return fmt.Errorf("set eth0 mtu to %d %s", config.Mtu, err)
	}
	if err := InterfaceUp("eth0"); err != nil {
		return fmt.Errorf("eth0 up %s", err)
	}
	if err := SetMtu("lo", config.Mtu); err != nil {
		return fmt.Errorf("set lo mtu to %d %s", config.Mtu, err)
	}
	if err := InterfaceUp("lo"); err != nil {
		return fmt.Errorf("lo up %s", err)
	}
	if config.Gateway != "" {
		if err := SetDefaultGateway(config.Gateway); err != nil {
			return fmt.Errorf("set gateway to %s %s", config.Gateway, err)
		}
	}
	return nil
}

// createVethPair will automatically generage two random names for
// the veth pair and ensure that they have been created
func createVethPair(prefix string) (name1 string, name2 string, err error) {
	name1, err = utils.GenerateRandomName(prefix, 4)
	if err != nil {
		return
	}
	name2, err = utils.GenerateRandomName(prefix, 4)
	if err != nil {
		return
	}
	if err = CreateVethPair(name1, name2); err != nil {
		return
	}
	return
}
