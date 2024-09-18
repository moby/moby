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

// URL is a custom URL type that points to a remote Git repository.
//
// URLs can be parsed from both standard URLs (e.g.
// "https://github.com/moby/buildkit.git"), as well as SCP-like URLs (e.g.
// "git@github.com:moby/buildkit.git").
//
// See https://git-scm.com/book/en/v2/Git-on-the-Server-The-Protocols
type GitURL struct {
	// Scheme is the protocol over which the git repo can be accessed
	Scheme string

	// Host is the remote host that hosts the git repo
	Host string
	// Path is the path on the host to access the repo
	Path string
	// User is the username/password to access the host
	User *url.Userinfo
	// Fragment can contain additional metadata
	Fragment *GitURLFragment

	// Remote is a valid URL remote to pass into the Git CLI tooling (i.e.
	// without the fragment metadata)
	Remote string
}

// GitURLFragment is the buildkit-specific metadata extracted from the fragment
// of a remote URL.
type GitURLFragment struct {
	// Ref is the git reference
	Ref string
	// Subdir is the sub-directory inside the git repository to use
	Subdir string
}

// splitGitFragment splits a git URL fragment into its respective git
// reference and subdirectory components.
func splitGitFragment(fragment string) *GitURLFragment {
	if fragment == "" {
		return nil
	}
	ref, subdir, _ := strings.Cut(fragment, ":")
	return &GitURLFragment{Ref: ref, Subdir: subdir}
}

// ParseURL parses a BuildKit-style Git URL (that may contain additional
// fragment metadata) and returns a parsed GitURL object.
func ParseURL(remote string) (*GitURL, error) {
	if proto := protoRegexp.FindString(remote); proto != "" {
		proto = strings.ToLower(strings.TrimSuffix(proto, "://"))
		if _, ok := supportedProtos[proto]; !ok {
			return nil, errors.Wrap(ErrInvalidProtocol, proto)
		}
		url, err := url.Parse(remote)
		if err != nil {
			return nil, err
		}
		return fromURL(url), nil
	}

	if url, err := sshutil.ParseSCPStyleURL(remote); err == nil {
		return fromSCPStyleURL(url), nil
	}

	return nil, ErrUnknownProtocol
}

func IsGitTransport(remote string) bool {
	if proto := protoRegexp.FindString(remote); proto != "" {
		proto = strings.ToLower(strings.TrimSuffix(proto, "://"))
		_, ok := supportedProtos[proto]
		return ok
	}
	return sshutil.IsImplicitSSHTransport(remote)
}

func fromURL(url *url.URL) *GitURL {
	withoutFragment := *url
	withoutFragment.Fragment = ""
	return &GitURL{
		Scheme:   url.Scheme,
		User:     url.User,
		Host:     url.Host,
		Path:     url.Path,
		Fragment: splitGitFragment(url.Fragment),
		Remote:   withoutFragment.String(),
	}
}

func fromSCPStyleURL(url *sshutil.SCPStyleURL) *GitURL {
	withoutFragment := *url
	withoutFragment.Fragment = ""
	return &GitURL{
		Scheme:   SSHProtocol,
		User:     url.User,
		Host:     url.Host,
		Path:     url.Path,
		Fragment: splitGitFragment(url.Fragment),
		Remote:   withoutFragment.String(),
	}
}
