package execdriver

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path"
	"strings"

	"github.com/docker/docker/utils"
	"github.com/docker/libcontainer/netlink"
	"github.com/docker/libcontainer/security/capabilities"
)

// Args provided to the init function for lxc and libvirt drivers
type InitArgs struct {
	User       string
	Gateway    string
	Ip         string
	WorkDir    string
	Privileged bool
	Env        []string
	Args       []string
	Mtu        int
	Console    string
	Pipe       int
	Root       string
	CapAdd     string
	CapDrop    string
}

func TweakCapabilities(basics, adds, drops []string) ([]string, error) {
	var (
		newCaps []string
		allCaps = capabilities.GetAllCapabilities()
	)

	// look for invalid cap in the drop list
	for _, cap := range drops {
		if strings.ToLower(cap) == "all" {
			continue
		}
		if !utils.StringsContainsNoCase(allCaps, cap) {
			return nil, fmt.Errorf("Unknown capability drop: %q", cap)
		}
	}

	// handle --cap-add=all
	if utils.StringsContainsNoCase(adds, "all") {
		basics = capabilities.GetAllCapabilities()
	}

	if !utils.StringsContainsNoCase(drops, "all") {
		for _, cap := range basics {
			// skip `all` aready handled above
			if strings.ToLower(cap) == "all" {
				continue
			}

			// if we don't drop `all`, add back all the non-dropped caps
			if !utils.StringsContainsNoCase(drops, cap) {
				newCaps = append(newCaps, strings.ToUpper(cap))
			}
		}
	}

	for _, cap := range adds {
		// skip `all` aready handled above
		if strings.ToLower(cap) == "all" {
			continue
		}

		if !utils.StringsContainsNoCase(allCaps, cap) {
			return nil, fmt.Errorf("Unknown capability to add: %q", cap)
		}

		// add cap if not already in the list
		if !utils.StringsContainsNoCase(newCaps, cap) {
			newCaps = append(newCaps, strings.ToUpper(cap))
		}
	}

	return newCaps, nil
}

func GenerateEnvConfig(c *Command, root string) error {
	data, err := json.Marshal(c.ProcessConfig.Env)
	if err != nil {
		return err
	}
	p := path.Join(root, "containers", c.ID, "config.env")
	c.Mounts = append(c.Mounts, Mount{
		Source:      p,
		Destination: "/.dockerenv",
		Writable:    false,
		Private:     true,
	})

	return ioutil.WriteFile(p, data, 0600)
}

// Clear environment pollution
func SetupEnv(args *InitArgs) error {
	// Get env
	var env []string
	content, err := ioutil.ReadFile(".dockerenv")
	if err != nil {
		return fmt.Errorf("Unable to load environment variables: %v", err)
	}
	if err := json.Unmarshal(content, &env); err != nil {
		return fmt.Errorf("Unable to unmarshal environment variables: %v", err)
	}
	// Propagate the plugin-specific container env variable
	env = append(env, "container="+os.Getenv("container"))

	args.Env = env

	os.Clearenv()
	for _, kv := range args.Env {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 1 {
			parts = append(parts, "")
		}
		os.Setenv(parts[0], parts[1])
	}

	return nil
}

// Setup networking
func SetupNetworking(args *InitArgs) error {
	if args.Ip != "" {
		// eth0
		iface, err := net.InterfaceByName("eth0")
		if err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
		ip, ipNet, err := net.ParseCIDR(args.Ip)
		if err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
		if err := netlink.NetworkLinkAddIp(iface, ip, ipNet); err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
		if err := netlink.NetworkSetMTU(iface, args.Mtu); err != nil {
			return fmt.Errorf("Unable to set MTU: %v", err)
		}
		if err := netlink.NetworkLinkUp(iface); err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}

		// loopback
		iface, err = net.InterfaceByName("lo")
		if err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
		if err := netlink.NetworkLinkUp(iface); err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
	}
	if args.Gateway != "" {
		gw := net.ParseIP(args.Gateway)
		if gw == nil {
			return fmt.Errorf("Unable to set up networking, %s is not a valid gateway IP", args.Gateway)
		}

		if err := netlink.AddDefaultGw(gw.String(), "eth0"); err != nil {
			return fmt.Errorf("Unable to set up networking: %v", err)
		}
	}

	return nil
}

func (args *InitArgs) GetEnv(key string) string {
	for _, kv := range args.Env {
		parts := strings.SplitN(kv, "=", 2)
		if parts[0] == key && len(parts) == 2 {
			return parts[1]
		}
	}
	return ""
}

type Template interface {
	Execute(wr io.Writer, data interface{}) error
}

func GenerateContainerConfig(t Template, c *Command, apparmor bool, path string) error {
	fo, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fo.Close()

	if err := t.Execute(fo, struct {
		*Command
		AppArmor bool
	}{
		Command:  c,
		AppArmor: apparmor,
	}); err != nil {
		return err
	}
	return nil
}
