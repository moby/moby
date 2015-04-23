package utils

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strings"

	"github.com/docker/docker/pkg/urlutil"
)

func GitClone(remoteURL string) (string, error) {
	if !urlutil.IsGitTransport(remoteURL) {
		remoteURL = "https://" + remoteURL
	}
	root, err := ioutil.TempDir("", "docker-build-git")
	if err != nil {
		return "", err
	}

	clone := cloneArgs(remoteURL, root)

	if output, err := exec.Command("git", clone...).CombinedOutput(); err != nil {
		return "", fmt.Errorf("Error trying to use git: %s (%s)", err, output)
	}

	return root, nil
}

func cloneArgs(remoteURL, root string) []string {
	args := []string{"clone", "--recursive"}
	shallow := true

	if strings.HasPrefix(remoteURL, "http") {
		res, err := http.Head(fmt.Sprintf("%s/info/refs?service=git-upload-pack", remoteURL))
		if err != nil || res.Header.Get("Content-Type") != "application/x-git-upload-pack-advertisement" {
			shallow = false
		}
	}

	if shallow {
		args = append(args, "--depth", "1")
	}

	return append(args, remoteURL, root)
}
