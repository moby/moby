// +build !windows

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"github.com/docker/docker/pkg/authorization"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/pkg/plugins"
	"github.com/go-check/check"
)

const (
	testAuthZPlugin     = "authzplugin"
	unauthorizedMessage = "User unauthorized authz plugin"
	errorMessage        = "something went wrong..."
	containerListAPI    = "/containers/json"
)

func init() {
	check.Suite(&DockerAuthzSuite{
		ds: &DockerSuite{},
	})
}

type DockerAuthzSuite struct {
	server *httptest.Server
	ds     *DockerSuite
	d      *Daemon
	ctrl   *authorizationController
}

type authorizationController struct {
	reqRes        authorization.Response // reqRes holds the plugin response to the initial client request
	resRes        authorization.Response // resRes holds the plugin response to the daemon response
	psRequestCnt  int                    // psRequestCnt counts the number of calls to list container request api
	psResponseCnt int                    // psResponseCnt counts the number of calls to list containers response API
	requestsURIs  []string               // requestsURIs stores all request URIs that are sent to the authorization controller
}

func (s *DockerAuthzSuite) SetUpTest(c *check.C) {
	s.d = NewDaemon(c)
	s.ctrl = &authorizationController{}
}

func (s *DockerAuthzSuite) TearDownTest(c *check.C) {
	s.d.Stop()
	s.ds.TearDownTest(c)
	s.ctrl = nil
}

func (s *DockerAuthzSuite) SetUpSuite(c *check.C) {
	mux := http.NewServeMux()
	s.server = httptest.NewServer(mux)
	c.Assert(s.server, check.NotNil, check.Commentf("Failed to start a HTTP Server"))

	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		b, err := json.Marshal(plugins.Manifest{Implements: []string{authorization.AuthZApiImplements}})
		c.Assert(err, check.IsNil)
		w.Write(b)
	})

	mux.HandleFunc("/AuthZPlugin.AuthZReq", func(w http.ResponseWriter, r *http.Request) {
		if s.ctrl.reqRes.Err != "" {
			w.WriteHeader(http.StatusInternalServerError)
		}
		b, err := json.Marshal(s.ctrl.reqRes)
		c.Assert(err, check.IsNil)
		w.Write(b)
		defer r.Body.Close()
		body, err := ioutil.ReadAll(r.Body)
		c.Assert(err, check.IsNil)
		authReq := authorization.Request{}
		err = json.Unmarshal(body, &authReq)
		c.Assert(err, check.IsNil)

		assertBody(c, authReq.RequestURI, authReq.RequestHeaders, authReq.RequestBody)
		assertAuthHeaders(c, authReq.RequestHeaders)

		// Count only container list api
		if strings.HasSuffix(authReq.RequestURI, containerListAPI) {
			s.ctrl.psRequestCnt++
		}

		s.ctrl.requestsURIs = append(s.ctrl.requestsURIs, authReq.RequestURI)
	})

	mux.HandleFunc("/AuthZPlugin.AuthZRes", func(w http.ResponseWriter, r *http.Request) {
		if s.ctrl.resRes.Err != "" {
			w.WriteHeader(http.StatusInternalServerError)
		}
		b, err := json.Marshal(s.ctrl.resRes)
		c.Assert(err, check.IsNil)
		w.Write(b)

		defer r.Body.Close()
		body, err := ioutil.ReadAll(r.Body)
		c.Assert(err, check.IsNil)
		authReq := authorization.Request{}
		err = json.Unmarshal(body, &authReq)
		c.Assert(err, check.IsNil)

		assertBody(c, authReq.RequestURI, authReq.ResponseHeaders, authReq.ResponseBody)
		assertAuthHeaders(c, authReq.ResponseHeaders)

		// Count only container list api
		if strings.HasSuffix(authReq.RequestURI, containerListAPI) {
			s.ctrl.psResponseCnt++
		}
	})

	err := os.MkdirAll("/etc/docker/plugins", 0755)
	c.Assert(err, checker.IsNil)

	fileName := fmt.Sprintf("/etc/docker/plugins/%s.spec", testAuthZPlugin)
	err = ioutil.WriteFile(fileName, []byte(s.server.URL), 0644)
	c.Assert(err, checker.IsNil)
}

// assertAuthHeaders validates authentication headers are removed
func assertAuthHeaders(c *check.C, headers map[string]string) error {
	for k := range headers {
		if strings.Contains(strings.ToLower(k), "auth") || strings.Contains(strings.ToLower(k), "x-registry") {
			c.Errorf("Found authentication headers in request '%v'", headers)
		}
	}
	return nil
}

// assertBody asserts that body is removed for non text/json requests
func assertBody(c *check.C, requestURI string, headers map[string]string, body []byte) {

	if strings.Contains(strings.ToLower(requestURI), "auth") && len(body) > 0 {
		//return fmt.Errorf("Body included for authentication endpoint %s", string(body))
		c.Errorf("Body included for authentication endpoint %s", string(body))
	}

	for k, v := range headers {
		if strings.EqualFold(k, "Content-Type") && strings.HasPrefix(v, "text/") || v == "application/json" {
			return
		}
	}
	if len(body) > 0 {
		c.Errorf("Body included while it should not (Headers: '%v')", headers)
	}
}

