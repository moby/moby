package git // import "github.com/docker/docker/builder/remotecontext/git"

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestParseRemoteURL(t *testing.T) {
	tests := []struct {
		doc      string
		url      string
		expected gitRepo
	}{
		{
			doc: "git scheme uppercase, no url-fragment",
			url: "GIT://github.com/user/repo.git",
			expected: gitRepo{
				remote: "git://github.com/user/repo.git",
				ref:    "master",
			},
		},
		{
			doc: "git scheme, no url-fragment",
			url: "git://github.com/user/repo.git",
			expected: gitRepo{
				remote: "git://github.com/user/repo.git",
				ref:    "master",
			},
		},
		{
			doc: "git scheme, with url-fragment",
			url: "git://github.com/user/repo.git#mybranch:mydir/mysubdir/",
			expected: gitRepo{
				remote: "git://github.com/user/repo.git",
				ref:    "mybranch",
				subdir: "mydir/mysubdir/",
			},
		},
		{
			doc: "https scheme, no url-fragment",
			url: "https://github.com/user/repo.git",
			expected: gitRepo{
				remote: "https://github.com/user/repo.git",
				ref:    "master",
			},
		},
		{
			doc: "https scheme, with url-fragment",
			url: "https://github.com/user/repo.git#mybranch:mydir/mysubdir/",
			expected: gitRepo{
				remote: "https://github.com/user/repo.git",
				ref:    "mybranch",
				subdir: "mydir/mysubdir/",
			},
		},
		{
			doc: "git@, no url-fragment",
			url: "git@github.com:user/repo.git",
			expected: gitRepo{
				remote: "git@github.com:user/repo.git",
				ref:    "master",
			},
		},
		{
			doc: "git@, with url-fragment",
			url: "git@github.com:user/repo.git#mybranch:mydir/mysubdir/",
			expected: gitRepo{
				remote: "git@github.com:user/repo.git",
				ref:    "mybranch",
				subdir: "mydir/mysubdir/",
			},
		},
		{
			doc: "ssh, no url-fragment",
			url: "ssh://github.com/user/repo.git",
			expected: gitRepo{
				remote: "ssh://github.com/user/repo.git",
				ref:    "master",
			},
		},
		{
			doc: "ssh, with url-fragment",
			url: "ssh://github.com/user/repo.git#mybranch:mydir/mysubdir/",
			expected: gitRepo{
				remote: "ssh://github.com/user/repo.git",
				ref:    "mybranch",
				subdir: "mydir/mysubdir/",
			},
		},
		{
			doc: "ssh, with url-fragment and user",
			url: "ssh://foo%40barcorp.com@github.com/user/repo.git#mybranch:mydir/mysubdir/",
			expected: gitRepo{
				remote: "ssh://foo%40barcorp.com@github.com/user/repo.git",
				ref:    "mybranch",
				subdir: "mydir/mysubdir/",
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.doc, func(t *testing.T) {
			repo, err := parseRemoteURL(tc.url)
			assert.NilError(t, err)
			assert.Check(t, is.DeepEqual(tc.expected, repo, cmp.AllowUnexported(gitRepo{})))
		})
	}
}

func TestCloneArgsSmartHttp(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	serverURL, _ := url.Parse(server.URL)

	serverURL.Path = "/repo.git"

	mux.HandleFunc("/repo.git/info/refs", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("service")
		w.Header().Set("Content-Type", fmt.Sprintf("application/x-%s-advertisement", q))
	})

	args := fetchArgs(serverURL.String(), "master")
	exp := []string{"fetch", "--depth", "1", "origin", "--", "master"}
	assert.Check(t, is.DeepEqual(exp, args))
}

func TestCloneArgsDumbHttp(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	serverURL, _ := url.Parse(server.URL)

	serverURL.Path = "/repo.git"

	mux.HandleFunc("/repo.git/info/refs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
	})

	args := fetchArgs(serverURL.String(), "master")
	exp := []string{"fetch", "origin", "--", "master"}
	assert.Check(t, is.DeepEqual(exp, args))
}

func TestCloneArgsGit(t *testing.T) {
	args := fetchArgs("git://github.com/docker/docker", "master")
	exp := []string{"fetch", "--depth", "1", "origin", "--", "master"}
	assert.Check(t, is.DeepEqual(exp, args))
}

