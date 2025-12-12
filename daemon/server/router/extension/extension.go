package extension

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containerd/log"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/moby/moby/v2/daemon/server/httputils"
	"github.com/moby/moby/v2/daemon/server/router"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	rpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
)

const extensionDir = "/var/lib/docker/extensions.d"

// LoadExtensionRouters scans the extension directory for Unix sockets,
// uses gRPC reflection to discover services, and creates routers for each service.
// If the socket is not serving gRPC, it falls back to a plain HTTP proxy route
// using the socket filename (without .sock extension) as the route prefix.
func LoadExtensionRouters(ctx context.Context) []router.Router {
	var routers []router.Router

	entries, err := os.ReadDir(extensionDir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.G(ctx).WithError(err).Warnf("Failed to read extension directory %s", extensionDir)
		}
		return routers
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sock") {
			continue
		}

		socketPath := filepath.Join(extensionDir, entry.Name())
		extensionName := strings.TrimSuffix(entry.Name(), ".sock")

		// Try to discover services via gRPC reflection
		services, isGRPC := discoverServices(ctx, socketPath)

		if !isGRPC {
			// Not a gRPC server, create a plain HTTP proxy route
			r := NewRouter(extensionName, socketPath)
			routers = append(routers, r)
			log.G(ctx).Infof("Loaded HTTP extension router for '%s' from %s", extensionName, socketPath)
			continue
		}

		if len(services) == 0 {
			log.G(ctx).Warnf("No services found for gRPC extension '%s' at %s", extensionName, socketPath)
			continue
		}

		// Create a router for each discovered gRPC service
		for _, serviceName := range services {
			// Skip the reflection service itself
			if serviceName == "grpc.reflection.v1alpha.ServerReflection" ||
				serviceName == "grpc.reflection.v1.ServerReflection" {
				continue
			}

			r := NewServiceRouter(serviceName, socketPath)
			routers = append(routers, r)
			log.G(ctx).Infof("Loaded gRPC extension router for service '%s' from %s", serviceName, socketPath)
		}
	}

	return routers
}

// discoverServices uses gRPC reflection to list all services exposed by the extension.
// Returns the list of services and a boolean indicating whether the target is a gRPC server.
// If the target is not a gRPC server (e.g., plain HTTP), returns (nil, false).
func discoverServices(ctx context.Context, socketPath string) ([]string, bool) {
	// Create a context with timeout for the gRPC connection attempt
	connCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Connect to the Unix socket via gRPC
	conn, err := grpc.NewClient(
		"unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.G(ctx).WithError(err).Debugf("Failed to create gRPC client for %s", socketPath)
		return nil, false
	}
	defer conn.Close()

	// Create a reflection client
	refClient := rpb.NewServerReflectionClient(conn)
	client := grpcreflect.NewClientV1Alpha(connCtx, refClient)
	defer client.Reset()

	// Try to list all services - this will fail if the server doesn't support gRPC reflection
	services, err := client.ListServices()
	if err != nil {
		// This likely means it's not a gRPC server or doesn't have reflection enabled
		log.G(ctx).WithError(err).Debugf("gRPC reflection not available for %s, treating as HTTP server", socketPath)
		return nil, false
	}

	return services, true
}

// NewServiceRouter creates a new extension router for a specific gRPC service
// that proxies requests to a Unix domain socket using httputil.ReverseProxy
func NewServiceRouter(serviceName, socketPath string) router.Router {
	r := &extensionRouter{
		serviceName: serviceName,
		socketPath:  socketPath,
	}
	r.initRoutes()
	return r
}

// NewRouter creates a new extension router that proxies requests to a Unix domain socket
// using the extension name as the route prefix. Used for plain HTTP extensions.
func NewRouter(extensionName, socketPath string) router.Router {
	r := &extensionRouter{
		serviceName: extensionName,
		socketPath:  socketPath,
	}
	r.initRoutes()
	return r
}

