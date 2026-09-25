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

package v2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	bootapi "github.com/containerd/containerd/api/runtime/bootstrap/v1"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/protobuf/proto"
	"github.com/containerd/containerd/v2/pkg/protobuf/types"
	"github.com/containerd/containerd/v2/version"
	"github.com/containerd/log"
)

// Environment variables passed to the shim binary.
//
// TODO: Remove in a future release in favor of Bootstrap protocol.
const (
	ttrpcAddressEnv = "TTRPC_ADDRESS"
	grpcAddressEnv  = "GRPC_ADDRESS"
	namespaceEnv    = "NAMESPACE"
	maxVersionEnv   = "MAX_SHIM_VERSION"
)

type commandConfig struct {
	ID           string
	RuntimePath  string
	BundlePath   string
	GRPCAddress  string
	TTRPCAddress string
	WorkDir      string
	Args         []string
	Opts         *types.Any
	Env          []string
	LogLevel     log.Level
	Action       string // Either "start" or "delete"
	SocketDir    string
}

// command returns the shim command with the provided args and configuration.
//
// It is used to invoke the shim binary for "start" and "delete" actions during the
// shim lifecycle, and encodes launch internals: backwards compatibility with older
// shim models and the new Bootstrap protocol used by 2.3+ shims.
func command(ctx context.Context, config *commandConfig) (*exec.Cmd, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}

	// TODO: Remove in a future release in favor of Bootstrap protocol.
	args := []string{
		"-namespace", ns,
		"-address", config.GRPCAddress,
		"-publish-binary", self,
		"-id", config.ID,
	}
	if config.BundlePath != "" {
		args = append(args, "-bundle", config.BundlePath)
	}
	switch config.LogLevel {
	case log.DebugLevel, log.TraceLevel:
		args = append(args, "-debug")
	}
	if config.Action == "" {
		return nil, errors.New("action must be specified in commandConfig")
	}

	args = append(args, config.Action)

	if len(config.Args) > 0 {
		args = append(args, config.Args...)
	}

	cmd := exec.CommandContext(ctx, config.RuntimePath, args...)
	cmd.Dir = config.WorkDir
	cmd.Env = append(
		os.Environ(),
		"GOMAXPROCS=2",
		fmt.Sprintf("%s=2", maxVersionEnv),
		// TODO: Remove in a future release in favor of Bootstrap protocol.
		fmt.Sprintf("%s=%s", ttrpcAddressEnv, config.TTRPCAddress),
		fmt.Sprintf("%s=%s", grpcAddressEnv, config.GRPCAddress),
		fmt.Sprintf("%s=%s", namespaceEnv, ns),
	)
	if len(config.Env) > 0 {
		cmd.Env = append(cmd.Env, config.Env...)
	}
	cmd.SysProcAttr = getSysProcAttr()

	// Special path when upgrading from 1.7 shims to 2.x containerd.
	// v1 shims would fail if passed wrong stdin data.
	// TODO: Remove in a future release in favor of Bootstrap protocol.
	execName := filepath.Base(config.RuntimePath)
	if strings.Contains(execName, "shim-runc-v1") || strings.Contains(execName, "shim-runhcs-v1") {
		if config.Opts != nil {
			d, err := proto.Marshal(config.Opts)
			if err != nil {
				return nil, err
			}
			cmd.Stdin = bytes.NewReader(d)
		}
	} else if config.Action == "start" {
		// Use the new Bootstrap protocol for all newer shims.
		params := bootapi.BootstrapParams{
			InstanceID:             config.ID,
			Namespace:              ns,
			LogLevel:               bootapi.LogLevelFromString(config.LogLevel.String()),
			ContainerdVersion:      version.Version,
			ContainerdGrpcAddress:  config.GRPCAddress,
			ContainerdTtrpcAddress: config.TTRPCAddress,
			ContainerdBinary:       self,
		}
		if config.SocketDir != "" {
			params.SocketDir = &config.SocketDir
		}

		if config.Opts != nil {
			if err := params.AddExtension(config.Opts); err != nil {
				return nil, fmt.Errorf("unable to add runtime options extensions: %w", err)
			}
		}

		data, err := proto.Marshal(&params)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal bootstrap params: %w", err)
		}

		cmd.Stdin = bytes.NewReader(data)
	}

	return cmd, nil
}
