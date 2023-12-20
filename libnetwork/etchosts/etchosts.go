package etchosts

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/docker/docker/libnetwork/internal/filelease"
)

// Record Structure for a single host record
type Record struct {
	Hosts string
	IP    string
}

// WriteTo writes record to file and returns bytes written or error
func (r Record) WriteTo(w io.Writer) (int64, error) {
	n, err := fmt.Fprintf(w, "%s\t%s\n", r.IP, r.Hosts)
	return int64(n), err
}

var (
	// Default hosts config records slice
	defaultContent = []Record{
		{Hosts: "localhost", IP: "127.0.0.1"},
	}
	defaultIPv6Content = []Record{
		{Hosts: "localhost ip6-localhost ip6-loopback", IP: "::1"},
		{Hosts: "ip6-localnet", IP: "fe00::0"},
		{Hosts: "ip6-mcastprefix", IP: "ff00::0"},
		{Hosts: "ip6-allnodes", IP: "ff02::1"},
		{Hosts: "ip6-allrouters", IP: "ff02::2"},
	}

	// A cache of path level locks for synchronizing /etc/hosts
	// updates on a file level
	pathMap = make(map[string]*sync.Mutex)

	// A package level mutex to synchronize the cache itself
	pathMutex sync.Mutex
)

func pathLock(path string) func() {
	pathMutex.Lock()
	defer pathMutex.Unlock()

	pl, ok := pathMap[path]
	if !ok {
		pl = &sync.Mutex{}
		pathMap[path] = pl
	}

	pl.Lock()
	return func() {
		pl.Unlock()
	}
}

// Drop drops the path string from the path cache
func Drop(path string) {
	pathMutex.Lock()
	defer pathMutex.Unlock()

	delete(pathMap, path)
}

// Build function
// path is path to host file string required
// IP, hostname, and domainname set main record leave empty for no master record
// extraContent is an array of extra host records.
func Build(path, IP, hostname, domainname string, extraContent []Record) error {
	defer pathLock(path)()

	content := bytes.NewBuffer(nil)
	if IP != "" {
		// set main record
		var mainRec Record
		mainRec.IP = IP
		// User might have provided a FQDN in hostname or split it across hostname
		// and domainname.  We want the FQDN and the bare hostname.
		fqdn := hostname
		if domainname != "" {
			fqdn += "." + domainname
		}
		mainRec.Hosts = fqdn

		if hostName, _, ok := strings.Cut(fqdn, "."); ok {
			mainRec.Hosts += " " + hostName
		}
		if _, err := mainRec.WriteTo(content); err != nil {
			return err
		}
	}
	// Write defaultContent slice to buffer
	for _, r := range defaultContent {
		if _, err := r.WriteTo(content); err != nil {
			return err
		}
	}
	// Write extra content from function arguments
	for _, r := range extraContent {
		if _, err := r.WriteTo(content); err != nil {
			return err
		}
	}

	return os.WriteFile(path, content.Bytes(), 0o644)
}

// Add or remove the built-in IPv6 hosts entries.
func UpdateIPv6Builtins(path string, add bool) error {
	if add {
		// Keep the built-in entries at top of the file by prepending.
		// TODO(robmry) - placing the IPv6 builtins before IPv4's localhost is unusual.
		//   The file will have:
		//     <IPv6 built-ins>
		//     <IPv4 built-in>
		//     <Other hosts>
		//   Once the IPv4 entry is removable in an IPv6-only network, it will be easier to
		//   construct the file in the usual order for containers initially attached to
		//   dual-stack networks.
		return Add(path, defaultIPv6Content, false)
	}
	return Delete(path, defaultIPv6Content)
}

// Add adds an arbitrary number of Records to an already existing /etc/hosts file.
// Records are added to the end of the file if 'append==true', else at the start.
func Add(path string, recs []Record, append bool) error {
	defer pathLock(path)()

	if len(recs) == 0 {
		return nil
	}

	f, err := filelease.OpenFile(path, false)
	if err != nil {
		return err
	}
	defer f.Close()

	b, err := mergeRecords(f, recs, append)
	if err != nil {
		return err
	}

	return f.WriteFile(b)
}

func mergeRecords(f io.Reader, recs []Record, append bool) ([]byte, error) {
	content := bytes.NewBuffer(nil)

	if append {
		if _, err := content.ReadFrom(f); err != nil {
			return nil, err
		}
	}

	for _, r := range recs {
		if _, err := r.WriteTo(content); err != nil {
			return nil, err
		}
	}

	if !append {
		if _, err := content.ReadFrom(f); err != nil {
			return nil, err
		}
	}

	return content.Bytes(), nil
}

// Delete deletes an arbitrary number of Records already existing in /etc/hosts file
//
// TODO(robmry) - should this function match on addresses as well as names?
//
//	When a container has been connected to more than one network, per-network
//	duplicate records are added to '/etc/hosts' mapping that network's addresses to
//	the container's hostname or short-id. (Perhaps that is a problem in itself,
//	but only the first entry is normally used.) Then, when the container is
//	disconnected from one of those networks, its addresses/names are sent here as
//	'recs'. But, because the removal is only by-name, records created for all
//	other networks are also removed, leaving '/etc/hosts' with no entries for the
//	container's hostname/short-id. On the default bridge network, with no DNS, the
//	container's own hostname can then no longer be resolved. (At present 'recs' do
//	not include IPv6, because Network.getSvcRecords() only looks at IPv4.)
func Delete(path string, recs []Record) error {
	defer pathLock(path)()

	if len(recs) == 0 {
		return nil
	}
	f, err := filelease.OpenFile(path, false)
	if err != nil {
		return err
	}
	defer f.Close()

	var buf bytes.Buffer

	s := bufio.NewScanner(f)
	eol := []byte{'\n'}
loop:
	for s.Scan() {
		b := s.Bytes()
		if len(b) == 0 {
			continue
		}

		if b[0] == '#' {
			buf.Write(b)
			buf.Write(eol)
			continue
		}
		for _, r := range recs {
			if bytes.HasSuffix(b, []byte("\t"+r.Hosts)) {
				continue loop
			}
		}
		buf.Write(b)
		buf.Write(eol)
	}
	if err := s.Err(); err != nil {
		return err
	}
	return f.WriteFile(buf.Bytes())
}

// Update all IP addresses where hostname matches.
// path is path to host file
// IP is new IP address
// hostname is hostname to search for to replace IP
func Update(path, IP, hostname string) error {
	defer pathLock(path)()

	f, err := filelease.OpenFile(path, false)
	if err != nil {
		return err
	}
	defer f.Close()

	old := bytes.NewBuffer(nil)
	if _, err := old.ReadFrom(f); err != nil {
		return err
	}

	re := regexp.MustCompile(fmt.Sprintf("(\\S*)(\\t%s)(\\s|\\.)", regexp.QuoteMeta(hostname)))
	return f.WriteFile(re.ReplaceAll(old.Bytes(), []byte(IP+"$2"+"$3")))
}
