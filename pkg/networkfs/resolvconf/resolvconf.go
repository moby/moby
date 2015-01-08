package resolvconf

import (
	"bytes"
	"io/ioutil"
	"regexp"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/utils"
)

var (
	defaultDns      = []string{"8.8.8.8", "8.8.4.4"}
	localHostRegexp = regexp.MustCompile(`(?m)^nameserver 127[^\n]+\n*`)
	nsRegexp        = regexp.MustCompile(`^\s*nameserver\s*(([0-9]+\.){3}([0-9]+))\s*$`)
	searchRegexp    = regexp.MustCompile(`^\s*search\s*(([^\s]+\s*)*)$`)
)

var lastModified struct {
	sync.Mutex
	sha256   string
	contents []byte
}

func Get() ([]byte, error) {
	resolv, err := ioutil.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil, err
	}
	return resolv, nil
}

// Retrieves the host /etc/resolv.conf file, checks against the last hash
// and, if modified since last check, returns the bytes and new hash.
// This feature is used by the resolv.conf updater for containers
func GetIfChanged() ([]byte, string, error) {
	lastModified.Lock()
	defer lastModified.Unlock()

	resolv, err := ioutil.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil, "", err
	}
	newHash, err := utils.HashData(bytes.NewReader(resolv))
	if err != nil {
		return nil, "", err
	}
	if lastModified.sha256 != newHash {
		lastModified.sha256 = newHash
		lastModified.contents = resolv
		return resolv, newHash, nil
	}
	// nothing changed, so return no data
	return nil, "", nil
}

// retrieve the last used contents and hash of the host resolv.conf
// Used by containers updating on restart
func GetLastModified() ([]byte, string) {
	lastModified.Lock()
	defer lastModified.Unlock()

	return lastModified.contents, lastModified.sha256
}

// RemoveReplaceLocalDns looks for localhost (127.*) entries in the provided
// resolv.conf, removing local nameserver entries, and, if the resulting
// cleaned config has no defined nameservers left, adds default DNS entries
// It also returns a boolean to notify the caller if changes were made at all
func RemoveReplaceLocalDns(resolvConf []byte) ([]byte, bool) {
	changed := false
	cleanedResolvConf := localHostRegexp.ReplaceAll(resolvConf, []byte{})
	// if the resulting resolvConf is empty, use defaultDns
	if !bytes.Contains(cleanedResolvConf, []byte("nameserver")) {
		log.Infof("No non-localhost DNS nameservers are left in resolv.conf. Using default external servers : %v", defaultDns)
		cleanedResolvConf = append(cleanedResolvConf, []byte("\nnameserver "+strings.Join(defaultDns, "\nnameserver "))...)
	}
	if !bytes.Equal(resolvConf, cleanedResolvConf) {
		changed = true
	}
	return cleanedResolvConf, changed
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
	for _, line := range getLines(resolvConf, []byte("#")) {
		var ns = nsRegexp.FindSubmatch(line)
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
	domains := []string{}
	for _, line := range getLines(resolvConf, []byte("#")) {
		match := searchRegexp.FindSubmatch(line)
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