func gitGetConfig(name string) string {
	b, err := gitRepo{}.gitWithinDir("", "config", "--get", name)
	if err != nil {
		// since we are interested in empty or non empty string,
		// we can safely ignore the err here.
		return ""
	}
	return strings.TrimSpace(string(b))
}

func TestCheckoutGit(t *testing.T) {
	root := t.TempDir()

	gitpath, err := exec.LookPath("git")
	assert.NilError(t, err)
	gitversion, _ := exec.Command(gitpath, "version").CombinedOutput()
	t.Logf("%s", gitversion) // E.g. "git version 2.30.2"

	// Serve all repositories under root using the Smart HTTP protocol so
	// they can be cloned. The Dumb HTTP protocol is incompatible with
	// shallow cloning but we unconditionally shallow-clone submodules, and
	// we explicitly disable the file protocol.
	// (Another option would be to use `git daemon` and the Git protocol,
	// but that listens on a fixed port number which is a recipe for
	// disaster in CI. Funnily enough, `git daemon --port=0` works but there
	// is no easy way to discover which port got picked!)

	// Associate git-http-backend logs with the current (sub)test.
	// Incompatible with parallel subtests.
	currentSubtest := t
	githttp := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var logs bytes.Buffer
		(&cgi.Handler{
			Path: gitpath,
			Args: []string{"http-backend"},
			Dir:  root,
			Env: []string{
				"GIT_PROJECT_ROOT=" + root,
				"GIT_HTTP_EXPORT_ALL=1",
			},
			Stderr: &logs,
		}).ServeHTTP(w, r)
		if logs.Len() == 0 {
			return
		}
		for {
			line, err := logs.ReadString('\n')
			currentSubtest.Log("git-http-backend: " + line)
			if err != nil {
				break
			}
		}
	})
	server := httptest.NewServer(&githttp)
	defer server.Close()

	autocrlf := gitGetConfig("core.autocrlf")
	if !(autocrlf == "true" || autocrlf == "false" ||
		autocrlf == "input" || autocrlf == "") {
		t.Logf("unknown core.autocrlf value: \"%s\"", autocrlf)
	}
	eol := "\n"
	if autocrlf == "true" {
		eol = "\r\n"
	}

	must := func(out []byte, err error) {
		t.Helper()
		if len(out) > 0 {
			t.Logf("%s", out)
		}
		assert.NilError(t, err)
	}

	gitDir := filepath.Join(root, "repo")
	must(gitRepo{}.gitWithinDir(root, "-c", "init.defaultBranch=master", "init", gitDir))
	must(gitRepo{}.gitWithinDir(gitDir, "config", "user.email", "test@docker.com"))
	must(gitRepo{}.gitWithinDir(gitDir, "config", "user.name", "Docker test"))
	assert.NilError(t, os.WriteFile(filepath.Join(gitDir, "Dockerfile"), []byte("FROM scratch"), 0644))

	subDir := filepath.Join(gitDir, "subdir")
	assert.NilError(t, os.Mkdir(subDir, 0755))
	assert.NilError(t, os.WriteFile(filepath.Join(subDir, "Dockerfile"), []byte("FROM scratch\nEXPOSE 5000"), 0644))

	if runtime.GOOS != "windows" {
		assert.NilError(t, os.Symlink("../subdir", filepath.Join(gitDir, "parentlink")))
		assert.NilError(t, os.Symlink("/subdir", filepath.Join(gitDir, "absolutelink")))
	}

	must(gitRepo{}.gitWithinDir(gitDir, "add", "-A"))
	must(gitRepo{}.gitWithinDir(gitDir, "commit", "-am", "First commit"))
	must(gitRepo{}.gitWithinDir(gitDir, "checkout", "-b", "test"))

	assert.NilError(t, os.WriteFile(filepath.Join(gitDir, "Dockerfile"), []byte("FROM scratch\nEXPOSE 3000"), 0644))
	assert.NilError(t, os.WriteFile(filepath.Join(subDir, "Dockerfile"), []byte("FROM busybox\nEXPOSE 5000"), 0644))

	must(gitRepo{}.gitWithinDir(gitDir, "add", "-A"))
	must(gitRepo{}.gitWithinDir(gitDir, "commit", "-am", "Branch commit"))
	must(gitRepo{}.gitWithinDir(gitDir, "checkout", "master"))

	// set up submodule
	subrepoDir := filepath.Join(root, "subrepo")
	must(gitRepo{}.gitWithinDir(root, "-c", "init.defaultBranch=master", "init", subrepoDir))
	must(gitRepo{}.gitWithinDir(subrepoDir, "config", "user.email", "test@docker.com"))
	must(gitRepo{}.gitWithinDir(subrepoDir, "config", "user.name", "Docker test"))

	assert.NilError(t, os.WriteFile(filepath.Join(subrepoDir, "subfile"), []byte("subcontents"), 0644))

	must(gitRepo{}.gitWithinDir(subrepoDir, "add", "-A"))
	must(gitRepo{}.gitWithinDir(subrepoDir, "commit", "-am", "Subrepo initial"))

	must(gitRepo{}.gitWithinDir(gitDir, "submodule", "add", server.URL+"/subrepo", "sub"))
	must(gitRepo{}.gitWithinDir(gitDir, "add", "-A"))
	must(gitRepo{}.gitWithinDir(gitDir, "commit", "-am", "With submodule"))

	type singleCase struct {
		frag      string
		exp       string
		fail      bool
		submodule bool
	}

	cases := []singleCase{
		{"", "FROM scratch", false, true},
		{"master", "FROM scratch", false, true},
		{":subdir", "FROM scratch" + eol + "EXPOSE 5000", false, false},
		{":nosubdir", "", true, false},   // missing directory error
		{":Dockerfile", "", true, false}, // not a directory error
		{"master:nosubdir", "", true, false},
		{"master:subdir", "FROM scratch" + eol + "EXPOSE 5000", false, false},
		{"master:../subdir", "", true, false},
		{"test", "FROM scratch" + eol + "EXPOSE 3000", false, false},
		{"test:", "FROM scratch" + eol + "EXPOSE 3000", false, false},
		{"test:subdir", "FROM busybox" + eol + "EXPOSE 5000", false, false},
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
		t.Run(c.frag, func(t *testing.T) {
			currentSubtest = t
			ref, subdir := getRefAndSubdir(c.frag)
			r, err := gitRepo{remote: server.URL + "/repo", ref: ref, subdir: subdir}.clone()

			if c.fail {
				assert.Check(t, is.ErrorContains(err, ""))
				return
			}
			assert.NilError(t, err)
			defer os.RemoveAll(r)
			if c.submodule {
				b, err := os.ReadFile(filepath.Join(r, "sub/subfile"))
				assert.NilError(t, err)
				assert.Check(t, is.Equal("subcontents", string(b)))
			} else {
				_, err := os.Stat(filepath.Join(r, "sub/subfile"))
				assert.Assert(t, is.ErrorContains(err, ""))
				assert.Assert(t, os.IsNotExist(err))
			}

			b, err := os.ReadFile(filepath.Join(r, "Dockerfile"))
			assert.NilError(t, err)
			assert.Check(t, is.Equal(c.exp, string(b)))
		})
	}
}