type extensionRouter struct {
	serviceName string
	socketPath  string
	routes      []router.Route
}

func (r *extensionRouter) initRoutes() {
	// Create a reverse proxy for this extension service
	// Route prefix is "/" + serviceName + "/"
	routePrefix := "/" + r.serviceName

	r.routes = []router.Route{
		router.NewRoute(http.MethodGet, routePrefix+"/{path:.*}", r.reverseProxyHandler()),
		router.NewRoute(http.MethodPost, routePrefix+"/{path:.*}", r.reverseProxyHandler()),
		router.NewRoute(http.MethodPut, routePrefix+"/{path:.*}", r.reverseProxyHandler()),
		router.NewRoute(http.MethodDelete, routePrefix+"/{path:.*}", r.reverseProxyHandler()),
		router.NewRoute(http.MethodPatch, routePrefix+"/{path:.*}", r.reverseProxyHandler()),
		router.NewRoute(http.MethodHead, routePrefix+"/{path:.*}", r.reverseProxyHandler()),
		router.NewRoute(http.MethodOptions, routePrefix+"/{path:.*}", r.reverseProxyHandler()),
	}
}

func (r *extensionRouter) Routes() []router.Route {
	return r.routes
}

// reverseProxyHandler returns an HTTP handler that uses httputil.ReverseProxy
// to forward requests to the extension's Unix socket
func (r *extensionRouter) reverseProxyHandler() httputils.APIFunc {
	// Create a reverse proxy that connects via Unix socket
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			// Set the scheme and host for the proxied request
			req.URL.Scheme = "http"
			req.URL.Host = "unix"

			// The path should already be set correctly by the router
			// Just ensure we're using the path after the service name prefix
		},
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", r.socketPath)
			},
		},
	}

	return func(ctx context.Context, w http.ResponseWriter, req *http.Request, vars map[string]string) error {
		// Check if this is an upgrade request (e.g., WebSocket, gRPC...)
		if req.Header.Get("Upgrade") != "" {
			return r.handleUpgrade(ctx, w, req, vars)
		}

		// Rewrite the URL path to strip the service name prefix
		targetPath := "/" + vars["path"]
		if req.URL.RawQuery != "" {
			targetPath += "?" + req.URL.RawQuery
		}

		// Update the request URL
		req.URL, _ = url.Parse("http://unix" + targetPath)

		// Use the reverse proxy
		proxy.ServeHTTP(w, req)
		return nil
	}
}

// handleUpgrade handles connection upgrade requests (e.g., WebSocket)
func (r *extensionRouter) handleUpgrade(ctx context.Context, w http.ResponseWriter, req *http.Request, vars map[string]string) error {
	// Hijack the client connection
	inStream, outStream, err := httputils.HijackConnection(w)
	if err != nil {
		http.Error(w, "Failed to hijack connection: "+err.Error(), http.StatusInternalServerError)
		return nil
	}
	defer httputils.CloseStreams(inStream, outStream)

	// Connect to the extension Unix socket
	extensionConn, err := net.Dial("unix", r.socketPath)
	if err != nil {
		return err
	}
	defer extensionConn.Close()

	// Build the target URL path (router already stripped the serviceName prefix)
	targetPath := "/" + vars["path"]
	if req.URL.RawQuery != "" {
		targetPath += "?" + req.URL.RawQuery
	}

	// Write the HTTP upgrade request to the backend
	req.URL.Path = targetPath
	if err := req.Write(extensionConn); err != nil {
		return err
	}

	eg := errgroup.Group{}
	// Backend -> Client
	eg.Go(func() error {
		_, err := io.Copy(outStream, extensionConn)
		return err
	})
	// Client -> Backend (including any buffered data)
	eg.Go(func() error {
		_, err := io.Copy(extensionConn, inStream)
		return err
	})
	// Wait for one direction to complete
	return eg.Wait()
}
