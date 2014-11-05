package etchosts

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"regexp"
)

type Record struct {
	Hosts string
	IP    string
}

func (r Record) WriteTo(w io.Writer) (int64, error) {
	n, err := fmt.Fprintf(w, "%s\t%s\n", r.IP, r.Hosts)
	return int64(n), err
}

var defaultContent = []Record{
	{Hosts: "localhost", IP: "127.0.0.1"},
	{Hosts: "localhost ip6-localhost ip6-loopback", IP: "::1"},
	{Hosts: "ip6-localnet", IP: "fe00::0"},
	{Hosts: "ip6-mcastprefix", IP: "ff00::0"},
	{Hosts: "ip6-allnodes", IP: "ff02::1"},
	{Hosts: "ip6-allrouters", IP: "ff02::2"},
}

func Build(path, IP, hostname, domainname string, extraContent []Record) error {
	content := bytes.NewBuffer(nil)
	if IP != "" {
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

	for _, r := range defaultContent {
		if _, err := r.WriteTo(content); err != nil {
			return err
		}
	}

	for _, r := range extraContent {
		if _, err := r.WriteTo(content); err != nil {
			return err
		}
	}

	return ioutil.WriteFile(path, content.Bytes(), 0644)
}

func Update(path, IP, hostname string) error {
	old, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	var re = regexp.MustCompile(fmt.Sprintf("(\\S*)(\\t%s)", regexp.QuoteMeta(hostname)))
	return ioutil.WriteFile(path, re.ReplaceAll(old, []byte(IP+"$2")), 0644)
}