func (s *DockerAuthzSuite) TearDownSuite(c *check.C) {
	if s.server == nil {
		return
	}

	s.server.Close()

	err := os.RemoveAll("/etc/docker/plugins")
	c.Assert(err, checker.IsNil)
}

func (s *DockerAuthzSuite) TestAuthZPluginAllowRequest(c *check.C) {
	err := s.d.Start("--authz-plugin=" + testAuthZPlugin)
	c.Assert(err, check.IsNil)
	s.ctrl.reqRes.Allow = true
	s.ctrl.resRes.Allow = true

	// Ensure command successful
	out, err := s.d.Cmd("run", "-d", "--name", "container1", "busybox:latest", "top")
	c.Assert(err, check.IsNil)

	// Extract the id of the created container
	res := strings.Split(strings.TrimSpace(out), "\n")
	id := res[len(res)-1]
	assertURIRecorded(c, s.ctrl.requestsURIs, "/containers/create")
	assertURIRecorded(c, s.ctrl.requestsURIs, fmt.Sprintf("/containers/%s/start", id))

	out, err = s.d.Cmd("ps")
	c.Assert(err, check.IsNil)
	c.Assert(assertContainerList(out, []string{id}), check.Equals, true)
	c.Assert(s.ctrl.psRequestCnt, check.Equals, 1)
	c.Assert(s.ctrl.psResponseCnt, check.Equals, 1)
}

func (s *DockerAuthzSuite) TestAuthZPluginDenyRequest(c *check.C) {
	err := s.d.Start("--authz-plugin=" + testAuthZPlugin)
	c.Assert(err, check.IsNil)
	s.ctrl.reqRes.Allow = false
	s.ctrl.reqRes.Msg = unauthorizedMessage

	// Ensure command is blocked
	res, err := s.d.Cmd("ps")
	c.Assert(err, check.NotNil)
	c.Assert(s.ctrl.psRequestCnt, check.Equals, 1)
	c.Assert(s.ctrl.psResponseCnt, check.Equals, 0)

	// Ensure unauthorized message appears in response
	c.Assert(res, check.Equals, fmt.Sprintf("Error response from daemon: authorization denied by plugin %s: %s\n", testAuthZPlugin, unauthorizedMessage))
}

func (s *DockerAuthzSuite) TestAuthZPluginDenyResponse(c *check.C) {
	err := s.d.Start("--authz-plugin=" + testAuthZPlugin)
	c.Assert(err, check.IsNil)
	s.ctrl.reqRes.Allow = true
	s.ctrl.resRes.Allow = false
	s.ctrl.resRes.Msg = unauthorizedMessage

	// Ensure command is blocked
	res, err := s.d.Cmd("ps")
	c.Assert(err, check.NotNil)
	c.Assert(s.ctrl.psRequestCnt, check.Equals, 1)
	c.Assert(s.ctrl.psResponseCnt, check.Equals, 1)

	// Ensure unauthorized message appears in response
	c.Assert(res, check.Equals, fmt.Sprintf("Error response from daemon: authorization denied by plugin %s: %s\n", testAuthZPlugin, unauthorizedMessage))
}

func (s *DockerAuthzSuite) TestAuthZPluginErrorResponse(c *check.C) {
	err := s.d.Start("--authz-plugin=" + testAuthZPlugin)
	c.Assert(err, check.IsNil)
	s.ctrl.reqRes.Allow = true
	s.ctrl.resRes.Err = errorMessage

	// Ensure command is blocked
	res, err := s.d.Cmd("ps")
	c.Assert(err, check.NotNil)

	c.Assert(res, check.Equals, fmt.Sprintf("Error response from daemon: plugin %s failed with error: %s: %s\n", testAuthZPlugin, authorization.AuthZApiResponse, errorMessage))
}

func (s *DockerAuthzSuite) TestAuthZPluginErrorRequest(c *check.C) {
	err := s.d.Start("--authz-plugin=" + testAuthZPlugin)
	c.Assert(err, check.IsNil)
	s.ctrl.reqRes.Err = errorMessage

	// Ensure command is blocked
	res, err := s.d.Cmd("ps")
	c.Assert(err, check.NotNil)

	c.Assert(res, check.Equals, fmt.Sprintf("Error response from daemon: plugin %s failed with error: %s: %s\n", testAuthZPlugin, authorization.AuthZApiRequest, errorMessage))
}

// assertURIRecorded verifies that the given URI was sent and recorded in the authz plugin
func assertURIRecorded(c *check.C, uris []string, uri string) {

	found := false
	for _, u := range uris {
		if strings.Contains(u, uri) {
			found = true
		}
	}
	if !found {
		c.Fatalf("Expected to find URI '%s', recorded uris '%s'", uri, strings.Join(uris, ","))
	}
}
