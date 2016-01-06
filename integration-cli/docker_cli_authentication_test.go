package main

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"github.com/docker/docker/pkg/authentication"
	"github.com/docker/docker/pkg/authorization"
	"github.com/docker/docker/pkg/integration/cmd"
	"github.com/docker/docker/pkg/plugins"
	"github.com/go-check/check"
)

type DockerAuthnSuite struct {
	ds                              *DockerDaemonSuite
	daemonAddr                      string
	basic, bearer, proxy, authz     *httptest.Server
	user, pass                      string
	goodtoken, badtoken, othertoken string
	authzTokens                     map[string]string
	proxyHeaderName                 string
	proxyHeaderNameOption           string
	options                         map[string]string
}

func (s *DockerAuthnSuite) SetUpSuite(c *check.C) {
	testRequires(c, UnixCli, SameHostDaemon)

	mux := http.NewServeMux()
	s.basic = httptest.NewServer(mux)
	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		json.NewEncoder(w).Encode(plugins.Manifest{Implements: []string{authentication.PluginImplements}})
	})
	mux.HandleFunc("/"+authentication.AuthenticationRequestName, s.AuthenticateBasic)
	mux.HandleFunc("/"+authentication.SetOptionsRequestName, s.SetOptions)

	mux = http.NewServeMux()
	s.bearer = httptest.NewServer(mux)
	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		json.NewEncoder(w).Encode(plugins.Manifest{Implements: []string{authentication.PluginImplements}})
	})
	mux.HandleFunc("/"+authentication.AuthenticationRequestName, s.AuthenticateBearer)
	mux.HandleFunc("/"+authentication.SetOptionsRequestName, s.SetOptions)

	mux = http.NewServeMux()
	s.proxy = httptest.NewServer(mux)
	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		json.NewEncoder(w).Encode(plugins.Manifest{Implements: []string{authentication.PluginImplements}})
	})
	mux.HandleFunc("/"+authentication.AuthenticationRequestName, s.AuthenticateProxy)
	mux.HandleFunc("/"+authentication.SetOptionsRequestName, s.SetOptions)

	mux = http.NewServeMux()
	s.authz = httptest.NewServer(mux)
	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		json.NewEncoder(w).Encode(plugins.Manifest{Implements: []string{authorization.AuthZApiImplements}})
	})
	mux.HandleFunc("/AuthZPlugin.AuthZReq", s.AuthzRequest)
	mux.HandleFunc("/AuthZPlugin.AuthZRes", s.AuthzResponse)

	if err := os.MkdirAll("/etc/docker/plugins", 0755); err != nil {
		c.Fatal(err)
	}
	if err := ioutil.WriteFile("/etc/docker/plugins/test-basic-authn-plugin.spec", []byte(s.basic.URL), 0644); err != nil {
		c.Fatal(err)
	}
	if err := ioutil.WriteFile("/etc/docker/plugins/test-bearer-authn-plugin.spec", []byte(s.bearer.URL), 0644); err != nil {
		c.Fatal(err)
	}
	if err := ioutil.WriteFile("/etc/docker/plugins/test-proxy-authn-plugin.spec", []byte(s.proxy.URL), 0644); err != nil {
		c.Fatal(err)
	}
	if err := ioutil.WriteFile("/etc/docker/plugins/test-authz-plugin.spec", []byte(s.authz.URL), 0644); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerAuthnSuite) TearDownSuite(c *check.C) {
	os.Remove("/etc/docker/plugins/test-basic-authn-plugin.spec")
	os.Remove("/etc/docker/plugins/test-bearer-authn-plugin.spec")
	os.Remove("/etc/docker/plugins/test-proxy-authn-plugin.spec")
	os.Remove("/etc/docker/plugins/test-authz-plugin.spec")
	if s.basic != nil {
		s.basic.Close()
	}
	if s.bearer != nil {
		s.bearer.Close()
	}
	if s.proxy != nil {
		s.proxy.Close()
	}
	if s.authz != nil {
		s.authz.Close()
	}
	if err := os.RemoveAll("/etc/docker/plugins"); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerAuthnSuite) SetUpTest(c *check.C) {
	s.ds.SetUpTest(c)
}

