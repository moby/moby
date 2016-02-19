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

	"bufio"
	"bytes"
	"os/exec"
	"strconv"
	"time"

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

var (
	alwaysAllowed = []string{"/_ping", "/info"}
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

		reqRes := s.ctrl.reqRes
		if isAllowed(authReq.RequestURI) {
			reqRes = authorization.Response{Allow: true}
		}
		if reqRes.Err != "" {
			w.WriteHeader(http.StatusInternalServerError)
		}
		b, err := json.Marshal(reqRes)
		c.Assert(err, check.IsNil)
		w.Write(b)
	})

	mux.HandleFunc("/AuthZPlugin.AuthZRes", func(w http.ResponseWriter, r *http.Request) {
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
		resRes := s.ctrl.resRes
		if isAllowed(authReq.RequestURI) {
			resRes = authorization.Response{Allow: true}
		}
		if resRes.Err != "" {
			w.WriteHeader(http.StatusInternalServerError)
		}
		b, err := json.Marshal(resRes)
		c.Assert(err, check.IsNil)
		w.Write(b)
	})

	err := os.MkdirAll("/etc/docker/plugins", 0755)
	c.Assert(err, checker.IsNil)

	fileName := fmt.Sprintf("/etc/docker/plugins/%s.spec", testAuthZPlugin)
	err = ioutil.WriteFile(fileName, []byte(s.server.URL), 0644)
	c.Assert(err, checker.IsNil)
}

// check for always allowed endpoints to not inhibit test framework functions
func isAllowed(reqURI string) bool {
	for _, endpoint := range alwaysAllowed {
		if strings.HasSuffix(reqURI, endpoint) {
			return true
		}
	}
	return false
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
	// start the daemon and load busybox, --net=none build fails otherwise
	// cause it needs to pull busybox
	c.Assert(s.d.Start("--authorization-plugin="+testAuthZPlugin), check.IsNil)
	s.ctrl.reqRes.Allow = true
	s.ctrl.resRes.Allow = true
	c.Assert(s.d.LoadBusybox(), check.IsNil)

	// Ensure command successful
	out, err := s.d.Cmd("run", "-d", "busybox", "top")
	c.Assert(err, check.IsNil)

	id := strings.TrimSpace(out)
	assertURIRecorded(c, s.ctrl.requestsURIs, "/containers/create")
	assertURIRecorded(c, s.ctrl.requestsURIs, fmt.Sprintf("/containers/%s/start", id))

	out, err = s.d.Cmd("ps")
	c.Assert(err, check.IsNil)
	c.Assert(assertContainerList(out, []string{id}), check.Equals, true)
	c.Assert(s.ctrl.psRequestCnt, check.Equals, 1)
	c.Assert(s.ctrl.psResponseCnt, check.Equals, 1)
}

