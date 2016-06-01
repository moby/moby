package libnetwork

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/go-events"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/discoverapi"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/networkdb"
)

type agent struct {
	networkDB         *networkdb.NetworkDB
	bindAddr          string
	epTblCancel       func()
	driverCancelFuncs map[string][]func()
}

func getBindAddr(ifaceName string) (string, error) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return "", fmt.Errorf("failed to find interface %s: %v", ifaceName, err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return "", fmt.Errorf("failed to get interface addresses: %v", err)
	}

	for _, a := range addrs {
		addr, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		addrIP := addr.IP

		if addrIP.IsLinkLocalUnicast() {
			continue
		}

		return addrIP.String(), nil
	}

	return "", fmt.Errorf("failed to get bind address")
}

func resolveAddr(addrOrInterface string) (string, error) {
	// Try and see if this is a valid IP address
	if net.ParseIP(addrOrInterface) != nil {
		return addrOrInterface, nil
	}

	// If not a valid IP address, it should be a valid interface
	return getBindAddr(addrOrInterface)
}

func (c *controller) agentInit(bindAddrOrInterface string) error {
	if !c.cfg.Daemon.IsAgent {
		return nil
	}

	bindAddr, err := resolveAddr(bindAddrOrInterface)
	if err != nil {
		return err
	}

	hostname, _ := os.Hostname()
	nDB, err := networkdb.New(&networkdb.Config{
		BindAddr: bindAddr,
		NodeName: hostname,
	})

	if err != nil {
		return err
	}

	ch, cancel := nDB.Watch("endpoint_table", "", "")

	c.agent = &agent{
		networkDB:         nDB,
		bindAddr:          bindAddr,
		epTblCancel:       cancel,
		driverCancelFuncs: make(map[string][]func()),
	}

	go c.handleTableEvents(ch, c.handleEpTableEvent)
	return nil
}

func (c *controller) agentJoin(remotes []string) error {
	if c.agent == nil {
		return nil
	}

	return c.agent.networkDB.Join(remotes)
}

func (c *controller) agentDriverNotify(d driverapi.Driver) {
	if c.agent == nil {
		return
	}

	d.DiscoverNew(discoverapi.NodeDiscovery, discoverapi.NodeDiscoveryData{
		Address: c.agent.bindAddr,
		Self:    true,
	})
}

func (c *controller) agentClose() {
	if c.agent == nil {
		return
	}

	for _, cancelFuncs := range c.agent.driverCancelFuncs {
		for _, cancel := range cancelFuncs {
			cancel()
		}
	}
	c.agent.epTblCancel()

	c.agent.networkDB.Close()
}

func (n *network) isClusterEligible() bool {
	if n.driverScope() != datastore.GlobalScope {
		return false
	}

	c := n.getController()
	if c.agent == nil {
		return false
	}

	return true
}

func (n *network) joinCluster() error {
	if !n.isClusterEligible() {
		return nil
	}

	c := n.getController()
	return c.agent.networkDB.JoinNetwork(n.ID())
}

func (n *network) leaveCluster() error {
	if !n.isClusterEligible() {
		return nil
	}

	c := n.getController()
	return c.agent.networkDB.LeaveNetwork(n.ID())
}

func (ep *endpoint) addToCluster() error {
	n := ep.getNetwork()
	if !n.isClusterEligible() {
		return nil
	}

	c := n.getController()
	if !ep.isAnonymous() && ep.Iface().Address() != nil {
		if err := c.addServiceBinding(ep.svcName, ep.svcID, n.ID(), ep.ID(), ep.Iface().Address().IP); err != nil {
			return err
		}

		if err := c.agent.networkDB.CreateEntry("endpoint_table", n.ID(), ep.ID(), []byte(fmt.Sprintf("%s,%s,%s,%s", ep.Name(), ep.svcName,
			ep.svcID, ep.Iface().Address().IP))); err != nil {
			return err
		}
	}

	for _, te := range ep.joinInfo.driverTableEntries {
		if err := c.agent.networkDB.CreateEntry(te.tableName, n.ID(), te.key, te.value); err != nil {
			return err
		}
	}

	return nil
}

