//go:build !windows

package authz // import "github.com/docker/docker/integration/plugin/authz"

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	eventtypes "github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/authorization"
	"github.com/docker/docker/testutil/environment"
	"github.com/docker/go-connections/sockets"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

const (
	testAuthZPlugin     = "authzplugin"
	unauthorizedMessage = "User unauthorized authz plugin"
	errorMessage        = "something went wrong..."
	serverVersionAPI    = "/version"
)

var (
	alwaysAllowed = []string{"/_ping", "/info"}
	ctrl          *authorizationController
)

type authorizationController struct {
	reqRes          authorization.Response // reqRes holds the plugin response to the initial client request
	resRes          authorization.Response // resRes holds the plugin response to the daemon response
	versionReqCount int                    // versionReqCount counts the number of requests to the server version API endpoint
	versionResCount int                    // versionResCount counts the number of responses from the server version API endpoint
	requestsURIs    []string               // requestsURIs stores all request URIs that are sent to the authorization controller
	reqUser         string
	resUser         string
}

func setupTestV1(t *testing.T) context.Context {
	ctx := setupTest(t)

	ctrl = &authorizationController{}

	err := os.MkdirAll("/etc/docker/plugins", 0o755)
	assert.NilError(t, err)

	fileName := fmt.Sprintf("/etc/docker/plugins/%s.spec", testAuthZPlugin)
	err = os.WriteFile(fileName, []byte(server.URL), 0o644)
	assert.NilError(t, err)

	t.Cleanup(func() {
		err := os.RemoveAll("/etc/docker/plugins")
		assert.NilError(t, err)
		ctrl = nil
	})
	return ctx
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

func socketHTTPClient(u *url.URL) (*http.Client, error) {
	transport := &http.Transport{}
	err := sockets.ConfigureTransport(transport, u.Scheme, u.Path)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Transport: transport,
	}, nil
}

func TestAuthZPluginAllowRequest(t *testing.T) {
	ctx := setupTestV1(t)

	ctrl.reqRes.Allow = true
	ctrl.resRes.Allow = true
	d.StartWithBusybox(ctx, t, "--authorization-plugin="+testAuthZPlugin)

	c := d.NewClientT(t)

	// Ensure command successful
	cID := container.Run(ctx, t, c)

	assertURIRecorded(t, ctrl.requestsURIs, "/containers/create")
	assertURIRecorded(t, ctrl.requestsURIs, fmt.Sprintf("/containers/%s/start", cID))

	_, err := c.ServerVersion(ctx)
	assert.NilError(t, err)
	assert.Equal(t, 1, ctrl.versionReqCount)
	assert.Equal(t, 1, ctrl.versionResCount)
}

func TestAuthZPluginTLS(t *testing.T) {
	ctx := setupTestV1(t)
	const (
		testDaemonHTTPSAddr = "tcp://localhost:4271"
		cacertPath          = "../../testdata/https/ca.pem"
		serverCertPath      = "../../testdata/https/server-cert.pem"
		serverKeyPath       = "../../testdata/https/server-key.pem"
		clientCertPath      = "../../testdata/https/client-cert.pem"
		clientKeyPath       = "../../testdata/https/client-key.pem"
	)

	d.Start(t,
		"--authorization-plugin="+testAuthZPlugin,
		"--tlsverify",
		"--tlscacert", cacertPath,
		"--tlscert", serverCertPath,
		"--tlskey", serverKeyPath,
		"-H", testDaemonHTTPSAddr)

	ctrl.reqRes.Allow = true
	ctrl.resRes.Allow = true

	c, err := newTLSAPIClient(testDaemonHTTPSAddr, cacertPath, clientCertPath, clientKeyPath)
	assert.NilError(t, err)

	_, err = c.ServerVersion(ctx)
	assert.NilError(t, err)

	assert.Equal(t, "client", ctrl.reqUser)
	assert.Equal(t, "client", ctrl.resUser)
}