func TestValidGitTransport(t *testing.T) {
	gitUrls := []string{
		"git://github.com/docker/docker",
		"git@github.com:docker/docker.git",
		"git@bitbucket.org:atlassianlabs/atlassian-docker.git",
		"https://github.com/docker/docker.git",
		"http://github.com/docker/docker.git",
		"http://github.com/docker/docker.git#branch",
		"http://github.com/docker/docker.git#:dir",
	}
	incompleteGitUrls := []string{
		"github.com/docker/docker",
	}

	for _, url := range gitUrls {
		if !isGitTransport(url) {
			t.Fatalf("%q should be detected as valid Git prefix", url)
		}
	}

	for _, url := range incompleteGitUrls {
		if isGitTransport(url) {
			t.Fatalf("%q should not be detected as valid Git prefix", url)
		}
	}
}

func TestGitInvalidRef(t *testing.T) {
	gitUrls := []string{
		"git://github.com/moby/moby#--foo bar",
		"git@github.com/moby/moby#--upload-pack=sleep;:",
		"git@g.com:a/b.git#-B",
		"git@g.com:a/b.git#with space",
	}

	for _, url := range gitUrls {
		_, err := Clone(url)
		assert.Assert(t, err != nil)
		assert.Check(t, is.Contains(strings.ToLower(err.Error()), "invalid refspec"))
	}
}