func (s *DockerAuthnSuite) TearDownTest(c *check.C) {
	s.ds.TearDownTest(c)
}

func (s *DockerAuthnSuite) SetOptions(w http.ResponseWriter, r *http.Request) {
	req := authentication.AuthnPluginSetOptionsRequest{Options: make(map[string]string)}
	resp := authentication.AuthnPluginSetOptionsResponse{}
	w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		s.ds.d.c.Fatalf("Error parsing Authentication.SetOptions request from docker daemon: %v", err)
		return
	}
	s.options = req.Options
	json.NewEncoder(w).Encode(resp)
}

func (s *DockerAuthnSuite) AuthenticateBasic(w http.ResponseWriter, r *http.Request) {
	req := authentication.AuthnPluginAuthenticateRequest{}
	resp := authentication.AuthnPluginAuthenticateResponse{Header: make(http.Header)}
	w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		s.ds.d.c.Fatalf("Error parsing Authentication.Authenticate request from docker daemon: %v", err)
		return
	}
	if req.Method == "" {
		s.ds.d.c.Fatalf("Error parsing Authentication.Authenticate request from docker daemon: %#v is missing required data", req)
		return
	}
	if req.URL == "" {
		s.ds.d.c.Fatalf("Error parsing Authentication.Authenticate request from docker daemon: %#v is missing URL fields", req)
		return
	}
	if len(req.Header) < 1 {
		s.ds.d.c.Fatalf("Error parsing Authentication.Authenticate request from docker daemon: %#v is missing header fields", req)
		return
	}
	realm, ok := s.options["realm"]
	if !ok {
		realm = "localhost"
	}
	headers := req.Header[http.CanonicalHeaderKey("Authorization")]
	if len(headers) == 0 {
		resp.Header.Add("WWW-Authenticate", "Basic realm=\""+realm+"\"")
	} else {
		for _, h := range headers {
			fields := strings.SplitN(strings.Replace(h, "\t", " ", -1), " ", 2)
			if len(fields) < 2 || strings.ToLower(fields[0]) != "basic" {
				continue
			}
			token, err := base64.StdEncoding.DecodeString(fields[1])
			if err != nil {
				continue
			}
			basic := strings.SplitN(string(token), ":", 2)
			if len(basic) < 2 {
				continue
			}
			if basic[0] == s.user && basic[1] == s.pass {
				resp.AuthedUser.Scheme = "Basic"
				resp.AuthedUser.Name = s.user
				break
			}
		}
	}
	json.NewEncoder(w).Encode(resp)
}

func (s *DockerAuthnSuite) AuthenticateBearer(w http.ResponseWriter, r *http.Request) {
	req := authentication.AuthnPluginAuthenticateRequest{}
	resp := authentication.AuthnPluginAuthenticateResponse{Header: make(http.Header)}
	w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		s.ds.d.c.Fatalf("Error parsing Authentication.Authenticate request from docker daemon: %v", err)
		return
	}
	headers := req.Header[http.CanonicalHeaderKey("Authorization")]
	if len(headers) == 0 {
		resp.Header.Add("WWW-Authenticate", "Bearer")
	} else {
		for _, h := range headers {
			fields := strings.SplitN(strings.Replace(h, "\t", " ", -1), " ", 2)
			if len(fields) < 2 || strings.ToLower(fields[0]) != "bearer" {
				continue
			}
			user, ok := s.authzTokens[fields[1]]
			if ok {
				resp.AuthedUser.Scheme = "Bearer"
				resp.AuthedUser.Name = user
			}
		}
	}
	json.NewEncoder(w).Encode(resp)
}

