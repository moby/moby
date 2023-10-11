package git

import (
	"path"
	"strings"

	"github.com/moby/buildkit/solver/llbsolver/provenance"
	"github.com/moby/buildkit/source"
	srctypes "github.com/moby/buildkit/source/types"
	"github.com/moby/buildkit/util/gitutil"
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
	if !isGitTransport(remoteURL) {
		remoteURL = "https://" + remoteURL
	}
	u, err := gitutil.ParseURL(remoteURL)
	if err != nil {
		return nil, err
	}

	repo := GitIdentifier{}
	repo.Ref, repo.Subdir = gitutil.SplitGitFragment(u.Fragment)
	u.Fragment = ""
	repo.Remote = u.String()

	if sd := path.Clean(repo.Subdir); sd == "/" || sd == "." {
		repo.Subdir = ""
	}
	return &repo, nil
}

func (GitIdentifier) Scheme() string {
	return srctypes.GitScheme
}

var _ source.Identifier = (*GitIdentifier)(nil)

func (id *GitIdentifier) Capture(c *provenance.Capture, pin string) error {
	url := id.Remote
	if id.Ref != "" {
		url += "#" + id.Ref
	}
	c.AddGit(provenance.GitSource{
		URL:    url,
		Commit: pin,
	})
	if id.AuthTokenSecret != "" {
		c.AddSecret(provenance.Secret{
			ID:       id.AuthTokenSecret,
			Optional: true,
		})
	}
	if id.AuthHeaderSecret != "" {
		c.AddSecret(provenance.Secret{
			ID:       id.AuthHeaderSecret,
			Optional: true,
		})
	}
	if id.MountSSHSock != "" {
		c.AddSSH(provenance.SSH{
			ID:       id.MountSSHSock,
			Optional: true,
		})
	}
	return nil
}

// isGitTransport returns true if the provided str is a git transport by inspecting
// the prefix of the string for known protocols used in git.
func isGitTransport(str string) bool {
	return strings.HasPrefix(str, "http://") || strings.HasPrefix(str, "https://") || strings.HasPrefix(str, "git://") || strings.HasPrefix(str, "ssh://") || sshutil.IsImplicitSSHTransport(str)
}
