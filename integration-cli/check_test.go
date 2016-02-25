package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/docker/docker/pkg/authorization"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/pkg/reexec"
	"github.com/go-check/check"
)

func Test(t *testing.T) {
	reexec.Init() // This is required for external graphdriver tests

	if !isLocalDaemon {
		fmt.Println("INFO: Testing against a remote daemon")
	} else {
		fmt.Println("INFO: Testing against a local daemon")
	}

	check.TestingT(t)
}

func init() {
	check.Suite(&DockerSuite{})
}

type DockerSuite struct {
}

func (s *DockerSuite) TearDownTest(c *check.C) {
	unpauseAllContainers()
	deleteAllContainers()
	deleteAllImages()
	deleteAllVolumes()
	deleteAllNetworks()
}

func init() {
	check.Suite(&DockerRegistrySuite{
		ds: &DockerSuite{},
	})
}

type DockerRegistrySuite struct {
	ds  *DockerSuite
	reg *testRegistryV2
	d   *Daemon
}

func (s *DockerRegistrySuite) SetUpTest(c *check.C) {
	testRequires(c, DaemonIsLinux, RegistryHosting)
	s.reg = setupRegistry(c, false, false)
	s.d = NewDaemon(c)
}

func (s *DockerRegistrySuite) TearDownTest(c *check.C) {
	if s.reg != nil {
		s.reg.Close()
	}
	if s.d != nil {
		s.d.Stop()
	}
	s.ds.TearDownTest(c)
}

func init() {
	check.Suite(&DockerSchema1RegistrySuite{
		ds: &DockerSuite{},
	})
}

type DockerSchema1RegistrySuite struct {
	ds  *DockerSuite
	reg *testRegistryV2
	d   *Daemon
}

func (s *DockerSchema1RegistrySuite) SetUpTest(c *check.C) {
	testRequires(c, DaemonIsLinux, RegistryHosting)
	s.reg = setupRegistry(c, true, false)
	s.d = NewDaemon(c)
}

func (s *DockerSchema1RegistrySuite) TearDownTest(c *check.C) {
	if s.reg != nil {
		s.reg.Close()
	}
	if s.d != nil {
		s.d.Stop()
	}
	s.ds.TearDownTest(c)
}

func init() {
	check.Suite(&DockerRegistryAuthSuite{
		ds: &DockerSuite{},
	})
}

type DockerRegistryAuthSuite struct {
	ds  *DockerSuite
	reg *testRegistryV2
	d   *Daemon
}

func (s *DockerRegistryAuthSuite) SetUpTest(c *check.C) {
	testRequires(c, DaemonIsLinux, RegistryHosting)
	s.reg = setupRegistry(c, false, true)
	s.d = NewDaemon(c)
}

func (s *DockerRegistryAuthSuite) TearDownTest(c *check.C) {
	if s.reg != nil {
		out, err := s.d.Cmd("logout", privateRegistryURL)
		c.Assert(err, check.IsNil, check.Commentf(out))
		s.reg.Close()
	}
	if s.d != nil {
		s.d.Stop()
	}
	s.ds.TearDownTest(c)
}

func init() {
	check.Suite(&DockerDaemonSuite{
		ds: &DockerSuite{},
	})
}

type DockerDaemonSuite struct {
	ds *DockerSuite
	d  *Daemon
}

func (s *DockerDaemonSuite) SetUpTest(c *check.C) {
	testRequires(c, DaemonIsLinux)
	s.d = NewDaemon(c)
}

func (s *DockerDaemonSuite) TearDownTest(c *check.C) {
	testRequires(c, DaemonIsLinux)
	if s.d != nil {
		s.d.Stop()
	}
	s.ds.TearDownTest(c)
}

func init() {
	check.Suite(&DockerTrustSuite{
		ds: &DockerSuite{},
	})
}

type DockerTrustSuite struct {
	ds  *DockerSuite
	reg *testRegistryV2
	not *testNotary
}

func (s *DockerTrustSuite) SetUpTest(c *check.C) {
	testRequires(c, RegistryHosting, NotaryHosting)
	s.reg = setupRegistry(c, false, false)
	s.not = setupNotary(c)
}

func (s *DockerTrustSuite) TearDownTest(c *check.C) {
	if s.reg != nil {
		s.reg.Close()
	}
	if s.not != nil {
		s.not.Close()
	}
	s.ds.TearDownTest(c)
}

func init() {
	check.Suite(&DockerAuthnSuite{
		ds: &DockerDaemonSuite{
			ds: &DockerSuite{},
		},
		daemonAddr: "localhost:4271",
	})
}

type DockerAuthnSuite struct {
	server     *httptest.Server
	ds         *DockerDaemonSuite
	krb5       *Krb5Env
	basic      *BasicEnv
	daemonAddr string
}

func (s *DockerAuthnSuite) SetUpSuite(c *check.C) {
	testRequires(c, UnixCli, SameHostDaemon, NeedsGSSAPI, NeedsLibSASL)
	mux := http.NewServeMux()
	s.server = httptest.NewServer(mux)
	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		json.NewEncoder(w).Encode(plugins.Manifest{Implements: []string{"ClientCertificateMapper", "Authentication", authorization.AuthZApiImplements}})
	})
	mux.HandleFunc("/ClientCertificateMapper.MapClientCertificateToUser", s.MapClientCertificateToUser)
	mux.HandleFunc("/Authentication.GetChallenge", s.GetChallenge)
	mux.HandleFunc("/Authentication.CheckResponse", s.CheckResponse)
	mux.HandleFunc("/AuthZPlugin.AuthZReq", s.AuthzRequest)
	mux.HandleFunc("/AuthZPlugin.AuthZRes", s.AuthzResponse)

	if err := os.MkdirAll("/etc/docker/plugins", 0755); err != nil {
		c.Fatal(err)
	}
	if err := ioutil.WriteFile("/etc/docker/plugins/test-authn-certmap.spec", []byte(s.server.URL), 0644); err != nil {
		c.Fatal(err)
	}
	if err := ioutil.WriteFile("/etc/docker/plugins/test-authn-plugin.spec", []byte(s.server.URL), 0644); err != nil {
		c.Fatal(err)
	}
	if err := ioutil.WriteFile("/etc/docker/plugins/test-authz-plugin.spec", []byte(s.server.URL), 0644); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerAuthnSuite) TearDownSuite(c *check.C) {
	if s.server != nil {
		s.server.Close()
	}
	if err := os.RemoveAll("/etc/docker/plugins"); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerAuthnSuite) SetUpTest(c *check.C) {
	s.ds.SetUpTest(c)
	s.krb5 = NewKrb5Env()
	s.basic = NewBasicEnv()
}

func (s *DockerAuthnSuite) TearDownTest(c *check.C) {
	if s.basic != nil {
		s.basic.Stop(c)
	}
	if s.krb5 != nil {
		s.krb5.Stop(c)
	}
	s.ds.TearDownTest(c)
}
