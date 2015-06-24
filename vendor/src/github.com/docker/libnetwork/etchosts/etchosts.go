package etchosts

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
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

// Default hosts config records slice
var defaultContent = []Record{
	{Hosts: "localhost", IP: "127.0.0.1"},
	{Hosts: "localhost ip6-localhost ip6-loopback", IP: "::1"},
	{Hosts: "ip6-localnet", IP: "fe00::0"},
	{Hosts: "ip6-mcastprefix", IP: "ff00::0"},
	{Hosts: "ip6-allnodes", IP: "ff02::1"},
	{Hosts: "ip6-allrouters", IP: "ff02::2"},
}

// Build function
// path is path to host file string required
// IP, hostname, and domainname set main record leave empty for no master record
// extraContent is an array of extra host records.
func Build(path, IP, hostname, domainname string, extraContent []Record) error {
	content := bytes.NewBuffer(nil)
	if IP != "" {
		//set main record
		var mainRec Record
		mainRec.IP = IP
		if domainname != "" {
			mainRec.Hosts = fmt.Sprintf("%s.%s %s", hostname, domainname, hostname)
		} else {
			mainRec.Hosts = hostname
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

	return ioutil.WriteFile(path, content.Bytes(), 0644)
}

// Add adds an arbitrary number of Records to an already existing /etc/hosts file
func Add(path string, recs []Record) error {
	if len(recs) == 0 {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}

	content := bytes.NewBuffer(nil)

	_, err = content.ReadFrom(f)
	if err != nil {
		return err
	}

	for _, r := range recs {
		if _, err := r.WriteTo(content); err != nil {
			return err
		}
	}

	return ioutil.WriteFile(path, content.Bytes(), 0644)
}

// Delete deletes an arbitrary number of Records already existing in /etc/hosts file
func Delete(path string, recs []Record) error {
	if len(recs) == 0 {
		return nil
	}

	old, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	regexpStr := fmt.Sprintf("\\S*\\t%s\\n", regexp.QuoteMeta(recs[0].Hosts))
	for _, r := range recs[1:] {
		regexpStr = regexpStr + "|" + fmt.Sprintf("\\S*\\t%s\\n", regexp.QuoteMeta(r.Hosts))
	}

	var re = regexp.MustCompile(regexpStr)
	return ioutil.WriteFile(path, re.ReplaceAll(old, []byte("")), 0644)
}

// Update all IP addresses where hostname matches.
// path is path to host file
// IP is new IP address
// hostname is hostname to search for to replace IP
func Update(path, IP, hostname string) error {
	old, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	var re = regexp.MustCompile(fmt.Sprintf("(\\S*)(\\t%s)", regexp.QuoteMeta(hostname)))
	return ioutil.WriteFile(path, re.ReplaceAll(old, []byte(IP+"$2")), 0644)
}
