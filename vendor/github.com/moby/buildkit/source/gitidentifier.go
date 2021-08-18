package source

import (
	"net/url"
	"path"
	"strings"

	"github.com/moby/buildkit/util/sshutil"
)

type GitIdentifier struct {
	Remote           string
	Ref              string
	Subdir           string
	KeepGitDir       bool
	AuthTokenSecret  string
	AuthHeaderSecret string
	MountSSHSock     string
	KnownSSHHosts    string
}

func NewGitIdentifier(remoteURL string) (*GitIdentifier, error) {
	repo := GitIdentifier{}

	if !isGitTransport(remoteURL) {
		remoteURL = "https://" + remoteURL
	}

	var fragment string
	if sshutil.IsImplicitSSHTransport(remoteURL) {
		// implicit ssh urls such as "git@.." are not actually a URL, so cannot be parsed as URL
		parts := strings.SplitN(remoteURL, "#", 2)

		repo.Remote = parts[0]
		if len(parts) == 2 {
			fragment = parts[1]
		}
		repo.Ref, repo.Subdir = getRefAndSubdir(fragment)
	} else {
		u, err := url.Parse(remoteURL)
		if err != nil {
			return nil, err
		}

		repo.Ref, repo.Subdir = getRefAndSubdir(u.Fragment)
		u.Fragment = ""
		repo.Remote = u.String()
	}
	if sd := path.Clean(repo.Subdir); sd == "/" || sd == "." {
		repo.Subdir = ""
	}
	return &repo, nil
}

func (i *GitIdentifier) ID() string {
	return "git"
}

// isGitTransport returns true if the provided str is a git transport by inspecting
// the prefix of the string for known protocols used in git.
func isGitTransport(str string) bool {
	return strings.HasPrefix(str, "http://") || strings.HasPrefix(str, "https://") || strings.HasPrefix(str, "git://") || strings.HasPrefix(str, "ssh://") || sshutil.IsImplicitSSHTransport(str)
}

func getRefAndSubdir(fragment string) (ref string, subdir string) {
	refAndDir := strings.SplitN(fragment, ":", 2)
	ref = ""
	if len(refAndDir[0]) != 0 {
		ref = refAndDir[0]
	}
	if len(refAndDir) > 1 && len(refAndDir[1]) != 0 {
		subdir = refAndDir[1]
	}
	return
}
