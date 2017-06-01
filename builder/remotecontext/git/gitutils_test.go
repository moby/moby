package git

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloneArgsSmartHttp(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	serverURL, _ := url.Parse(server.URL)

	serverURL.Path = "/repo.git"

	mux.HandleFunc("/repo.git/info/refs", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("service")
		w.Header().Set("Content-Type", fmt.Sprintf("application/x-%s-advertisement", q))
	})

	args := fetchArgs(serverURL, "master")
	exp := []string{"fetch", "--recurse-submodules=yes", "--depth", "1", "origin", "master"}
	assert.Equal(t, exp, args)
}

func TestCloneArgsDumbHttp(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	serverURL, _ := url.Parse(server.URL)

	serverURL.Path = "/repo.git"

	mux.HandleFunc("/repo.git/info/refs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
	})

	args := fetchArgs(serverURL, "master")
	exp := []string{"fetch", "--recurse-submodules=yes", "origin", "master"}
	assert.Equal(t, exp, args)
}

func TestCloneArgsGit(t *testing.T) {
	u, _ := url.Parse("git://github.com/docker/docker")
	args := fetchArgs(u, "master")
	exp := []string{"fetch", "--recurse-submodules=yes", "--depth", "1", "origin", "master"}
	assert.Equal(t, exp, args)
}

func gitGetConfig(name string) string {
	b, err := git([]string{"config", "--get", name}...)
	if err != nil {
		// since we are interested in empty or non empty string,
		// we can safely ignore the err here.
		return ""
	}
	return strings.TrimSpace(string(b))
}

func TestCheckoutGit(t *testing.T) {
	root, err := ioutil.TempDir("", "docker-build-git-checkout")
	require.NoError(t, err)
	defer os.RemoveAll(root)

	autocrlf := gitGetConfig("core.autocrlf")
	if !(autocrlf == "true" || autocrlf == "false" ||
		autocrlf == "input" || autocrlf == "") {
		t.Logf("unknown core.autocrlf value: \"%s\"", autocrlf)
	}
	eol := "\n"
	if autocrlf == "true" {
		eol = "\r\n"
	}

	gitDir := filepath.Join(root, "repo")
	_, err = git("init", gitDir)
	require.NoError(t, err)

	_, err = gitWithinDir(gitDir, "config", "user.email", "test@docker.com")
	require.NoError(t, err)

	_, err = gitWithinDir(gitDir, "config", "user.name", "Docker test")
	require.NoError(t, err)

	err = ioutil.WriteFile(filepath.Join(gitDir, "Dockerfile"), []byte("FROM scratch"), 0644)
	require.NoError(t, err)

	subDir := filepath.Join(gitDir, "subdir")
	require.NoError(t, os.Mkdir(subDir, 0755))

	err = ioutil.WriteFile(filepath.Join(subDir, "Dockerfile"), []byte("FROM scratch\nEXPOSE 5000"), 0644)
	require.NoError(t, err)

	if runtime.GOOS != "windows" {
		if err = os.Symlink("../subdir", filepath.Join(gitDir, "parentlink")); err != nil {
			t.Fatal(err)
		}

		if err = os.Symlink("/subdir", filepath.Join(gitDir, "absolutelink")); err != nil {
			t.Fatal(err)
		}
	}

	_, err = gitWithinDir(gitDir, "add", "-A")
	require.NoError(t, err)

	_, err = gitWithinDir(gitDir, "commit", "-am", "First commit")
	require.NoError(t, err)

	_, err = gitWithinDir(gitDir, "checkout", "-b", "test")
	require.NoError(t, err)

	err = ioutil.WriteFile(filepath.Join(gitDir, "Dockerfile"), []byte("FROM scratch\nEXPOSE 3000"), 0644)
	require.NoError(t, err)

	err = ioutil.WriteFile(filepath.Join(subDir, "Dockerfile"), []byte("FROM busybox\nEXPOSE 5000"), 0644)
	require.NoError(t, err)

	_, err = gitWithinDir(gitDir, "add", "-A")
	require.NoError(t, err)

	_, err = gitWithinDir(gitDir, "commit", "-am", "Branch commit")
	require.NoError(t, err)

	_, err = gitWithinDir(gitDir, "checkout", "master")
	require.NoError(t, err)

	type singleCase struct {
		frag string
		exp  string
		fail bool
	}

	cases := []singleCase{
		{"", "FROM scratch", false},
		{"master", "FROM scratch", false},
		{":subdir", "FROM scratch" + eol + "EXPOSE 5000", false},
		{":nosubdir", "", true},   // missing directory error
		{":Dockerfile", "", true}, // not a directory error
		{"master:nosubdir", "", true},
		{"master:subdir", "FROM scratch" + eol + "EXPOSE 5000", false},
		{"master:../subdir", "", true},
		{"test", "FROM scratch" + eol + "EXPOSE 3000", false},
		{"test:", "FROM scratch" + eol + "EXPOSE 3000", false},
		{"test:subdir", "FROM busybox" + eol + "EXPOSE 5000", false},
	}

	if runtime.GOOS != "windows" {
		// Windows GIT (2.7.1 x64) does not support parentlink/absolutelink. Sample output below
		// 	git --work-tree .\repo --git-dir .\repo\.git add -A
		//	error: readlink("absolutelink"): Function not implemented
		// 	error: unable to index file absolutelink
		// 	fatal: adding files failed
		cases = append(cases, singleCase{frag: "master:absolutelink", exp: "FROM scratch" + eol + "EXPOSE 5000", fail: false})
		cases = append(cases, singleCase{frag: "master:parentlink", exp: "FROM scratch" + eol + "EXPOSE 5000", fail: false})
	}

	for _, c := range cases {
		ref, subdir := getRefAndSubdir(c.frag)
		r, err := checkoutGit(gitDir, ref, subdir)

		if c.fail {
			assert.Error(t, err)
			continue
		}

		b, err := ioutil.ReadFile(filepath.Join(r, "Dockerfile"))
		require.NoError(t, err)
		assert.Equal(t, c.exp, string(b))
	}
}
