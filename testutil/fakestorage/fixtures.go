package fakestorage

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/moby/go-archive"
	"github.com/moby/moby/api/types/build"
	"gotest.tools/v3/assert"
)

var ensureHTTPServerOnce sync.Once

func ensureHTTPServerImage(t testing.TB) {
	t.Helper()
	var doIt bool
	ensureHTTPServerOnce.Do(func() {
		doIt = true
	})

	if !doIt {
		return
	}

	goos := testEnv.DaemonInfo.OSType
	if goos == "" {
		goos = "linux"
	}
	goarch := testEnv.DaemonVersion.Arch
	if goarch == "" {
		goarch = "amd64"
	}

	goCmd, err := exec.LookPath("go")
	assert.NilError(t, err, "could not find go executable to build http server")

	tmp := t.TempDir()
	const httpServer = `package main

import (
	"log"
	"net/http"
)

func main() {
	fs := http.FileServer(http.Dir("/static"))
	http.Handle("/", fs)
	log.Panic(http.ListenAndServe(":80", nil)) // #nosec G114 -- Ignoring for test-code: G114: Use of net/http serve function that has no support for setting timeouts (gosec)
}
`
	src := filepath.Join(tmp, "main.go")
	err = os.WriteFile(filepath.Join(tmp, "main.go"), []byte(httpServer), 0o0644)
	assert.NilError(t, err)

	cmd := exec.Command(goCmd, "build", "-o", filepath.Join(tmp, "httpserver"), src)
	cmd.Env = append(os.Environ(), []string{
		"CGO_ENABLED=0",
		"GOOS=" + goos,
		"GOARCH=" + goarch,
	}...)
	out, err := cmd.CombinedOutput()
	assert.NilError(t, err, "could not build http server: %s", string(out))
	const dockerfile = `FROM scratch
EXPOSE 80/tcp
COPY httpserver .
CMD ["./httpserver"]
`
	err = os.WriteFile(filepath.Join(tmp, "Dockerfile"), []byte(dockerfile), 0o644)
	assert.NilError(t, err, "could not write Dockerfile")
	reader, err := archive.TarWithOptions(tmp, &archive.TarOptions{})
	assert.NilError(t, err)

	apiClient := testEnv.APIClient()
	resp, err := apiClient.ImageBuild(context.Background(), reader, build.ImageBuildOptions{
		Remove:      true,
		ForceRemove: true,
		Tags:        []string{"httpserver"},
	})
	assert.NilError(t, err)
	_, err = io.Copy(io.Discard, resp.Body)
	assert.NilError(t, err)
	testEnv.ProtectImage(t, "httpserver:latest")
}
