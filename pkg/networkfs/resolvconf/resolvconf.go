package resolvconf

import (
	"bytes"
	"io/ioutil"
	"regexp"
	"strings"
)

func Get() ([]byte, error) {
	resolv, err := ioutil.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil, err
	}
	return resolv, nil
}

// getLines parses input into lines and strips away comments.
func getLines(input []byte, commentMarker []byte) [][]byte {
	lines := bytes.Split(input, []byte("\n"))
	var output [][]byte
	for _, currentLine := range lines {
		var commentIndex = bytes.Index(currentLine, commentMarker)
		if commentIndex == -1 {
			output = append(output, currentLine)
		} else {
			output = append(output, currentLine[:commentIndex])
		}
	}
	return output
}

// GetNameservers returns nameservers (if any) listed in /etc/resolv.conf
func GetNameservers(resolvConf []byte) []string {
	nameservers := []string{}
	re := regexp.MustCompile(`^\s*nameserver\s*(([0-9]+\.){3}([0-9]+))\s*$`)
	for _, line := range getLines(resolvConf, []byte("#")) {
		var ns = re.FindSubmatch(line)
		if len(ns) > 0 {
			nameservers = append(nameservers, string(ns[1]))
		}
	}
	return nameservers
}

// GetNameserversAsCIDR returns nameservers (if any) listed in
// /etc/resolv.conf as CIDR blocks (e.g., "1.2.3.4/32")
// This function's output is intended for net.ParseCIDR
func GetNameserversAsCIDR(resolvConf []byte) []string {
	nameservers := []string{}
	for _, nameserver := range GetNameservers(resolvConf) {
		nameservers = append(nameservers, nameserver+"/32")
	}
	return nameservers
}

// GetSearchDomains returns search domains (if any) listed in /etc/resolv.conf
// If more than one search line is encountered, only the contents of the last
// one is returned.
func GetSearchDomains(resolvConf []byte) []string {
	re := regexp.MustCompile(`^\s*search\s*(([^\s]+\s*)*)$`)
	domains := []string{}
	for _, line := range getLines(resolvConf, []byte("#")) {
		match := re.FindSubmatch(line)
		if match == nil {
			continue
		}
		domains = strings.Fields(string(match[1]))
	}
	return domains
}

func Build(path string, dns, dnsSearch []string) error {
	content := bytes.NewBuffer(nil)
	for _, dns := range dns {
		if _, err := content.WriteString("nameserver " + dns + "\n"); err != nil {
			return err
		}
	}
	if len(dnsSearch) > 0 {
		if searchString := strings.Join(dnsSearch, " "); strings.Trim(searchString, " ") != "." {
			if _, err := content.WriteString("search " + searchString + "\n"); err != nil {
				return err
			}
		}
	}

	return ioutil.WriteFile(path, content.Bytes(), 0644)
}
