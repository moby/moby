//go:build (linux || windows) && !no_embedded_containerd

package embedded

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/plugin"
	"github.com/containerd/ttrpc"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type recordingService struct {
	id                 string
	grpcRegistrations  *[]string
	ttrpcRegistrations *[]string
	closed             *[]string
	registerErr        error
}

func (s *recordingService) Register(*grpc.Server) error {
	if s.grpcRegistrations != nil {
		*s.grpcRegistrations = append(*s.grpcRegistrations, s.id)
	}
	return s.registerErr
}

func (s *recordingService) RegisterTTRPC(*ttrpc.Server) error {
	if s.ttrpcRegistrations != nil {
		*s.ttrpcRegistrations = append(*s.ttrpcRegistrations, s.id)
	}
	return nil
}

func (s *recordingService) Close() error {
	if s.closed != nil {
		*s.closed = append(*s.closed, s.id)
	}
	return nil
}

func TestContainerdServerPluginLifecycle(t *testing.T) {
	var initialized, grpcRegistrations, ttrpcRegistrations, closed []string
	cfg := &serverConfig{
		root:         filepath.Join(t.TempDir(), "root"),
		state:        filepath.Join(t.TempDir(), "state"),
		grpcAddress:  "grpc-address",
		ttrpcAddress: "ttrpc-address",
	}

	const pluginType = plugin.Type("io.moby.test.v1")
	newRegistration := func(id string) plugin.Registration {
		pluginConfig := &struct{ ID string }{ID: id}
		return plugin.Registration{
			Type:   pluginType,
			ID:     id,
			Config: pluginConfig,
			InitFn: func(initContext *plugin.InitContext) (any, error) {
				uri := string(pluginType) + "." + id
				assert.Check(t, is.Equal(initContext.Properties[plugins.PropertyRootDir], filepath.Join(cfg.root, uri)))
				assert.Check(t, is.Equal(initContext.Properties[plugins.PropertyStateDir], filepath.Join(cfg.state, uri)))
				assert.Check(t, is.Equal(initContext.Properties[plugins.PropertyGRPCAddress], cfg.grpcAddress))
				assert.Check(t, is.Equal(initContext.Properties[plugins.PropertyTTRPCAddress], cfg.ttrpcAddress))
				assert.Check(t, is.Equal(initContext.Config, pluginConfig))
				ready := initContext.RegisterReadiness()
				ready()
				initialized = append(initialized, id)
				return &recordingService{
					id:                 id,
					grpcRegistrations:  &grpcRegistrations,
					ttrpcRegistrations: &ttrpcRegistrations,
					closed:             &closed,
				}, nil
			},
		}
	}
	registrations := []plugin.Registration{
		newRegistration("first"),
		newRegistration("second"),
	}
	server, err := newServerWithRegistrations(t.Context(), cfg, registrations)
	assert.NilError(t, err)
	server.Wait()
	assert.Check(t, is.DeepEqual(initialized, []string{"first", "second"}))
	assert.Check(t, is.DeepEqual(grpcRegistrations, []string{"first", "second"}))
	assert.Check(t, is.DeepEqual(ttrpcRegistrations, []string{"first", "second"}))

	server.Stop()
	server.Stop()
	assert.Check(t, is.DeepEqual(closed, []string{"second", "first"}))
}

func TestContainerdServerClosesPluginsAfterRegistrationFailure(t *testing.T) {
	var closed []string
	cfg := &serverConfig{
		root:  t.TempDir(),
		state: t.TempDir(),
	}
	registrations := []plugin.Registration{
		{
			Type: "io.moby.test.v1",
			ID:   "loaded",
			InitFn: func(*plugin.InitContext) (any, error) {
				return &recordingService{id: "loaded", closed: &closed}, nil
			},
		},
		{
			Type: "io.moby.test.v1",
			ID:   "failing",
			InitFn: func(*plugin.InitContext) (any, error) {
				return &recordingService{
					id:          "failing",
					closed:      &closed,
					registerErr: errors.New("registration failed"),
				}, nil
			},
		},
	}

	server, err := newServerWithRegistrations(t.Context(), cfg, registrations)
	assert.Check(t, is.Nil(server))
	assert.Check(t, is.ErrorContains(err, "registration failed"))
	assert.Check(t, is.DeepEqual(closed, []string{"failing", "loaded"}))
}

func TestContainerdServerStopCancelsTTRPCRequests(t *testing.T) {
	ttrpcServer, err := ttrpc.NewServer()
	assert.NilError(t, err)

	requestStarted := make(chan struct{})
	requestCanceled := make(chan struct{})
	ttrpcServer.RegisterService("test", &ttrpc.ServiceDesc{
		Methods: map[string]ttrpc.Method{
			"Wait": func(ctx context.Context, _ func(any) error) (any, error) {
				close(requestStarted)
				<-ctx.Done()
				close(requestCanceled)
				return nil, ctx.Err()
			},
		},
	})

	serveCtx, cancelServe := context.WithCancel(t.Context())
	server := &containerdServer{
		grpcServer:  grpc.NewServer(),
		ttrpcServer: ttrpcServer,
		serveCtx:    serveCtx,
		cancelServe: cancelServe,
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.ServeTTRPC(listener)
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	assert.NilError(t, err)
	client := ttrpc.NewClient(conn)
	t.Cleanup(func() { _ = client.Close() })
	callErr := make(chan error, 1)
	go func() {
		callErr <- client.Call(t.Context(), "test", "Wait", &emptypb.Empty{}, &emptypb.Empty{})
	}()

	deadline, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	select {
	case <-requestStarted:
	case <-deadline.Done():
		t.Fatal("timed out waiting for ttrpc request to start")
	}

	server.Stop()

	select {
	case <-requestCanceled:
	case <-deadline.Done():
		t.Fatal("timed out waiting for ttrpc request cancellation")
	}
	select {
	case err := <-serveErr:
		assert.NilError(t, err)
	case <-deadline.Done():
		t.Fatal("timed out waiting for ttrpc server to stop")
	}
	select {
	case <-callErr:
	case <-deadline.Done():
		t.Fatal("timed out waiting for ttrpc client call to return")
	}
}
