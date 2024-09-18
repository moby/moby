package sshutil

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
)

var gitSSHRegex = regexp.MustCompile("^([a-zA-Z0-9-_]+)@([a-zA-Z0-9-.]+):(.*?)(?:#(.*))?$")

func IsImplicitSSHTransport(s string) bool {
	return gitSSHRegex.MatchString(s)
}

type SCPStyleURL struct {
	User *url.Userinfo
	Host string

	Path     string
	Fragment string
}

func ParseSCPStyleURL(raw string) (*SCPStyleURL, error) {
	matches := gitSSHRegex.FindStringSubmatch(raw)
	if matches == nil {
		return nil, errors.New("invalid scp-style url")
	}
	return &SCPStyleURL{
		User:     url.User(matches[1]),
		Host:     matches[2],
		Path:     matches[3],
		Fragment: matches[4],
	}, nil
}

func (url *SCPStyleURL) String() string {
	base := fmt.Sprintf("%s@%s:%s", url.User.String(), url.Host, url.Path)
	if url.Fragment == "" {
		return base
	}
	return base + "#" + url.Fragment
}
