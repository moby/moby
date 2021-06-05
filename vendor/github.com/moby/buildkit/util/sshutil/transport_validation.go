package sshutil

import (
	"regexp"
)

var gitSSHRegex = regexp.MustCompile("^[a-zA-Z0-9-_]+@[a-zA-Z0-9-.]+:.*$")

func IsImplicitSSHTransport(s string) bool {
	return gitSSHRegex.MatchString(s)
}
