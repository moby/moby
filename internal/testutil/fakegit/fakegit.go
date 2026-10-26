package fakegit

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/moby/moby/v2/internal/testutil/fakecontext"
	"github.com/moby/moby/v2/internal/testutil/fakestorage"
)

type gitServer interface {
	URL() string
	Close() error
}

type localGitServer struct {
	*httptest.Server
}

func (r *localGitServer) Close() error {
	r.Server.Close()
	return nil
}

func (r *localGitServer) URL() string {
	return r.Server.URL
}

// FakeGit is a fake git server
type FakeGit struct {
	root    string
	server  gitServer
	RepoURL string
}

// Close closes the server, implements Closer interface
func (g *FakeGit) Close() {
	g.server.Close()
	os.RemoveAll(g.root)
}

// New creates a fake git server that can be used for git-related tests.
func New(c testing.TB, name string, files map[string]string, enforceLocalServer bool) *FakeGit {
	c.Helper()
	ctx := fakecontext.New(c, "", fakecontext.WithFiles(files))
	defer func() { _ = ctx.Close() }()

	if output, err := exec.Command("git", "init", ctx.Dir).CombinedOutput(); err != nil {
		c.Fatalf("error trying to init repo: %s (%s)", err, output)
	}
	if output, err := exec.Command("git", "-C", ctx.Dir, "config", "user.name", "Fake User").CombinedOutput(); err != nil {
		c.Fatalf("error trying to set 'user.name': %s (%s)", err, output)
	}
	if output, err := exec.Command("git", "-C", ctx.Dir, "config", "user.email", "fake.user@example.com").CombinedOutput(); err != nil {
		c.Fatalf("error trying to set 'user.email': %s (%s)", err, output)
	}
	if output, err := exec.Command("git", "-C", ctx.Dir, "add", ".").CombinedOutput(); err != nil {
		c.Fatalf("error trying to add files to repo: %s (%s)", err, output)
	}
	if output, err := exec.Command("git", "-C", ctx.Dir, "commit", "-m", "Initial commit").CombinedOutput(); err != nil {
		c.Fatalf("error trying to commit to repo: %s (%s)", err, output)
	}

	root, err := os.MkdirTemp("", "docker-test-git-repo")
	if err != nil {
		c.Fatal(err)
	}
	c.Cleanup(func() { _ = os.RemoveAll(root) })

	repoPath := filepath.Join(root, name+".git")
	if output, err := exec.Command("git", "clone", "--bare", ctx.Dir, repoPath).CombinedOutput(); err != nil {
		c.Fatalf("error trying to clone --bare: %s (%s)", err, output)
	}
	if output, err := exec.Command("git", "-C", repoPath, "update-server-info").CombinedOutput(); err != nil {
		c.Fatalf("error trying to git update-server-info: %s (%s)", err, output)
	}

	var server gitServer
	if !enforceLocalServer {
		// use fakeStorage server, which might be local or remote (at test daemon)
		server = fakestorage.New(c, root)
	} else {
		// always start a local http server on CLI test machine
		httpServer := httptest.NewServer(http.FileServer(http.Dir(root)))
		server = &localGitServer{httpServer}
	}
	return &FakeGit{
		root:    root,
		server:  server,
		RepoURL: fmt.Sprintf("%s/%s.git", server.URL(), name),
	}
}