func newTLSAPIClient(host, cacertPath, certPath, keyPath string) (client.APIClient, error) {
	dialer := &net.Dialer{
		KeepAlive: 30 * time.Second,
		Timeout:   30 * time.Second,
	}
	return client.NewClientWithOpts(
		client.WithTLSClientConfig(cacertPath, certPath, keyPath),
		client.WithDialContext(dialer.DialContext),
		client.WithHost(host))
}

func TestAuthZPluginDenyRequest(t *testing.T) {
	ctx := setupTestV1(t)

	d.Start(t, "--authorization-plugin="+testAuthZPlugin)
	ctrl.reqRes.Allow = false
	ctrl.reqRes.Msg = unauthorizedMessage

	c := d.NewClientT(t)

	// Ensure command is blocked
	_, err := c.ServerVersion(ctx)
	assert.Assert(t, err != nil)
	assert.Equal(t, 1, ctrl.versionReqCount)
	assert.Equal(t, 0, ctrl.versionResCount)

	// Ensure unauthorized message appears in response
	assert.Equal(t, fmt.Sprintf("Error response from daemon: authorization denied by plugin %s: %s", testAuthZPlugin, unauthorizedMessage), err.Error())
}

// TestAuthZPluginAPIDenyResponse validates that when authorization
// plugin deny the request, the status code is forbidden
func TestAuthZPluginAPIDenyResponse(t *testing.T) {
	ctx := setupTestV1(t)

	d.Start(t, "--authorization-plugin="+testAuthZPlugin)
	ctrl.reqRes.Allow = false
	ctrl.resRes.Msg = unauthorizedMessage

	daemonURL, err := url.Parse(d.Sock())
	assert.NilError(t, err)

	socketClient, err := socketHTTPClient(daemonURL)
	assert.NilError(t, err)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/version", nil)
	assert.NilError(t, err)
	req.URL.Scheme = "http"
	req.URL.Host = client.DummyHost

	resp, err := socketClient.Do(req)
	assert.NilError(t, err)

	assert.DeepEqual(t, http.StatusForbidden, resp.StatusCode)
}

func TestAuthZPluginDenyResponse(t *testing.T) {
	ctx := setupTestV1(t)

	d.Start(t, "--authorization-plugin="+testAuthZPlugin)
	ctrl.reqRes.Allow = true
	ctrl.resRes.Allow = false
	ctrl.resRes.Msg = unauthorizedMessage

	c := d.NewClientT(t)

	// Ensure command is blocked
	_, err := c.ServerVersion(ctx)
	assert.Assert(t, err != nil)
	assert.Equal(t, 1, ctrl.versionReqCount)
	assert.Equal(t, 1, ctrl.versionResCount)

	// Ensure unauthorized message appears in response
	assert.Equal(t, fmt.Sprintf("Error response from daemon: authorization denied by plugin %s: %s", testAuthZPlugin, unauthorizedMessage), err.Error())
}

// TestAuthZPluginAllowEventStream verifies event stream propagates
// correctly after request pass through by the authorization plugin
func TestAuthZPluginAllowEventStream(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTestV1(t)
	ctrl.reqRes.Allow = true
	ctrl.resRes.Allow = true
	d.StartWithBusybox(ctx, t, "--authorization-plugin="+testAuthZPlugin)

	c := d.NewClientT(t)

	startTime := strconv.FormatInt(systemTime(ctx, t, c, testEnv).Unix(), 10)
	events, errs, cancel := systemEventsSince(ctx, c, startTime)
	defer cancel()

	// Create a container and wait for the creation events
	cID := container.Run(ctx, t, c)

	created := false
	started := false
	for !created && !started {
		select {
		case event := <-events:
			if event.Type == eventtypes.ContainerEventType && event.Actor.ID == cID {
				if event.Action == eventtypes.ActionCreate {
					created = true
				}
				if event.Action == eventtypes.ActionStart {
					started = true
				}
			}
		case err := <-errs:
			if err == io.EOF {
				t.Fatal("premature end of event stream")
			}
			assert.NilError(t, err)
		case <-time.After(30 * time.Second):
			// Fail the test
			t.Fatal("event stream timeout")
		}
	}

	// Ensure both events and container endpoints are passed to the
	// authorization plugin
	assertURIRecorded(t, ctrl.requestsURIs, "/events")
	assertURIRecorded(t, ctrl.requestsURIs, "/containers/create")
	assertURIRecorded(t, ctrl.requestsURIs, fmt.Sprintf("/containers/%s/start", cID))
}

