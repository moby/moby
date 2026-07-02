/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package shim

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/containerd/ttrpc"
	"github.com/containerd/typeurl/v2"

	"github.com/containerd/containerd/v2/pkg/atomicfile"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/protobuf/proto"
	"github.com/containerd/containerd/v2/pkg/protobuf/types"
	"github.com/containerd/errdefs"
)

type CommandConfig struct {
	Runtime      string
	Address      string
	TTRPCAddress string
	Path         string
	Args         []string
	Opts         *types.Any
	Env          []string
}

// Command returns the shim command with the provided args and configuration
func Command(ctx context.Context, config *CommandConfig) (*exec.Cmd, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	args := []string{
		"-namespace", ns,
		"-address", config.Address,
		"-publish-binary", self,
	}
	args = append(args, config.Args...)
	cmd := exec.CommandContext(ctx, config.Runtime, args...)
	cmd.Dir = config.Path
	cmd.Env = append(
		os.Environ(),
		"GOMAXPROCS=2",
		fmt.Sprintf("%s=2", maxVersionEnv),
		fmt.Sprintf("%s=%s", ttrpcAddressEnv, config.TTRPCAddress),
		fmt.Sprintf("%s=%s", grpcAddressEnv, config.Address),
		fmt.Sprintf("%s=%s", namespaceEnv, ns),
	)
	if len(config.Env) > 0 {
		cmd.Env = append(cmd.Env, config.Env...)
	}
	cmd.SysProcAttr = getSysProcAttr()
	if config.Opts != nil {
		d, err := proto.Marshal(config.Opts)
		if err != nil {
			return nil, err
		}
		cmd.Stdin = bytes.NewReader(d)
	}
	return cmd, nil
}

// BinaryName returns the shim binary name from the runtime name,
// empty string returns means runtime name is invalid
func BinaryName(runtime string) string {
	// runtime name should format like $prefix.name.version
	parts := strings.Split(runtime, ".")
	if len(parts) < 2 || parts[0] == "" {
		return ""
	}

	return fmt.Sprintf(shimBinaryFormat, parts[len(parts)-2], parts[len(parts)-1])
}

// BinaryPath returns the full path for the shim binary from the runtime name,
// empty string returns means runtime name is invalid
func BinaryPath(runtime string) string {
	dir := filepath.Dir(runtime)
	binary := BinaryName(runtime)

	path, err := filepath.Abs(filepath.Join(dir, binary))
	if err != nil {
		return ""
	}

	return path
}

// Connect to the provided address
func Connect(address string, d func(string, time.Duration) (net.Conn, error)) (net.Conn, error) {
	return d(address, 100*time.Second)
}

// WritePidFile writes a pid file atomically
func WritePidFile(path string, pid int) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	f, err := atomicfile.New(path, 0o644)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "%d", pid)
	if err != nil {
		f.Cancel()
		return err
	}
	return f.Close()
}

// ErrNoAddress is returned when the address file has no content
var ErrNoAddress = errors.New("no shim address")

// ReadAddress returns the shim's socket address from the path
func ReadAddress(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", ErrNoAddress
	}
	return string(data), nil
}

// ReadRuntimeOptions reads config bytes from io.Reader and unmarshals it into the provided type.
// The type must be registered with typeurl.
//
// The function will return ErrNotFound, if the config is not provided.
// And ErrInvalidArgument, if unable to cast the config to the provided type T.
func ReadRuntimeOptions[T any](reader io.Reader) (T, error) {
	var config T

	data, err := io.ReadAll(reader)
	if err != nil {
		return config, fmt.Errorf("failed to read config bytes from stdin: %w", err)
	}

	if len(data) == 0 {
		return config, errdefs.ErrNotFound
	}

	var any types.Any
	if err := proto.Unmarshal(data, &any); err != nil {
		return config, err
	}

	v, err := typeurl.UnmarshalAny(&any)
	if err != nil {
		return config, err
	}

	config, ok := v.(T)
	if !ok {
		return config, fmt.Errorf("invalid type %T: %w", v, errdefs.ErrInvalidArgument)
	}

	return config, nil
}

// chainUnaryServerInterceptors creates a single ttrpc server interceptor from
// a chain of many interceptors executed from first to last.
func chainUnaryServerInterceptors(interceptors ...ttrpc.UnaryServerInterceptor) ttrpc.UnaryServerInterceptor {
	n := len(interceptors)

	// force to use default interceptor in ttrpc
	if n == 0 {
		return nil
	}

	return func(ctx context.Context, unmarshal ttrpc.Unmarshaler, info *ttrpc.UnaryServerInfo, method ttrpc.Method) (interface{}, error) {
		currentMethod := method

		for i := n - 1; i > 0; i-- {
			interceptor := interceptors[i]
			innerMethod := currentMethod

			currentMethod = func(currentCtx context.Context, currentUnmarshal func(interface{}) error) (interface{}, error) {
				return interceptor(currentCtx, currentUnmarshal, info, innerMethod)
			}
		}
		return interceptors[0](ctx, unmarshal, info, currentMethod)
	}
}