func (s *DockerAuthnSuite) AuthenticateProxy(w http.ResponseWriter, r *http.Request) {
	req := authentication.AuthnPluginAuthenticateRequest{}
	resp := authentication.AuthnPluginAuthenticateResponse{Header: make(http.Header)}
	w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		s.ds.d.c.Fatalf("Error parsing Authentication.Authenticate request from docker daemon: %v", err)
		return
	}
	proxyHeaderName := s.options[s.proxyHeaderNameOption]
	headers := req.Header[http.CanonicalHeaderKey(proxyHeaderName)]
	if len(headers) > 0 {
		resp.AuthedUser.Scheme = "Proxy"
		resp.AuthedUser.Name = headers[0]
	}
	json.NewEncoder(w).Encode(resp)
}

func (s *DockerAuthnSuite) AuthzRequest(w http.ResponseWriter, r *http.Request) {
	req := authorization.Request{}
	resp := authorization.Response{}
	w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		s.ds.d.c.Fatalf("Error parsing AuthZ.Req request from docker daemon: %v", err)
		return
	}
	resp.Allow = req.User == s.user
	if !resp.Allow {
		if req.User != "" {
			resp.Msg = "Authorization denied: not the user we were looking for."
		} else {
			resp.Msg = "Authorization denied: client not authenticated."
		}
	}
	json.NewEncoder(w).Encode(resp)
}

func (s *DockerAuthnSuite) AuthzResponse(w http.ResponseWriter, r *http.Request) {
	req := authorization.Request{}
	resp := authorization.Response{}
	w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		s.ds.d.c.Fatalf("Error parsing AuthZ.Rep request from docker daemon: %v", err)
		return
	}
	resp.Allow = req.User == s.user
	if !resp.Allow {
		if req.User != "" {
			resp.Msg = "Authorization denied: not the user we were looking for."
		} else {
			resp.Msg = "Authorization denied: client not authenticated."
		}
	}
	json.NewEncoder(w).Encode(resp)
}

func runCurlWithOutput(args ...string) (stdout, stderr string, exitCode int, err error) {
	result := cmd.RunCommand("curl", args...)
	return result.Stdout(), result.Stderr(), result.ExitCode, result.Error
}

func (s *DockerAuthnSuite) TestNoneAuthnBad(c *check.C) {
	serverOpts := []string{
		"--host", s.daemonAddr, "--authn",
		"--authn-opt", "plugins=test-basic-authn-plugin,test-bearer-authn-plugin",
		"--authn-opt", "realm=example.com",
	}
	if err := s.ds.d.Start(serverOpts...); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	stdout, stderr, code, err := runCurlWithOutput(s.daemonAddr+"/_ping", "-i")
	if code != 0 {
		c.Fatalf("Error Occurred: exit status %d: %s(%s)", code, stdout, stderr)
	}
	if err != nil {
		c.Fatalf("Error Occurred: %v and output: %s(%s)", err, stdout, stderr)
	}
	c.Assert(strings.Contains(stdout, "Authenticate: Basic"), check.Equals, true, check.Commentf("actual output is: %q (stderr is %q)", stdout, stderr))
	c.Assert(strings.Contains(stdout, "Authenticate: Bearer"), check.Equals, true, check.Commentf("actual output is: %q (stderr is %q)", stdout, stderr))
	c.Assert(strings.Contains(stdout, "unauthenticated request rejected"), check.Equals, true, check.Commentf("actual output is: %q (stderr is %q)", stdout, stderr))
}

func (s *DockerAuthnSuite) TestBasicAuthnGood(c *check.C) {
	serverOpts := []string{
		"--host", s.daemonAddr, "--authn",
		"--authn-opt", "plugins=test-basic-authn-plugin",
		"--authn-opt", "realm=example.com",
	}
	if err := s.ds.d.Start(serverOpts...); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	stdout, stderr, code, err := runCurlWithOutput(s.daemonAddr+"/_ping", "--basic", "-u", s.user+":"+s.pass)
	if code != 0 {
		c.Fatalf("Error Occurred: exit status %d: %s(%s)", code, stdout, stderr)
	}
	if err != nil {
		c.Fatalf("Error Occurred: %v and output: %s(%s)", err, stdout, stderr)
	}
	c.Assert(stdout == "OK", check.Equals, true, check.Commentf("actual output is: %q (stderr is %q)", stdout, stderr))
}