func (s *DockerAuthzSuite) TestAuthZPluginDenyRequest(c *check.C) {
	err := s.d.Start("--authorization-plugin=" + testAuthZPlugin)
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
	err := s.d.Start("--authorization-plugin=" + testAuthZPlugin)
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

// TestAuthZPluginAllowEventStream verifies event stream propagates correctly after request pass through by the authorization plugin
func (s *DockerAuthzSuite) TestAuthZPluginAllowEventStream(c *check.C) {
	testRequires(c, DaemonIsLinux)

	// start the daemon and load busybox to avoid pulling busybox from Docker Hub
	c.Assert(s.d.Start("--authorization-plugin="+testAuthZPlugin), check.IsNil)
	s.ctrl.reqRes.Allow = true
	s.ctrl.resRes.Allow = true
	c.Assert(s.d.LoadBusybox(), check.IsNil)

	startTime := strconv.FormatInt(daemonTime(c).Unix(), 10)
	// Add another command to to enable event pipelining
	eventsCmd := exec.Command(s.d.cmd.Path, "--host", s.d.sock(), "events", "--since", startTime)
	stdout, err := eventsCmd.StdoutPipe()
	if err != nil {
		c.Assert(err, check.IsNil)
	}

	observer := eventObserver{
		buffer:    new(bytes.Buffer),
		command:   eventsCmd,
		scanner:   bufio.NewScanner(stdout),
		startTime: startTime,
	}

	err = observer.Start()
	c.Assert(err, checker.IsNil)
	defer observer.Stop()

	// Create a container and wait for the creation events
	out, err := s.d.Cmd("run", "-d", "busybox", "top")
	c.Assert(err, check.IsNil, check.Commentf(out))
	containerID := strings.TrimSpace(out)
	c.Assert(s.d.waitRun(containerID), checker.IsNil)

	events := map[string]chan bool{
		"create": make(chan bool),
		"start":  make(chan bool),
	}

	matcher := matchEventLine(containerID, "container", events)
	processor := processEventMatch(events)
	go observer.Match(matcher, processor)

	// Ensure all events are received
	for event, eventChannel := range events {

		select {
		case <-time.After(5 * time.Second):
			// Fail the test
			observer.CheckEventError(c, containerID, event, matcher)
			c.FailNow()
		case <-eventChannel:
			// Ignore, event received
		}
	}

	// Ensure both events and container endpoints are passed to the authorization plugin
	assertURIRecorded(c, s.ctrl.requestsURIs, "/events")
	assertURIRecorded(c, s.ctrl.requestsURIs, "/containers/create")
	assertURIRecorded(c, s.ctrl.requestsURIs, fmt.Sprintf("/containers/%s/start", containerID))
}

func (s *DockerAuthzSuite) TestAuthZPluginErrorResponse(c *check.C) {
	err := s.d.Start("--authorization-plugin=" + testAuthZPlugin)
	c.Assert(err, check.IsNil)
	s.ctrl.reqRes.Allow = true
	s.ctrl.resRes.Err = errorMessage

	// Ensure command is blocked
	res, err := s.d.Cmd("ps")
	c.Assert(err, check.NotNil)

	c.Assert(res, check.Equals, fmt.Sprintf("Error response from daemon: plugin %s failed with error: %s: %s\n", testAuthZPlugin, authorization.AuthZApiResponse, errorMessage))
}

func (s *DockerAuthzSuite) TestAuthZPluginErrorRequest(c *check.C) {
	err := s.d.Start("--authorization-plugin=" + testAuthZPlugin)
	c.Assert(err, check.IsNil)
	s.ctrl.reqRes.Err = errorMessage

	// Ensure command is blocked
	res, err := s.d.Cmd("ps")
	c.Assert(err, check.NotNil)

	c.Assert(res, check.Equals, fmt.Sprintf("Error response from daemon: plugin %s failed with error: %s: %s\n", testAuthZPlugin, authorization.AuthZApiRequest, errorMessage))
}

func (s *DockerAuthzSuite) TestAuthZPluginEnsureNoDuplicatePluginRegistration(c *check.C) {
	c.Assert(s.d.Start("--authorization-plugin="+testAuthZPlugin, "--authorization-plugin="+testAuthZPlugin), check.IsNil)

	s.ctrl.reqRes.Allow = true
	s.ctrl.resRes.Allow = true

	out, err := s.d.Cmd("ps")
	c.Assert(err, check.IsNil, check.Commentf(out))

	// assert plugin is only called once..
	c.Assert(s.ctrl.psRequestCnt, check.Equals, 1)
	c.Assert(s.ctrl.psResponseCnt, check.Equals, 1)
}

// assertURIRecorded verifies that the given URI was sent and recorded in the authz plugin
func assertURIRecorded(c *check.C, uris []string, uri string) {
	var found bool
	for _, u := range uris {
		if strings.Contains(u, uri) {
			found = true
			break
		}
	}
	if !found {
		c.Fatalf("Expected to find URI '%s', recorded uris '%s'", uri, strings.Join(uris, ","))
	}
}