func systemTime(ctx context.Context, t *testing.T, client client.APIClient, testEnv *environment.Execution) time.Time {
	if testEnv.IsLocalDaemon() {
		return time.Now()
	}

	info, err := client.Info(ctx)
	assert.NilError(t, err)

	dt, err := time.Parse(time.RFC3339Nano, info.SystemTime)
	assert.NilError(t, err, "invalid time format in GET /info response")
	return dt
}

func systemEventsSince(ctx context.Context, client client.APIClient, since string) (<-chan eventtypes.Message, <-chan error, func()) {
	eventOptions := eventtypes.ListOptions{
		Since: since,
	}
	ctx, cancel := context.WithCancel(ctx)
	events, errs := client.Events(ctx, eventOptions)

	return events, errs, cancel
}

func TestAuthZPluginErrorResponse(t *testing.T) {
	ctx := setupTestV1(t)
	d.Start(t, "--authorization-plugin="+testAuthZPlugin)
	ctrl.reqRes.Allow = true
	ctrl.resRes.Err = errorMessage

	c := d.NewClientT(t)

	// Ensure command is blocked
	_, err := c.ServerVersion(ctx)
	assert.Assert(t, err != nil)
	assert.Equal(t, fmt.Sprintf("Error response from daemon: plugin %s failed with error: %s: %s", testAuthZPlugin, authorization.AuthZApiResponse, errorMessage), err.Error())
}

func TestAuthZPluginErrorRequest(t *testing.T) {
	ctx := setupTestV1(t)
	d.Start(t, "--authorization-plugin="+testAuthZPlugin)
	ctrl.reqRes.Err = errorMessage

	c := d.NewClientT(t)

	// Ensure command is blocked
	_, err := c.ServerVersion(ctx)
	assert.Assert(t, err != nil)
	assert.Equal(t, fmt.Sprintf("Error response from daemon: plugin %s failed with error: %s: %s", testAuthZPlugin, authorization.AuthZApiRequest, errorMessage), err.Error())
}

func TestAuthZPluginEnsureNoDuplicatePluginRegistration(t *testing.T) {
	ctx := setupTestV1(t)
	d.Start(t, "--authorization-plugin="+testAuthZPlugin, "--authorization-plugin="+testAuthZPlugin)

	ctrl.reqRes.Allow = true
	ctrl.resRes.Allow = true

	c := d.NewClientT(t)

	_, err := c.ServerVersion(ctx)
	assert.NilError(t, err)

	// assert plugin is only called once..
	assert.Equal(t, 1, ctrl.versionReqCount)
	assert.Equal(t, 1, ctrl.versionResCount)
}

func TestAuthZPluginEnsureLoadImportWorking(t *testing.T) {
	ctx := setupTestV1(t)

	ctrl.reqRes.Allow = true
	ctrl.resRes.Allow = true
	d.StartWithBusybox(ctx, t, "--authorization-plugin="+testAuthZPlugin, "--authorization-plugin="+testAuthZPlugin)

	c := d.NewClientT(t)

	tmp, err := os.MkdirTemp("", "test-authz-load-import")
	assert.NilError(t, err)
	defer os.RemoveAll(tmp)

	savedImagePath := filepath.Join(tmp, "save.tar")

	err = imageSave(ctx, c, savedImagePath, "busybox")
	assert.NilError(t, err)
	err = imageLoad(ctx, c, savedImagePath)
	assert.NilError(t, err)

	exportedImagePath := filepath.Join(tmp, "export.tar")

	cID := container.Run(ctx, t, c)

	responseReader, err := c.ContainerExport(ctx, cID)
	assert.NilError(t, err)
	defer responseReader.Close()
	file, err := os.Create(exportedImagePath)
	assert.NilError(t, err)
	defer file.Close()
	_, err = io.Copy(file, responseReader)
	assert.NilError(t, err)

	err = imageImport(ctx, c, exportedImagePath)
	assert.NilError(t, err)
}