func (s *DockerAuthnSuite) TestBasicAuthnBad1(c *check.C) {
	serverOpts := []string{
		"--host", s.daemonAddr, "--authn",
		"--authn-opt", "plugins=test-basic-authn-plugin",
		"--authn-opt", "realm=example.com",
	}
	if err := s.ds.d.Start(serverOpts...); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	stdout, stderr, code, err := runCurlWithOutput(s.daemonAddr+"/_ping", "--basic", "-u", "not-"+s.user+":"+s.pass)
	if code != 0 {
		c.Fatalf("Error Occurred: exit status %d: %s(%s)", code, stdout, stderr)
	}
	if err != nil {
		c.Fatalf("Error Occurred: %v and output: %s(%s)", err, stdout, stderr)
	}
	c.Assert(strings.Contains(stdout, "authentication failed for request"), check.Equals, true, check.Commentf("actual output is: %q (stderr is %q)", stdout, stderr))
}

func (s *DockerAuthnSuite) TestBasicAuthnBad2(c *check.C) {
	serverOpts := []string{
		"--host", s.daemonAddr, "--authn",
		"--authn-opt", "plugins=test-basic-authn-plugin",
		"--authn-opt", "realm=example.com",
	}
	if err := s.ds.d.Start(serverOpts...); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	stdout, stderr, code, err := runCurlWithOutput(s.daemonAddr+"/_ping", "--basic", "-u", s.user+":not-"+s.pass)
	if code != 0 {
		c.Fatalf("Error Occurred: exit status %d: %s(%s)", code, stdout, stderr)
	}
	if err != nil {
		c.Fatalf("Error Occurred: %v and output: %s(%s)", err, stdout, stderr)
	}
	c.Assert(strings.Contains(stdout, "authentication failed for request"), check.Equals, true, check.Commentf("actual output is: %q (stderr is %q)", stdout, stderr))
}

func (s *DockerAuthnSuite) TestBearerAuthnGood1a(c *check.C) {
	serverOpts := []string{
		"--host", s.daemonAddr, "--authn",
		"--authn-opt", "plugins=test-bearer-authn-plugin",
	}
	if err := s.ds.d.Start(serverOpts...); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	stdout, stderr, code, err := runCurlWithOutput(s.daemonAddr+"/_ping", "-H", "Authorization: Bearer "+s.goodtoken)
	if code != 0 {
		c.Fatalf("Error Occurred: exit status %d: %s(%s)", code, stdout, stderr)
	}
	if err != nil {
		c.Fatalf("Error Occurred: %v and output: %s(%s)", err, stdout, stderr)
	}
	c.Assert(stdout == "OK", check.Equals, true, check.Commentf("actual output is: %q (stderr is %q)", stdout, stderr))
}

func (s *DockerAuthnSuite) TestBearerAuthnGood1b(c *check.C) {
	serverOpts := []string{
		"--host", s.daemonAddr, "--authn",
		"--authn-opt", "plugins=test-bearer-authn-plugin",
	}
	if err := s.ds.d.Start(serverOpts...); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	stdout, stderr, code, err := runCurlWithOutput(s.daemonAddr+"/_ping", "-H", "Authorization: Bearer "+s.othertoken)
	if code != 0 {
		c.Fatalf("Error Occurred: exit status %d: %s(%s)", code, stdout, stderr)
	}
	if err != nil {
		c.Fatalf("Error Occurred: %v and output: %s(%s)", err, stdout, stderr)
	}
	c.Assert(stdout == "OK", check.Equals, true, check.Commentf("actual output is: %q (stderr is %q)", stdout, stderr))
}

