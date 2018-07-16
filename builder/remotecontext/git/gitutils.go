package git // import "github.com/docker/docker/builder/remotecontext/git"

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/pkg/symlink"
	"github.com/docker/docker/pkg/urlutil"
	"github.com/pkg/errors"
)

type gitRepo struct {
	remote string
	ref    string
	subdir string
}

// Clone clones a repository into a newly created directory which
// will be under "docker-build-git"
func Clone(remoteURL string) (string, error) {
	repo, err := parseRemoteURL(remoteURL)

	if err != nil {
		return "", err
	}

	return cloneGitRepo(repo)
}

func cloneGitRepo(repo gitRepo) (checkoutDir string, err error) {
	fetch := fetchArgs(repo.remote, repo.ref)

	root, err := ioutil.TempDir("", "docker-build-git")
	if err != nil {
		return "", err
	}

	defer func() {
		if err != nil {
			os.RemoveAll(root)
		}
	}()

	if out, err := gitWithinDir(root, "init"); err != nil {
		return "", errors.Wrapf(err, "failed to init repo at %s: %s", root, out)
	}

	// Add origin remote for compatibility with previous implementation that
	// used "git clone" and also to make sure local refs are created for branches
	if out, err := gitWithinDir(root, "remote", "add", "origin", repo.remote); err != nil {
		return "", errors.Wrapf(err, "failed add origin repo at %s: %s", repo.remote, out)
	}

	if output, err := gitWithinDir(root, fetch...); err != nil {
		return "", errors.Wrapf(err, "error fetching: %s", output)
	}

	checkoutDir, err = checkoutGit(root, repo.ref, repo.subdir)
	if err != nil {
		return "", err
	}

	v, err := gitVersion()
	if err != nil {
		return "", err
	}
	args := []string{"submodule", "update", "--init", "--recursive"}
	if versions.GreaterThanOrEqualTo(v, "1.8.4") {
		// --depth was added in Git 1.8.4; https://github.com/git/git/commit/275cd184d52b5b81cb89e4ec33e540fb2ae61c1f
		args = append(args, "--depth=1")
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.Wrapf(err, "error initializing submodules: %s", output)
	}

	return checkoutDir, nil
}

// gitVersion returns the version of git that is installed on the host
func gitVersion() (string, error) {
	cmd := exec.Command("git", "--version")
	output, err := cmd.Output() // e.g. git version 2.17.0
	if err != nil {
		return "", errors.Wrap(err, "unable to determine installed git version")
	}

	parts := strings.SplitN(string(output), " ", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("unable to determine installed git version from output: %s", output)
	}
	return parts[2], nil
}

func parseRemoteURL(remoteURL string) (gitRepo, error) {
	repo := gitRepo{}

	if !isGitTransport(remoteURL) {
		remoteURL = "https://" + remoteURL
	}

	var fragment string
	if strings.HasPrefix(remoteURL, "git@") {
		// git@.. is not an URL, so cannot be parsed as URL
		parts := strings.SplitN(remoteURL, "#", 2)

		repo.remote = parts[0]
		if len(parts) == 2 {
			fragment = parts[1]
		}
		repo.ref, repo.subdir = getRefAndSubdir(fragment)
	} else {
		u, err := url.Parse(remoteURL)
		if err != nil {
			return repo, err
		}

		repo.ref, repo.subdir = getRefAndSubdir(u.Fragment)
		u.Fragment = ""
		repo.remote = u.String()
	}
	return repo, nil
}

func getRefAndSubdir(fragment string) (ref string, subdir string) {
	refAndDir := strings.SplitN(fragment, ":", 2)
	ref = "master"
	if len(refAndDir[0]) != 0 {
		ref = refAndDir[0]
	}
	if len(refAndDir) > 1 && len(refAndDir[1]) != 0 {
		subdir = refAndDir[1]
	}
	return
}

func fetchArgs(remoteURL string, ref string) []string {
	args := []string{"fetch"}

	if supportsShallowClone(remoteURL) {
		args = append(args, "--depth", "1")
	}

	return append(args, "origin", ref)
}

// Check if a given git URL supports a shallow git clone,
// i.e. it is a non-HTTP server or a smart HTTP server.
func supportsShallowClone(remoteURL string) bool {
	if urlutil.IsURL(remoteURL) {
		// Check if the HTTP server is smart

		// Smart servers must correctly respond to a query for the git-upload-pack service
		serviceURL := remoteURL + "/info/refs?service=git-upload-pack"

		// Try a HEAD request and fallback to a Get request on error
		res, err := http.Head(serviceURL)
		if err != nil || res.StatusCode != http.StatusOK {
			res, err = http.Get(serviceURL)
			if err == nil {
				res.Body.Close()
			}
			if err != nil || res.StatusCode != http.StatusOK {
				// request failed
				return false
			}
		}

		if res.Header.Get("Content-Type") != "application/x-git-upload-pack-advertisement" {
			// Fallback, not a smart server
			return false
		}
		return true
	}
	// Non-HTTP protocols always support shallow clones
	return true
}

func checkoutGit(root, ref, subdir string) (string, error) {
	// Try checking out by ref name first. This will work on branches and sets
	// .git/HEAD to the current branch name
	if output, err := gitWithinDir(root, "checkout", ref); err != nil {
		// If checking out by branch name fails check out the last fetched ref
		if _, err2 := gitWithinDir(root, "checkout", "FETCH_HEAD"); err2 != nil {
			return "", errors.Wrapf(err, "error checking out %s: %s", ref, output)
		}
	}

	if subdir != "" {
		newCtx, err := symlink.FollowSymlinkInScope(filepath.Join(root, subdir), root)
		if err != nil {
			return "", errors.Wrapf(err, "error setting git context, %q not within git root", subdir)
		}

		fi, err := os.Stat(newCtx)
		if err != nil {
			return "", err
		}
		if !fi.IsDir() {
			return "", errors.Errorf("error setting git context, not a directory: %s", newCtx)
		}
		root = newCtx
	}

	return root, nil
}

func gitWithinDir(dir string, args ...string) ([]byte, error) {
	a := []string{"--work-tree", dir, "--git-dir", filepath.Join(dir, ".git")}
	return git(append(a, args...)...)
}

func git(args ...string) ([]byte, error) {
	return exec.Command("git", args...).CombinedOutput()
}

// isGitTransport returns true if the provided str is a git transport by inspecting
// the prefix of the string for known protocols used in git.
func isGitTransport(str string) bool {
	return urlutil.IsURL(str) || strings.HasPrefix(str, "git://") || strings.HasPrefix(str, "git@")
}
