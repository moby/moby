package git

import (
	"path"

	"github.com/moby/buildkit/solver/llbsolver/provenance"
	provenancetypes "github.com/moby/buildkit/solver/llbsolver/provenance/types"
	"github.com/moby/buildkit/source"
	srctypes "github.com/moby/buildkit/source/types"
	"github.com/moby/buildkit/util/gitutil"
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
	if !gitutil.IsGitTransport(remoteURL) {
		remoteURL = "https://" + remoteURL
	}
	u, err := gitutil.ParseURL(remoteURL)
	if err != nil {
		return nil, err
	}

	repo := GitIdentifier{Remote: u.Remote}
	if u.Fragment != nil {
		repo.Ref = u.Fragment.Ref
		repo.Subdir = u.Fragment.Subdir
	}
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
	c.AddGit(provenancetypes.GitSource{
		URL:    url,
		Commit: pin,
	})
	if id.AuthTokenSecret != "" {
		c.AddSecret(provenancetypes.Secret{
			ID:       id.AuthTokenSecret,
			Optional: true,
		})
	}
	if id.AuthHeaderSecret != "" {
		c.AddSecret(provenancetypes.Secret{
			ID:       id.AuthHeaderSecret,
			Optional: true,
		})
	}
	if id.MountSSHSock != "" {
		c.AddSSH(provenancetypes.SSH{
			ID:       id.MountSSHSock,
			Optional: true,
		})
	}
	return nil
}
