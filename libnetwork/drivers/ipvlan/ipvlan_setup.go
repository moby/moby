package ipvlan

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/osl"
	"github.com/vishvananda/netlink"
)

// Create the ipvlan netlink interface specifying the source name
func createIPVlan(containerIfName, hostIface, ipVlanMode string) (string, error) {
	defer osl.InitOSContext()()
	// Set the ipvlan mode. Default is L2 mode
	mode, err := setIPVlanMode(ipVlanMode)
	if err != nil {
		return "", fmt.Errorf("unsupported %s ipvlan mode: %v", ipVlanMode, err)
	}
	// Get the link for the master index (Example: the docker host eth iface)
	hostEth, err := netlink.LinkByName(hostIface)
	if err != nil {
		return "", fmt.Errorf("the requested host interface %s was not found on the Docker host", hostIface)
	}
	// Create a ipvlan link
	ipvlan := &netlink.IPVlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:        containerIfName,
			ParentIndex: hostEth.Attrs().Index,
		},
		Mode: mode,
	}
	if err := netlink.LinkAdd(ipvlan); err != nil {
		// verbose but will be an issue if you create a macvlan and ipvlan using the same parent iface
		logrus.Warn("Ensure there are no macvlan networks using the same `-o host_iface` as the ipvlan network.")
		return "", fmt.Errorf("Failed to create ipvlan link: %s with the error: %v", ipvlan.Name, err)
	}
	return ipvlan.Attrs().Name, nil
}

// setIPVlanMode setter for one of the two ipvlan port types
func setIPVlanMode(mode string) (netlink.IPVlanMode, error) {
	switch mode {
	case modeL2:
		return netlink.IPVLAN_MODE_L2, nil
	case modeL3:
		return netlink.IPVLAN_MODE_L3, nil
	default:
		return 0, fmt.Errorf("Unknown ipvlan mode: %s", mode)
	}
}

// validateHostIface check if the specified interface exists in the default namespace
func hostIfaceExists(ifaceStr string) bool {
	_, err := netlink.LinkByName(ifaceStr)
	if err != nil {
		return false
	}
	return true
}

// kernelSupport for the necessary kernel module for the driver type
func kernelSupport(networkTpe string) error {
	// attempt to load the module, silent if successful or already loaded
	exec.Command("modprobe", ipvlanType).Run()
	f, err := os.Open("/proc/modules")
	if err != nil {
		return err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		if strings.Contains(s.Text(), ipvlanType) {
			return nil
		}
	}
	return fmt.Errorf("required kernel module '%s' was not found in /proc/modules, ipvlan requires kernel version >= 3.19", ipvlanType)
}

// createVlanLink parses sub-interfaces and vlan id for creation
func createVlanLink(ifaceName string) error {
	if strings.Contains(ifaceName, ".") {
		parentIface, vidInt, err := parseVlan(ifaceName)
		if err != nil {
			return err
		}
		// VLAN identifier or VID is a 12-bit field specifying the VLAN to which the frame belongs
		if vidInt > 4094 || vidInt < 1 {
			return fmt.Errorf("vlan id must be between 1-4094, received: %d", vidInt)
		}
		// get the parent link to attach a vlan subinterface
		hostIface, err := netlink.LinkByName(parentIface)
		if err != nil {
			return fmt.Errorf("failed to find master interface %s on the Docker host: %v", parentIface, err)
		}
		vlanIface := &netlink.Vlan{
			LinkAttrs: netlink.LinkAttrs{
				Name:        ifaceName,
				ParentIndex: hostIface.Attrs().Index,
			},
			VlanId: vidInt,
		}
		// create the subinterface
		if err := netlink.LinkAdd(vlanIface); err != nil {
			return fmt.Errorf("failed to create %s vlan link: %v", vlanIface.Name, err)
		}
		// Bring the new netlink iface up
		if err := netlink.LinkSetUp(vlanIface); err != nil {
			return fmt.Errorf("failed to enable %s the ipvlan netlink link: %v", vlanIface.Name, err)
		}
		logrus.Debugf("Added a vlan tagged netlink subinterface: %s with a vlan id: %d", ifaceName, vidInt)
		return nil
	}
	return fmt.Errorf("invalid subinterface vlan name %s, example formatting is eth0.10", ifaceName)
}

// verifyVlanDel verifies only sub-interfaces with a vlan id get deleted
func delVlanLink(ifaceName string) error {
	if strings.Contains(ifaceName, ".") {
		_, _, err := parseVlan(ifaceName)
		if err != nil {
			return err
		}
		// delete the vlan subinterface
		vlanIface, err := netlink.LinkByName(ifaceName)
		if err != nil {
			return fmt.Errorf("failed to find interface %s on the Docker host : %v", ifaceName, err)
		}
		// verify a parent interface isn't being deleted
		if vlanIface.Attrs().ParentIndex == 0 {
			return fmt.Errorf("interface %s does not appear to be a slave device: %v", ifaceName, err)
		}
		// delete the ipvlan slave device
		if err := netlink.LinkDel(vlanIface); err != nil {
			return fmt.Errorf("failed to delete  %s link: %v", ifaceName, err)
		}
		logrus.Debugf("Deleted a vlan tagged netlink subinterface: %s", ifaceName)
	}
	// if the subinterface doesn't parse to iface.vlan_id leave the interface in
	// place since it could be a user specified name not created by the driver.
	return nil
}

// parseVlan parses and verifies a slave interface name: -o host_iface=eth0.10
func parseVlan(ifaceName string) (string, int, error) {
	// parse -o host_iface=eth0.10
	splitIface := strings.Split(ifaceName, ".")
	if len(splitIface) != 2 {
		return "", 0, fmt.Errorf("required interface name format is: name.vlan_id, ex. eth0.10 for vlan 10, instead received %s", ifaceName)
	}
	parentIface, vidStr := splitIface[0], splitIface[1]
	// validate type and convert vlan id to int
	vidInt, err := strconv.Atoi(vidStr)
	if err != nil {
		return "", 0, fmt.Errorf("unable to parse a valid vlan id from: %s (ex. eth0.10 for vlan 10)", vidStr)
	}
	// Check if the interface exists
	if ok := hostIfaceExists(parentIface); !ok {
		return "", 0, fmt.Errorf("-o host_iface parent interface does was not found on the host: %s", parentIface)
	}
	return parentIface, vidInt, nil
}