func (s *DockerAuthnSuite) TestBearerAuthnBad1(c *check.C) {
	serverOpts := []string{
		"--host", s.daemonAddr, "--authn",
		"--authn-opt", "plugins=test-bearer-authn-plugin",
	}
	if err := s.ds.d.Start(serverOpts...); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	stdout, stderr, code, err := runCurlWithOutput(s.daemonAddr+"/_ping", "-H", "Authorization: Bearer "+s.badtoken)
	if code != 0 {
		c.Fatalf("Error Occurred: exit status %d: %s(%s)", code, stdout, stderr)
	}
	if err != nil {
		c.Fatalf("Error Occurred: %v and output: %s(%s)", err, stdout, stderr)
	}
	c.Assert(strings.Contains(stdout, "authentication failed for request"), check.Equals, true, check.Commentf("actual output is: %q (stderr is %q)", stdout, stderr))
}

func (s *DockerAuthnSuite) TestBearerAuthnGood2(c *check.C) {
	serverOpts := []string{
		"--host", s.daemonAddr, "--authn",
		"--authn-opt", "plugins=test-bearer-authn-plugin",
		"--authorization-plugin", "test-authz-plugin",
	}
	if err := s.ds.d.Start(serverOpts...); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	stdout, stderr, code, err := runCurlWithOutput(s.daemonAddr+"/_ping", "-vv", "-H", "Authorization: Bearer "+s.goodtoken)
	if code != 0 {
		c.Fatalf("Error Occurred: exit status %d: %s(%s)", code, stdout, stderr)
	}
	if err != nil {
		c.Fatalf("Error Occurred: %v and output: %s(%s)", err, stdout, stderr)
	}
	c.Assert(stdout == "OK", check.Equals, true, check.Commentf("actual output is: %q (stderr is %q)", stdout, stderr))
}

func (s *DockerAuthnSuite) TestBearerAuthnBad2a(c *check.C) {
	serverOpts := []string{
		"--host", s.daemonAddr,
		"--authorization-plugin", "test-authz-plugin",
	}
	if err := s.ds.d.Start(serverOpts...); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	stdout, stderr, code, err := runCurlWithOutput(s.daemonAddr + "/_ping")
	if code != 0 {
		c.Fatalf("Error Occurred: exit status %d: %s(%s)", code, stdout, stderr)
	}
	if err != nil {
		c.Fatalf("Error Occurred: %v and output: %s(%s)", err, stdout, stderr)
	}
	c.Assert(strings.Contains(stdout, "authorization denied by plugin test-authz-plugin"), check.Equals, true, check.Commentf("actual output is: %q (stderr is %q)", stdout, stderr))
	c.Assert(strings.Contains(stdout, "client not authenticated"), check.Equals, true, check.Commentf("actual output is: %q (stderr is %q)", stdout, stderr))
}

func (s *DockerAuthnSuite) TestBearerAuthnBad2b(c *check.C) {
	serverOpts := []string{
		"--host", s.daemonAddr, "--authn",
		"--authn-opt", "plugins=test-bearer-authn-plugin",
		"--authorization-plugin", "test-authz-plugin",
	}
	if err := s.ds.d.Start(serverOpts...); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	stdout, stderr, code, err := runCurlWithOutput(s.daemonAddr+"/_ping", "-vv", "-H", "Authorization: Bearer "+s.othertoken)
	if code != 0 {
		c.Fatalf("Error Occurred: exit status %d: %s(%s)", code, stdout, stderr)
	}
	if err != nil {
		c.Fatalf("Error Occurred: %v and output: %s(%s)", err, stdout, stderr)
	}
	c.Assert(strings.Contains(stdout, "authorization denied by plugin test-authz-plugin"), check.Equals, true, check.Commentf("actual output is: %q (stderr is %q)", stdout, stderr))
	c.Assert(strings.Contains(stdout, "not the user we were looking for"), check.Equals, true, check.Commentf("actual output is: %q (stderr is %q)", stdout, stderr))
}

