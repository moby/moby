package gitutil

import (
	"net/url"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/pkg/errors"
)

// GitRef represents a git ref.
//
// Examples:
//   - "https://github.com/foo/bar.git#baz/qux:quux/quuz" is parsed into:
//     {Remote: "https://github.com/foo/bar.git", ShortName: "bar", Commit:"baz/qux", SubDir: "quux/quuz"}.
type GitRef struct {
	// Remote is the remote repository path.
	Remote string

	// ShortName is the directory name of the repo.
	// e.g., "bar" for "https://github.com/foo/bar.git"
	ShortName string

	// Commit is a commit hash, a tag, or branch name.
	// Commit is optional.
	Commit string

	// SubDir is a directory path inside the repo.
	// SubDir is optional.
	SubDir string

	// IndistinguishableFromLocal is true for a ref that is indistinguishable from a local file path,
	// e.g., "github.com/foo/bar".
	//
	// Deprecated.
	// Instead, use a distinguishable form such as "https://github.com/foo/bar.git".
	//
	// The dockerfile frontend still accepts this form only for build contexts.
	IndistinguishableFromLocal bool

	// UnencryptedTCP is true for a ref that needs an unencrypted TCP connection,
	// e.g., "git://..." and "http://..." .
	//
	// Discouraged, although not deprecated.
	// Instead, consider using an encrypted TCP connection such as "git@github.com/foo/bar.git" or "https://github.com/foo/bar.git".
	UnencryptedTCP bool
}

// var gitURLPathWithFragmentSuffix = regexp.MustCompile(`\.git(?:#.+)?$`)

// ParseGitRef parses a git ref.
func ParseGitRef(ref string) (*GitRef, error) {
	res := &GitRef{}

	var (
		remote *GitURL
		err    error
	)

	if strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "../") {
		return nil, cerrdefs.ErrInvalidArgument
	} else if strings.HasPrefix(ref, "github.com/") {
		res.IndistinguishableFromLocal = true // Deprecated
		remote = fromURL(&url.URL{
			Scheme: "https",
			Host:   "github.com",
			Path:   strings.TrimPrefix(ref, "github.com/"),
		})
	} else {
		remote, err = ParseURL(ref)
		if errors.Is(err, ErrUnknownProtocol) {
			return nil, err
		}
		if err != nil {
			return nil, err
		}

		switch remote.Scheme {
		case HTTPProtocol, GitProtocol:
			res.UnencryptedTCP = true // Discouraged, but not deprecated
		}

		switch remote.Scheme {
		// An HTTP(S) URL is considered to be a valid git ref only when it has the ".git[...]" suffix.
		case HTTPProtocol, HTTPSProtocol:
			if !strings.HasSuffix(remote.Path, ".git") {
				return nil, cerrdefs.ErrInvalidArgument
			}
		}
	}

	res.Remote = remote.Remote
	if res.IndistinguishableFromLocal {
		_, res.Remote, _ = strings.Cut(res.Remote, "://")
	}
	if remote.Fragment != nil {
		res.Commit, res.SubDir = remote.Fragment.Ref, remote.Fragment.Subdir
	}

	repoSplitBySlash := strings.Split(res.Remote, "/")
	res.ShortName = strings.TrimSuffix(repoSplitBySlash[len(repoSplitBySlash)-1], ".git")

	return res, nil
}