func (ep *endpoint) deleteFromCluster() error {
	n := ep.getNetwork()
	if !n.isClusterEligible() {
		return nil
	}

	c := n.getController()
	if !ep.isAnonymous() {
		if ep.Iface().Address() != nil {
			if err := c.rmServiceBinding(ep.svcName, ep.svcID, n.ID(), ep.ID(), ep.Iface().Address().IP); err != nil {
				return err
			}
		}

		if err := c.agent.networkDB.DeleteEntry("endpoint_table", n.ID(), ep.ID()); err != nil {
			return err
		}
	}

	if ep.joinInfo == nil {
		return nil
	}

	for _, te := range ep.joinInfo.driverTableEntries {
		if err := c.agent.networkDB.DeleteEntry(te.tableName, n.ID(), te.key); err != nil {
			return err
		}
	}

	return nil
}

func (n *network) addDriverWatches() {
	if !n.isClusterEligible() {
		return
	}

	c := n.getController()
	for _, tableName := range n.driverTables {
		ch, cancel := c.agent.networkDB.Watch(tableName, n.ID(), "")
		c.Lock()
		c.agent.driverCancelFuncs[n.ID()] = append(c.agent.driverCancelFuncs[n.ID()], cancel)
		c.Unlock()

		go c.handleTableEvents(ch, n.handleDriverTableEvent)
		d, err := n.driver(false)
		if err != nil {
			logrus.Errorf("Could not resolve driver %s while walking driver tabl: %v", n.networkType, err)
			return
		}

		c.agent.networkDB.WalkTable(tableName, func(nid, key string, value []byte) bool {
			d.EventNotify(driverapi.Create, n.ID(), tableName, key, value)
			return false
		})
	}
}

func (n *network) cancelDriverWatches() {
	if !n.isClusterEligible() {
		return
	}

	c := n.getController()
	c.Lock()
	cancelFuncs := c.agent.driverCancelFuncs[n.ID()]
	delete(c.agent.driverCancelFuncs, n.ID())
	c.Unlock()

	for _, cancel := range cancelFuncs {
		cancel()
	}
}

func (c *controller) handleTableEvents(ch chan events.Event, fn func(events.Event)) {
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return
			}

			fn(ev)
		}
	}
}

func (n *network) handleDriverTableEvent(ev events.Event) {
	d, err := n.driver(false)
	if err != nil {
		logrus.Errorf("Could not resolve driver %s while handling driver table event: %v", n.networkType, err)
		return
	}

	var (
		etype driverapi.EventType
		tname string
		key   string
		value []byte
	)

	switch event := ev.(type) {
	case networkdb.CreateEvent:
		tname = event.Table
		key = event.Key
		value = event.Value
		etype = driverapi.Create
	case networkdb.DeleteEvent:
		tname = event.Table
		key = event.Key
		value = event.Value
		etype = driverapi.Delete
	case networkdb.UpdateEvent:
		tname = event.Table
		key = event.Key
		value = event.Value
		etype = driverapi.Delete
	}

	d.EventNotify(etype, n.ID(), tname, key, value)
}

func (c *controller) handleEpTableEvent(ev events.Event) {
	var (
		nid   string
		eid   string
		value string
		isAdd bool
	)

	switch event := ev.(type) {
	case networkdb.CreateEvent:
		nid = event.NetworkID
		eid = event.Key
		value = string(event.Value)
		isAdd = true
	case networkdb.DeleteEvent:
		nid = event.NetworkID
		eid = event.Key
		value = string(event.Value)
	case networkdb.UpdateEvent:
		logrus.Errorf("Unexpected update service table event = %#v", event)
	}

	nw, err := c.NetworkByID(nid)
	if err != nil {
		logrus.Errorf("Could not find network %s while handling service table event: %v", nid, err)
		return
	}
	n := nw.(*network)

	vals := strings.Split(value, ",")
	if len(vals) < 4 {
		logrus.Errorf("Incorrect service table value = %s", value)
		return
	}

	name := vals[0]
	svcName := vals[1]
	svcID := vals[2]
	ip := net.ParseIP(vals[3])

	if name == "" || ip == nil {
		logrus.Errorf("Invalid endpoint name/ip received while handling service table event %s", value)
		return
	}

	if isAdd {
		if err := c.addServiceBinding(svcName, svcID, nid, eid, ip); err != nil {
			logrus.Errorf("Failed adding service binding for value %s: %v", value, err)
			return
		}

		n.addSvcRecords(name, ip, nil, true)
	} else {
		if err := c.rmServiceBinding(svcName, svcID, nid, eid, ip); err != nil {
			logrus.Errorf("Failed adding service binding for value %s: %v", value, err)
			return
		}

		n.deleteSvcRecords(name, ip, nil, true)
	}
}
