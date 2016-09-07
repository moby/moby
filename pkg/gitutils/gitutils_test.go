package gitutils

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestCloneArgsSmartHttp(c *check.C) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	serverURL, _ := url.Parse(server.URL)

	serverURL.Path = "/repo.git"
	gitURL := serverURL.String()

	mux.HandleFunc("/repo.git/info/refs", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("service")
		w.Header().Set("Content-Type", fmt.Sprintf("application/x-%s-advertisement", q))
	})

	args := cloneArgs(serverURL, "/tmp")
	exp := []string{"clone", "--recursive", "--depth", "1", gitURL, "/tmp"}
	if !reflect.DeepEqual(args, exp) {
		c.Fatalf("Expected %v, got %v", exp, args)
	}
}

func (s *DockerSuite) TestCloneArgsDumbHttp(c *check.C) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	serverURL, _ := url.Parse(server.URL)

	serverURL.Path = "/repo.git"
	gitURL := serverURL.String()

	mux.HandleFunc("/repo.git/info/refs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
	})

	args := cloneArgs(serverURL, "/tmp")
	exp := []string{"clone", "--recursive", gitURL, "/tmp"}
	if !reflect.DeepEqual(args, exp) {
		c.Fatalf("Expected %v, got %v", exp, args)
	}
}

func (s *DockerSuite) TestCloneArgsGit(c *check.C) {
	u, _ := url.Parse("git://github.com/docker/docker")
	args := cloneArgs(u, "/tmp")
	exp := []string{"clone", "--recursive", "--depth", "1", "git://github.com/docker/docker", "/tmp"}
	if !reflect.DeepEqual(args, exp) {
		c.Fatalf("Expected %v, got %v", exp, args)
	}
}

func (s *DockerSuite) TestCloneArgsStripFragment(c *check.C) {
	u, _ := url.Parse("git://github.com/docker/docker#test")
	args := cloneArgs(u, "/tmp")
	exp := []string{"clone", "--recursive", "git://github.com/docker/docker", "/tmp"}
	if !reflect.DeepEqual(args, exp) {
		c.Fatalf("Expected %v, got %v", exp, args)
	}
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

func (s *DockerSuite) TestCheckoutGit(c *check.C) {
	root, err := ioutil.TempDir("", "docker-build-git-checkout")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(root)

	autocrlf := gitGetConfig("core.autocrlf")
	if !(autocrlf == "true" || autocrlf == "false" ||
		autocrlf == "input" || autocrlf == "") {
		c.Logf("unknown core.autocrlf value: \"%s\"", autocrlf)
	}
	eol := "\n"
	if autocrlf == "true" {
		eol = "\r\n"
	}

	gitDir := filepath.Join(root, "repo")
	_, err = git("init", gitDir)
	if err != nil {
		c.Fatal(err)
	}

	if _, err = gitWithinDir(gitDir, "config", "user.email", "test@docker.com"); err != nil {
		c.Fatal(err)
	}

	if _, err = gitWithinDir(gitDir, "config", "user.name", "Docker test"); err != nil {
		c.Fatal(err)
	}

	if err = ioutil.WriteFile(filepath.Join(gitDir, "Dockerfile"), []byte("FROM scratch"), 0644); err != nil {
		c.Fatal(err)
	}

	subDir := filepath.Join(gitDir, "subdir")
	if err = os.Mkdir(subDir, 0755); err != nil {
		c.Fatal(err)
	}

	if err = ioutil.WriteFile(filepath.Join(subDir, "Dockerfile"), []byte("FROM scratch\nEXPOSE 5000"), 0644); err != nil {
		c.Fatal(err)
	}

	if runtime.GOOS != "windows" {
		if err = os.Symlink("../subdir", filepath.Join(gitDir, "parentlink")); err != nil {
			c.Fatal(err)
		}

		if err = os.Symlink("/subdir", filepath.Join(gitDir, "absolutelink")); err != nil {
			c.Fatal(err)
		}
	}

	if _, err = gitWithinDir(gitDir, "add", "-A"); err != nil {
		c.Fatal(err)
	}

	if _, err = gitWithinDir(gitDir, "commit", "-am", "First commit"); err != nil {
		c.Fatal(err)
	}

	if _, err = gitWithinDir(gitDir, "checkout", "-b", "test"); err != nil {
		c.Fatal(err)
	}

	if err = ioutil.WriteFile(filepath.Join(gitDir, "Dockerfile"), []byte("FROM scratch\nEXPOSE 3000"), 0644); err != nil {
		c.Fatal(err)
	}

	if err = ioutil.WriteFile(filepath.Join(subDir, "Dockerfile"), []byte("FROM busybox\nEXPOSE 5000"), 0644); err != nil {
		c.Fatal(err)
	}

	if _, err = gitWithinDir(gitDir, "add", "-A"); err != nil {
		c.Fatal(err)
	}

	if _, err = gitWithinDir(gitDir, "commit", "-am", "Branch commit"); err != nil {
		c.Fatal(err)
	}

	if _, err = gitWithinDir(gitDir, "checkout", "master"); err != nil {
		c.Fatal(err)
	}

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

	for _, ca := range cases {
		r, err := checkoutGit(ca.frag, gitDir)

		fail := err != nil
		if fail != ca.fail {
			c.Fatalf("Expected %v failure, error was %v\n", ca.fail, err)
		}
		if ca.fail {
			continue
		}

		b, err := ioutil.ReadFile(filepath.Join(r, "Dockerfile"))
		if err != nil {
			c.Fatal(err)
		}

		if string(b) != ca.exp {
			c.Fatalf("Expected %v, was %v\n", ca.exp, string(b))
		}
	}
}
