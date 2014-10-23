package etchosts

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"regexp"
)

var defaultContent = map[string]string{
	"localhost":                            "127.0.0.1",
	"localhost ip6-localhost ip6-loopback": "::1",
	"ip6-localnet":                         "fe00::0",
	"ip6-mcastprefix":                      "ff00::0",
	"ip6-allnodes":                         "ff02::1",
	"ip6-allrouters":                       "ff02::2",
}

func Build(path, IP, hostname, domainname string, extraContent *map[string]string) error {
	content := bytes.NewBuffer(nil)
	if IP != "" {
		if domainname != "" {
			content.WriteString(fmt.Sprintf("%s\t%s.%s %s\n", IP, hostname, domainname, hostname))
		} else {
			content.WriteString(fmt.Sprintf("%s\t%s\n", IP, hostname))
		}
	}

	for hosts, ip := range defaultContent {
		if _, err := content.WriteString(fmt.Sprintf("%s\t%s\n", ip, hosts)); err != nil {
			return err
		}
	}

	if extraContent != nil {
		for hosts, ip := range *extraContent {
			if _, err := content.WriteString(fmt.Sprintf("%s\t%s\n", ip, hosts)); err != nil {
				return err
			}
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
