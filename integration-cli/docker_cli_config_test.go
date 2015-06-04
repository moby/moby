package main

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/docker/docker/autogen/dockerversion"
	"github.com/docker/docker/pkg/homedir"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestConfigHttpHeader(c *check.C) {
	testRequires(c, UnixCli) // Can't set/unset HOME on windows right now
	// We either need a level of Go that supports Unsetenv (for cases
	// when HOME/USERPROFILE isn't set), or we need to be able to use
	// os/user but user.Current() only works if we aren't statically compiling

	var headers map[string][]string

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			headers = r.Header
		}))
	defer server.Close()

	homeKey := homedir.Key()
	homeVal := homedir.Get()
	tmpDir, _ := ioutil.TempDir("", "fake-home")
	defer os.RemoveAll(tmpDir)

	dotDocker := filepath.Join(tmpDir, ".docker")
	os.Mkdir(dotDocker, 0600)
	tmpCfg := filepath.Join(dotDocker, "config.json")

	defer func() { os.Setenv(homeKey, homeVal) }()
	os.Setenv(homeKey, tmpDir)

	data := `{
		"HttpHeaders": { "MyHeader": "MyValue" }
	}`

	err := ioutil.WriteFile(tmpCfg, []byte(data), 0600)
	if err != nil {
		c.Fatalf("Err creating file(%s): %v", tmpCfg, err)
	}

	cmd := exec.Command(dockerBinary, "-H="+server.URL[7:], "ps")
	out, _, _ := runCommandWithOutput(cmd)

	if headers["User-Agent"] == nil {
		c.Fatalf("Missing User-Agent: %q\nout:%v", headers, out)
	}

	if headers["User-Agent"][0] != "Docker-Client/"+dockerversion.VERSION+" ("+runtime.GOOS+")" {
		c.Fatalf("Badly formatted User-Agent: %q\nout:%v", headers, out)
	}

	if headers["Myheader"] == nil || headers["Myheader"][0] != "MyValue" {
		c.Fatalf("Missing/bad header: %q\nout:%v", headers, out)
	}
}
