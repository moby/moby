package sshutil

import (
	"fmt"
	"net/url"
	"regexp"

	"github.com/pkg/errors"
)

var gitSSHRegex = regexp.MustCompile(`^([a-zA-Z0-9-_]+)@([a-zA-Z0-9-.]+):(.*?)(?:\?(.*?))?(?:#(.*))?$`)

func IsImplicitSSHTransport(s string) bool {
	return gitSSHRegex.MatchString(s)
}

type SCPStyleURL struct {
	User *url.Userinfo
	Host string

	Path     string
	Query    url.Values
	Fragment string
}

func ParseSCPStyleURL(raw string) (*SCPStyleURL, error) {
	matches := gitSSHRegex.FindStringSubmatch(raw)
	if matches == nil {
		return nil, errors.New("invalid scp-style url")
	}

	rawQuery := matches[4]
	vals := url.Values{}
	if rawQuery != "" {
		var err error
		vals, err = url.ParseQuery(rawQuery)
		if err != nil {
			return nil, errors.Wrap(err, "invalid query in scp-style url")
		}
	}

	return &SCPStyleURL{
		User:     url.User(matches[1]),
		Host:     matches[2],
		Path:     matches[3],
		Query:    vals,
		Fragment: matches[5],
	}, nil
}

func (u *SCPStyleURL) String() string {
	s := fmt.Sprintf("%s@%s:%s", u.User.String(), u.Host, u.Path)

	if len(u.Query) > 0 {
		s += "?" + u.Query.Encode()
	}
	if u.Fragment != "" {
		s += "#" + u.Fragment
	}
	return s
}
