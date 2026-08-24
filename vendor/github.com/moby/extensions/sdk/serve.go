package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/containerd/log"
	sdkapipb "github.com/moby/extensions/sdk/sdkapi/protogen"
)

// Listen reads the startup config from stdin and serves the registered services.
func (s *Server) Listen(ctx context.Context) error {
	return s.ListenWithIO(ctx, os.Stdin, os.Stdout)
}

// ListenWithIO is Listen with explicit streams, for tests.
func (s *Server) ListenWithIO(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	var cfg StartupConfig
	if err := json.NewDecoder(stdin).Decode(&cfg); err != nil {
		return fmt.Errorf("read startup config: %w", err)
	}
	if cfg.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("unsupported extension protocol version %d", cfg.ProtocolVersion)
	}
	listener, err := net.Listen("unix", cfg.Endpoint)
	if err != nil {
		return fmt.Errorf("listen on extension socket: %w", err)
	}
	defer func() { _ = listener.Close() }()

	s.config = cfg.Config
	s.callbackEndpoint = cfg.CallbackEndpoint
	s.initCtx = ctx

	defer func() {
		if s.callbackConn != nil {
			_ = s.callbackConn.Close()
		}
		// Shutdown only what initialized, using a context detached from cancellation
		// because Serve usually returns after the daemon signals the process.
		if s.initialized && s.shutdown != nil {
			if err := s.shutdown(context.WithoutCancel(ctx)); err != nil {
				log.G(ctx).WithError(err).Warn("extension shutdown failed")
			}
		}
	}()

	sdkapipb.RegisterServer(s.grpc, runtimeServer{s: s})
	if _, err := io.WriteString(stdout, ReadinessAck); err != nil {
		return fmt.Errorf("write readiness ack: %w", err)
	}
	go func() {
		<-ctx.Done()
		s.grpc.GracefulStop()
	}()
	if err := s.grpc.Serve(listener); err != nil {
		log.G(ctx).WithError(err).Debug("extension gRPC server stopped")
	}
	return nil
}

// initialize runs Init with the handshake config and callback resolver.
func (s *Server) initialize() error {
	if s.init == nil {
		s.initialized = true
		return nil
	}
	resolver, err := s.resolver()
	if err != nil {
		return err
	}
	if err := s.init(s.initCtx, s.config, resolver); err != nil {
		return fmt.Errorf("initialize extension: %w", err)
	}
	s.initialized = true
	return nil
}
