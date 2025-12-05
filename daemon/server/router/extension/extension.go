package extension

import (
	"context"
	"io"
	"net"
	"net/http"

	"github.com/moby/moby/v2/daemon/server/httputils"
	"github.com/moby/moby/v2/daemon/server/router"
	"golang.org/x/sync/errgroup"
)

// NewRouter creates a new extension router that proxies requests to a Unix domain socket
func NewRouter(extensionName, socketPath string) router.Router {
	r := &extensionRouter{
		extensionName: extensionName,
		socketPath:    socketPath,
	}
	r.initRoutes()
	return r
}

type extensionRouter struct {
	extensionName string
	socketPath    string
	routes        []router.Route
}

func (r *extensionRouter) initRoutes() {
	// Match all methods and paths under /{extensionName}
	r.routes = []router.Route{
		router.NewRoute(http.MethodGet, "/"+r.extensionName+"/{path:.*}", r.proxyHandler()),
		router.NewRoute(http.MethodPost, "/"+r.extensionName+"/{path:.*}", r.proxyHandler()),
		router.NewRoute(http.MethodPut, "/"+r.extensionName+"/{path:.*}", r.proxyHandler()),
		router.NewRoute(http.MethodDelete, "/"+r.extensionName+"/{path:.*}", r.proxyHandler()),
		router.NewRoute(http.MethodPatch, "/"+r.extensionName+"/{path:.*}", r.proxyHandler()),
		router.NewRoute(http.MethodHead, "/"+r.extensionName+"/{path:.*}", r.proxyHandler()),
		router.NewRoute(http.MethodOptions, "/"+r.extensionName+"/{path:.*}", r.proxyHandler()),
	}
}

func (r *extensionRouter) Routes() []router.Route {
	return r.routes
}

func (r *extensionRouter) proxyHandler() httputils.APIFunc {
	return func(ctx context.Context, w http.ResponseWriter, req *http.Request, vars map[string]string) error {
		// Check if this is an upgrade request (e.g., WebSocket, gRPC...)
		if req.Header.Get("Upgrade") != "" {
			err := r.handleUpgrade(ctx, w, req, vars)
			if err != nil {
				http.Error(w, "Failed to Upgrade connexion: "+err.Error(), http.StatusBadGateway)
				return nil
			}
		}

		// Create a Unix domain socket connection
		conn, err := net.Dial("unix", r.socketPath)
		if err != nil {
			http.Error(w, "Failed to connect to extension: "+err.Error(), http.StatusBadGateway)
			return nil
		}
		defer conn.Close()

		// Create a new HTTP client that uses the Unix socket
		client := &http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", r.socketPath)
				},
			},
		}

		// Build the target URL path (router already stripped the extensionName prefix)
		targetPath := "/" + vars["path"]
		if req.URL.RawQuery != "" {
			targetPath += "?" + req.URL.RawQuery
		}

		// Create a new request with the target path
		proxyReq, err := http.NewRequestWithContext(ctx, req.Method, "http://unix"+targetPath, req.Body)
		if err != nil {
			http.Error(w, "Failed to create proxy request: "+err.Error(), http.StatusInternalServerError)
			return nil
		}

		// Copy headers
		for key, values := range req.Header {
			for _, value := range values {
				proxyReq.Header.Add(key, value)
			}
		}

		// Send the request
		resp, err := client.Do(proxyReq)
		if err != nil {
			http.Error(w, "Failed to proxy request: "+err.Error(), http.StatusBadGateway)
			return nil
		}
		defer resp.Body.Close()

		// Copy response headers
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}

		// Write status code
		w.WriteHeader(resp.StatusCode)

		// Copy response body
		_, err = io.Copy(w, resp.Body)
		return err
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

	// Build the target URL path (router already stripped the extensionName prefix)
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