func (s *DockerAuthnSuite) TestBearerAuthnBad2c(c *check.C) {
	serverOpts := []string{
		"--host", s.daemonAddr, "--authn",
		"--authn-opt", "plugins=test-bearer-authn-plugin",
		"--authorization-plugin", "test-authz-plugin",
	}
	if err := s.ds.d.Start(serverOpts...); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	stdout, stderr, code, err := runCurlWithOutput(s.daemonAddr+"/_ping", "-vv", "-H", "Authorization: Bearer "+s.badtoken)
	if code != 0 {
		c.Fatalf("Error Occurred: exit status %d: %s(%s)", code, stdout, stderr)
	}
	if err != nil {
		c.Fatalf("Error Occurred: %v and output: %s(%s)", err, stdout, stderr)
	}
	c.Assert(strings.Contains(stdout, "authentication failed for request"), check.Equals, true, check.Commentf("actual output is: %q (stderr is %q)", stdout, stderr))
}

func (s *DockerAuthnSuite) TestProxyAuthnBad(c *check.C) {
	serverOpts := []string{
		"--host", s.daemonAddr, "--authn",
		"--authn-opt", "plugins=test-proxy-authn-plugin",
		"--authn-opt", s.proxyHeaderNameOption + "=" + s.proxyHeaderName,
	}
	if err := s.ds.d.Start(serverOpts...); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	stdout, stderr, code, err := runCurlWithOutput(s.daemonAddr+"/_ping", "-vv")
	if code != 0 {
		c.Fatalf("Error Occurred: exit status %d: %s(%s)", code, stdout, stderr)
	}
	if err != nil {
		c.Fatalf("Error Occurred: %v and output: %s(%s)", err, stdout, stderr)
	}
	c.Assert(strings.Contains(stdout, "unauthenticated request rejected"), check.Equals, true, check.Commentf("actual output is: %q (stderr is %q)", stdout, stderr))
}

func (s *DockerAuthnSuite) TestProxyAuthnGood(c *check.C) {
	serverOpts := []string{
		"--host", s.daemonAddr, "--authn",
		"--authn-opt", "plugins=test-proxy-authn-plugin",
		"--authn-opt", s.proxyHeaderNameOption + "=" + s.proxyHeaderName,
		"--authorization-plugin", "test-authz-plugin",
	}
	if err := s.ds.d.Start(serverOpts...); err != nil {
		c.Fatalf("Could not start daemon: %v", err)
	}
	stdout, stderr, code, err := runCurlWithOutput(s.daemonAddr+"/_ping", "-H", s.proxyHeaderName+": "+s.user, "-vv")
	if code != 0 {
		c.Fatalf("Error Occurred: exit status %d: %s(%s)", code, stdout, stderr)
	}
	if err != nil {
		c.Fatalf("Error Occurred: %v and output: %s(%s)", err, stdout, stderr)
	}
	c.Assert(stdout == "OK", check.Equals, true, check.Commentf("actual output is: %q (stderr is %q)", stdout, stderr))
}

func init() {
	check.Suite(&DockerAuthnSuite{
		ds: &DockerDaemonSuite{
			ds: &DockerSuite{},
		},
		daemonAddr: "localhost:4271",
		user:       "docker",
		pass:       "docker",
		goodtoken:  "YES-I-AM-A-BEAR",
		badtoken:   "NO-I-AM-NOT-A-BEAR",
		othertoken: "SO-MANY-BEARS",
		authzTokens: map[string]string{
			"YES-I-AM-A-BEAR": "docker",
			"SO-MANY-BEARS":   "otheruser",
		},
		proxyHeaderName:       "RemoteUser",
		proxyHeaderNameOption: "TrustedHeader",
	})
}
