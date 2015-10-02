package overlay

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/ipallocator"
	"github.com/docker/libnetwork/osl"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
)

type networkTable map[string]*network

type network struct {
	id          string
	vni         uint32
	dbIndex     uint64
	dbExists    bool
	sbox        osl.Sandbox
	endpoints   endpointTable
	ipAllocator *ipallocator.IPAllocator
	gw          net.IP
	vxlanName   string
	driver      *driver
	joinCnt     int
	once        *sync.Once
	initEpoch   int
	initErr     error
	sync.Mutex
}

func (d *driver) CreateNetwork(id string, option map[string]interface{}) error {
	if id == "" {
		return fmt.Errorf("invalid network id")
	}

	if err := d.configure(); err != nil {
		return err
	}

	n := &network{
		id:        id,
		driver:    d,
		endpoints: endpointTable{},
		once:      &sync.Once{},
	}

	n.gw = bridgeIP.IP

	d.addNetwork(n)

	if err := n.obtainVxlanID(); err != nil {
		return err
	}

	return nil
}

func (d *driver) DeleteNetwork(nid string) error {
	if nid == "" {
		return fmt.Errorf("invalid network id")
	}

	n := d.network(nid)
	if n == nil {
		return fmt.Errorf("could not find network with id %s", nid)
	}

	d.deleteNetwork(nid)

	return n.releaseVxlanID()
}

func (n *network) joinSandbox() error {
	n.Lock()
	if n.joinCnt != 0 {
		n.joinCnt++
		n.Unlock()
		return nil
	}
	n.Unlock()

	// If there is a race between two go routines here only one will win
	// the other will wait.
	n.once.Do(func() {
		// save the error status of initSandbox in n.initErr so that
		// all the racing go routines are able to know the status.
		n.initErr = n.initSandbox()
	})

	// Increment joinCnt in all the goroutines only when the one time initSandbox
	// was a success.
	n.Lock()
	if n.initErr == nil {
		n.joinCnt++
	}
	err := n.initErr
	n.Unlock()

	return err
}

func (n *network) leaveSandbox() {
	n.Lock()
	n.joinCnt--
	if n.joinCnt != 0 {
		n.Unlock()
		return
	}

	// We are about to destroy sandbox since the container is leaving the network
	// Reinitialize the once variable so that we will be able to trigger one time
	// sandbox initialization(again) when another container joins subsequently.
	n.once = &sync.Once{}
	n.Unlock()

	n.destroySandbox()
}

func (n *network) destroySandbox() {
	sbox := n.sandbox()
	if sbox != nil {
		for _, iface := range sbox.Info().Interfaces() {
			iface.Remove()
		}

		if err := deleteVxlan(n.vxlanName); err != nil {
			logrus.Warnf("could not cleanup sandbox properly: %v", err)
		}

		sbox.Destroy()
	}
}

func (n *network) initSandbox() error {
	n.Lock()
	n.initEpoch++
	n.Unlock()

	sbox, err := osl.NewSandbox(
		osl.GenerateKey(fmt.Sprintf("%d-", n.initEpoch)+n.id), true)
	if err != nil {
		return fmt.Errorf("could not create network sandbox: %v", err)
	}

	// Add a bridge inside the namespace
	if err := sbox.AddInterface("bridge1", "br",
		sbox.InterfaceOptions().Address(bridgeIP),
		sbox.InterfaceOptions().Bridge(true)); err != nil {
		return fmt.Errorf("could not create bridge inside the network sandbox: %v", err)
	}

	n.setSandbox(sbox)

	var nlSock *nl.NetlinkSocket
	sbox.InvokeFunc(func() {
		nlSock, err = nl.Subscribe(syscall.NETLINK_ROUTE, syscall.RTNLGRP_NEIGH)
		if err != nil {
			err = fmt.Errorf("failed to subscribe to neighbor group netlink messages")
		}
	})

	go n.watchMiss(nlSock)
	return n.initVxlan()
}

func (n *network) initVxlan() error {
	var vxlanName string
	n.Lock()
	sbox := n.sbox
	n.Unlock()

	vxlanName, err := createVxlan(n.vxlanID())
	if err != nil {
		return err
	}

	if err = sbox.AddInterface(vxlanName, "vxlan",
		sbox.InterfaceOptions().Master("bridge1")); err != nil {
		return fmt.Errorf("could not add vxlan interface inside the network sandbox: %v", err)
	}

	n.vxlanName = vxlanName
	n.driver.peerDbUpdateSandbox(n.id)
	return nil
}

