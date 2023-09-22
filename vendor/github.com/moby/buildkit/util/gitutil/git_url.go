package gitutil

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/moby/buildkit/util/sshutil"
	"github.com/pkg/errors"
)

const (
	HTTPProtocol  string = "http"
	HTTPSProtocol string = "https"
	SSHProtocol   string = "ssh"
	GitProtocol   string = "git"
)

var (
	ErrUnknownProtocol = errors.New("unknown protocol")
	ErrInvalidProtocol = errors.New("invalid protocol")
)

var supportedProtos = map[string]struct{}{
	HTTPProtocol:  {},
	HTTPSProtocol: {},
	SSHProtocol:   {},
	GitProtocol:   {},
}

var protoRegexp = regexp.MustCompile(`^[a-zA-Z0-9]+://`)

// ParseURL parses a git URL and returns a parsed URL object.
//
// ParseURL understands implicit ssh URLs such as "git@host:repo", and
// returns the same response as if the URL were "ssh://git@host/repo".
func ParseURL(remote string) (*url.URL, error) {
	if proto := protoRegexp.FindString(remote); proto != "" {
		proto = strings.ToLower(strings.TrimSuffix(proto, "://"))
		if _, ok := supportedProtos[proto]; !ok {
			return nil, errors.Wrap(ErrInvalidProtocol, proto)
		}

		return url.Parse(remote)
	}

	if sshutil.IsImplicitSSHTransport(remote) {
		remote, fragment, _ := strings.Cut(remote, "#")
		remote, path, _ := strings.Cut(remote, ":")
		user, host, _ := strings.Cut(remote, "@")
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		return &url.URL{
			Scheme:   SSHProtocol,
			User:     url.User(user),
			Host:     host,
			Path:     path,
			Fragment: fragment,
		}, nil
	}

	return nil, ErrUnknownProtocol
}

// SplitGitFragments splits a git URL fragment into its respective git
// reference and subdirectory components.
func SplitGitFragment(fragment string) (ref string, subdir string) {
	ref, subdir, _ = strings.Cut(fragment, ":")
	return ref, subdir
}
