package gitutil

import (
	"regexp"
	"strings"

	"github.com/containerd/containerd/errdefs"
)

// GitRef represents a git ref.
//
// Examples:
// - "https://github.com/foo/bar.git#baz/qux:quux/quuz" is parsed into:
//   {Remote: "https://github.com/foo/bar.git", ShortName: "bar", Commit:"baz/qux", SubDir: "quux/quuz"}.
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

	if strings.HasPrefix(ref, "github.com/") {
		res.IndistinguishableFromLocal = true // Deprecated
	} else {
		_, proto := ParseProtocol(ref)
		switch proto {
		case UnknownProtocol:
			return nil, errdefs.ErrInvalidArgument
		}
		switch proto {
		case HTTPProtocol, GitProtocol:
			res.UnencryptedTCP = true // Discouraged, but not deprecated
		}
		switch proto {
		// An HTTP(S) URL is considered to be a valid git ref only when it has the ".git[...]" suffix.
		case HTTPProtocol, HTTPSProtocol:
			var gitURLPathWithFragmentSuffix = regexp.MustCompile(`\.git(?:#.+)?$`)
			if !gitURLPathWithFragmentSuffix.MatchString(ref) {
				return nil, errdefs.ErrInvalidArgument
			}
		}
	}

	refSplitBySharp := strings.SplitN(ref, "#", 2)
	res.Remote = refSplitBySharp[0]
	if len(res.Remote) == 0 {
		return res, errdefs.ErrInvalidArgument
	}

	if len(refSplitBySharp) > 1 {
		refSplitBySharpSplitByColon := strings.SplitN(refSplitBySharp[1], ":", 2)
		res.Commit = refSplitBySharpSplitByColon[0]
		if len(res.Commit) == 0 {
			return res, errdefs.ErrInvalidArgument
		}
		if len(refSplitBySharpSplitByColon) > 1 {
			res.SubDir = refSplitBySharpSplitByColon[1]
		}
	}
	repoSplitBySlash := strings.Split(res.Remote, "/")
	res.ShortName = strings.TrimSuffix(repoSplitBySlash[len(repoSplitBySlash)-1], ".git")
	return res, nil
}