func TestAuthzPluginEnsureContainerCopyToFrom(t *testing.T) {
	ctx := setupTestV1(t)
	ctrl.reqRes.Allow = true
	ctrl.resRes.Allow = true
	d.StartWithBusybox(ctx, t, "--authorization-plugin="+testAuthZPlugin, "--authorization-plugin="+testAuthZPlugin)

	dir, err := os.MkdirTemp("", t.Name())
	assert.NilError(t, err)
	defer os.RemoveAll(dir)

	f, err := os.CreateTemp(dir, "send")
	assert.NilError(t, err)
	defer f.Close()

	buf := make([]byte, 1024)
	fileSize := len(buf) * 1024 * 10
	for written := 0; written < fileSize; {
		n, err := f.Write(buf)
		assert.NilError(t, err)
		written += n
	}

	c := d.NewClientT(t)

	cID := container.Run(ctx, t, c)
	defer c.ContainerRemove(ctx, cID, containertypes.RemoveOptions{Force: true})

	_, err = f.Seek(0, io.SeekStart)
	assert.NilError(t, err)

	srcInfo, err := archive.CopyInfoSourcePath(f.Name(), false)
	assert.NilError(t, err)
	srcArchive, err := archive.TarResource(srcInfo)
	assert.NilError(t, err)
	defer srcArchive.Close()

	dstDir, preparedArchive, err := archive.PrepareArchiveCopy(srcArchive, srcInfo, archive.CopyInfo{Path: "/test"})
	assert.NilError(t, err)

	err = c.CopyToContainer(ctx, cID, dstDir, preparedArchive, containertypes.CopyToContainerOptions{})
	assert.NilError(t, err)

	rdr, _, err := c.CopyFromContainer(ctx, cID, "/test")
	assert.NilError(t, err)
	_, err = io.Copy(io.Discard, rdr)
	assert.NilError(t, err)
}

func imageSave(ctx context.Context, client client.APIClient, path, image string) error {
	responseReader, err := client.ImageSave(ctx, []string{image})
	if err != nil {
		return err
	}
	defer responseReader.Close()
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, responseReader)
	return err
}

func imageLoad(ctx context.Context, client client.APIClient, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	quiet := true
	response, err := client.ImageLoad(ctx, file, quiet)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return nil
}

func imageImport(ctx context.Context, client client.APIClient, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	options := image.ImportOptions{}
	ref := ""
	source := image.ImportSource{
		Source:     file,
		SourceName: "-",
	}
	responseReader, err := client.ImageImport(ctx, source, ref, options)
	if err != nil {
		return err
	}
	defer responseReader.Close()
	return nil
}

func TestAuthZPluginHeader(t *testing.T) {
	ctx := setupTestV1(t)

	ctrl.reqRes.Allow = true
	ctrl.resRes.Allow = true
	d.StartWithBusybox(ctx, t, "--debug", "--authorization-plugin="+testAuthZPlugin)

	daemonURL, err := url.Parse(d.Sock())
	assert.NilError(t, err)

	socketClient, err := socketHTTPClient(daemonURL)
	assert.NilError(t, err)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/version", nil)
	assert.NilError(t, err)
	req.URL.Scheme = "http"
	req.URL.Host = client.DummyHost

	resp, err := socketClient.Do(req)
	assert.NilError(t, err)
	assert.Equal(t, "application/json", resp.Header["Content-Type"][0])
}

// assertURIRecorded verifies that the given URI was sent and recorded
// in the authz plugin
func assertURIRecorded(t *testing.T, uris []string, uri string) {
	var found bool
	for _, u := range uris {
		if strings.Contains(u, uri) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Expected to find URI '%s', recorded uris '%s'", uri, strings.Join(uris, ","))
	}
}