func (n *network) watchMiss(nlSock *nl.NetlinkSocket) {
	for {
		msgs, err := nlSock.Receive()
		if err != nil {
			logrus.Errorf("Failed to receive from netlink: %v ", err)
			continue
		}

		for _, msg := range msgs {
			if msg.Header.Type != syscall.RTM_GETNEIGH && msg.Header.Type != syscall.RTM_NEWNEIGH {
				continue
			}

			neigh, err := netlink.NeighDeserialize(msg.Data)
			if err != nil {
				logrus.Errorf("Failed to deserialize netlink ndmsg: %v", err)
				continue
			}

			if neigh.IP.To16() != nil {
				continue
			}

			if neigh.State&(netlink.NUD_STALE|netlink.NUD_INCOMPLETE) == 0 {
				continue
			}

			mac, vtep, err := n.driver.resolvePeer(n.id, neigh.IP)
			if err != nil {
				logrus.Errorf("could not resolve peer %q: %v", neigh.IP, err)
				continue
			}

			if err := n.driver.peerAdd(n.id, "dummy", neigh.IP, mac, vtep, true); err != nil {
				logrus.Errorf("could not add neighbor entry for missed peer: %v", err)
			}
		}
	}
}

func (d *driver) addNetwork(n *network) {
	d.Lock()
	d.networks[n.id] = n
	d.Unlock()
}

func (d *driver) deleteNetwork(nid string) {
	d.Lock()
	delete(d.networks, nid)
	d.Unlock()
}

func (d *driver) network(nid string) *network {
	d.Lock()
	defer d.Unlock()

	return d.networks[nid]
}

func (n *network) sandbox() osl.Sandbox {
	n.Lock()
	defer n.Unlock()

	return n.sbox
}

func (n *network) setSandbox(sbox osl.Sandbox) {
	n.Lock()
	n.sbox = sbox
	n.Unlock()
}

func (n *network) vxlanID() uint32 {
	n.Lock()
	defer n.Unlock()

	return n.vni
}

func (n *network) setVxlanID(vni uint32) {
	n.Lock()
	n.vni = vni
	n.Unlock()
}

func (n *network) Key() []string {
	return []string{"overlay", "network", n.id}
}

func (n *network) KeyPrefix() []string {
	return []string{"overlay", "network"}
}

func (n *network) Value() []byte {
	b, err := json.Marshal(n.vxlanID())
	if err != nil {
		return []byte{}
	}

	return b
}

func (n *network) Index() uint64 {
	return n.dbIndex
}

func (n *network) SetIndex(index uint64) {
	n.dbIndex = index
	n.dbExists = true
}

func (n *network) Exists() bool {
	return n.dbExists
}

func (n *network) Skip() bool {
	return false
}

func (n *network) SetValue(value []byte) error {
	var vni uint32
	err := json.Unmarshal(value, &vni)
	if err == nil {
		n.setVxlanID(vni)
	}
	return err
}

func (n *network) DataScope() datastore.DataScope {
	return datastore.GlobalScope
}

func (n *network) writeToStore() error {
	return n.driver.store.PutObjectAtomic(n)
}

func (n *network) releaseVxlanID() error {
	if n.driver.store == nil {
		return fmt.Errorf("no datastore configured. cannot release vxlan id")
	}

	if n.vxlanID() == 0 {
		return nil
	}

	if err := n.driver.store.DeleteObjectAtomic(n); err != nil {
		if err == datastore.ErrKeyModified || err == datastore.ErrKeyNotFound {
			// In both the above cases we can safely assume that the key has been removed by some other
			// instance and so simply get out of here
			return nil
		}

		return fmt.Errorf("failed to delete network to vxlan id map: %v", err)
	}

	n.driver.vxlanIdm.Release(n.vxlanID())
	n.setVxlanID(0)
	return nil
}

func (n *network) obtainVxlanID() error {
	if n.driver.store == nil {
		return fmt.Errorf("no datastore configured. cannot obtain vxlan id")
	}

	for {
		var vxlanID uint32
		if err := n.driver.store.GetObject(datastore.Key(n.Key()...), n); err != nil {
			if err == datastore.ErrKeyNotFound {
				vxlanID, err = n.driver.vxlanIdm.GetID()
				if err != nil {
					return fmt.Errorf("failed to allocate vxlan id: %v", err)
				}

				n.setVxlanID(vxlanID)
				if err := n.writeToStore(); err != nil {
					n.driver.vxlanIdm.Release(n.vxlanID())
					n.setVxlanID(0)
					if err == datastore.ErrKeyModified {
						continue
					}
					return fmt.Errorf("failed to update data store with vxlan id: %v", err)
				}
				return nil
			}
			return fmt.Errorf("failed to obtain vxlan id from data store: %v", err)
		}
		return nil
	}
}
