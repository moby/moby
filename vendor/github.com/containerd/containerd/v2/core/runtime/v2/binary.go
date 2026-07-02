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
	"fmt"
	"io"
	"os"
	"path/filepath"
	gruntime "runtime"

	"github.com/containerd/containerd/api/runtime/task/v2"
	"github.com/containerd/containerd/v2/core/runtime"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/protobuf"
	"github.com/containerd/containerd/v2/pkg/protobuf/proto"
	"github.com/containerd/containerd/v2/pkg/protobuf/types"
	client "github.com/containerd/containerd/v2/pkg/shim"
	"github.com/containerd/log"
)

type shimBinaryConfig struct {
	runtime      string
	address      string
	ttrpcAddress string
	env          []string
}

func shimBinary(bundle *Bundle, config shimBinaryConfig) *binary {
	return &binary{
		bundle:                 bundle,
		runtime:                config.runtime,
		containerdAddress:      config.address,
		containerdTTRPCAddress: config.ttrpcAddress,
		env:                    config.env,
	}
}

type binary struct {
	runtime                string
	containerdAddress      string
	containerdTTRPCAddress string
	bundle                 *Bundle
	env                    []string
}

func (b *binary) Start(ctx context.Context, opts *types.Any, onClose func()) (_ *shim, err error) {
	args := []string{"-id", b.bundle.ID}
	switch log.GetLevel() {
	case log.DebugLevel, log.TraceLevel:
		args = append(args, "-debug")
	}
	args = append(args, "start")

	cmd, err := client.Command(
		ctx,
		&client.CommandConfig{
			Runtime:      b.runtime,
			Address:      b.containerdAddress,
			TTRPCAddress: b.containerdTTRPCAddress,
			Path:         b.bundle.Path,
			Opts:         opts,
			Args:         args,
			Env:          b.env,
		})
	if err != nil {
		return nil, err
	}
	// Windows needs a namespace when openShimLog
	ns, _ := namespaces.Namespace(ctx)
	shimCtx, cancelShimLog := context.WithCancel(namespaces.WithNamespace(context.Background(), ns))
	defer func() {
		if err != nil {
			cancelShimLog()
		}
	}()
	f, err := openShimLog(shimCtx, b.bundle, client.AnonDialer)
	if err != nil {
		return nil, fmt.Errorf("open shim log pipe: %w", err)
	}
	defer func() {
		if err != nil {
			f.Close()
		}
	}()
	// open the log pipe and block until the writer is ready
	// this helps with synchronization of the shim
	// copy the shim's logs to containerd's output
	go func() {
		defer f.Close()
		_, err := io.Copy(os.Stderr, f)
		// To prevent flood of error messages, the expected error
		// should be reset, like os.ErrClosed or os.ErrNotExist, which
		// depends on platform.
		err = checkCopyShimLogError(ctx, err)
		if err != nil {
			log.G(ctx).WithError(err).Error("copy shim log")
		}
	}()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", out, err)
	}
	response := bytes.TrimSpace(out)

	onCloseWithShimLog := func() {
		onClose()
		cancelShimLog()
		f.Close()
	}
	// Save runtime binary path for restore.
	if err := os.WriteFile(filepath.Join(b.bundle.Path, "shim-binary-path"), []byte(b.runtime), 0600); err != nil {
		return nil, err
	}

	params, err := parseStartResponse(response)
	if err != nil {
		return nil, err
	}

	conn, err := makeConnection(ctx, b.bundle.ID, params, onCloseWithShimLog)
	if err != nil {
		return nil, err
	}

	// Save bootstrap configuration (so containerd can restore shims after restart).
	if err := writeBootstrapParams(filepath.Join(b.bundle.Path, "bootstrap.json"), params); err != nil {
		return nil, fmt.Errorf("failed to write bootstrap.json: %w", err)
	}
	// The address is in the form like ttrpc+unix://<uds-path> or grpc+vsock://<cid>:<port>
	address := fmt.Sprintf("%s+%s", params.Protocol, params.Address)
	return &shim{
		bundle:  b.bundle,
		client:  conn,
		address: address,
		version: params.Version,
	}, nil
}

func (b *binary) Delete(ctx context.Context) (*runtime.Exit, error) {
	log.G(ctx).WithField("id", b.bundle.ID).Info("cleaning up dead shim")

	// On Windows and FreeBSD, the current working directory of the shim should
	// not be the bundle path during the delete operation. Instead, we invoke
	// with the default work dir and forward the bundle path on the cmdline.
	// Windows cannot delete the current working directory while an executable
	// is in use with it. On FreeBSD, fork/exec can fail.
	var bundlePath string
	if gruntime.GOOS != "windows" && gruntime.GOOS != "freebsd" {
		bundlePath = b.bundle.Path
	}
	args := []string{
		"-id", b.bundle.ID,
		"-bundle", b.bundle.Path,
	}
	switch log.GetLevel() {
	case log.DebugLevel, log.TraceLevel:
		args = append(args, "-debug")
	}
	args = append(args, "delete")

	cmd, err := client.Command(ctx,
		&client.CommandConfig{
			Runtime:      b.runtime,
			Address:      b.containerdAddress,
			TTRPCAddress: b.containerdTTRPCAddress,
			Path:         bundlePath,
			Opts:         nil,
			Args:         args,
		})

	if err != nil {
		return nil, err
	}
	var (
		out  = bytes.NewBuffer(nil)
		errb = bytes.NewBuffer(nil)
	)
	cmd.Stdout = out
	cmd.Stderr = errb
	if err := cmd.Run(); err != nil {
		log.G(ctx).WithFields(log.Fields{
			"cmd":   cmd.String(),
			"error": err,
			"id":    b.bundle.ID,
		}).Error("failed to delete dead shim")
		return nil, fmt.Errorf("%s: %w", errb.String(), err)
	}
	if s := errb.String(); s != "" {
		log.G(ctx).WithFields(log.Fields{
			"id":       b.bundle.ID,
			"warnings": s,
		}).Warn("warnings while cleaning up dead shim")
	}
	var response task.DeleteResponse
	if err := proto.Unmarshal(out.Bytes(), &response); err != nil {
		return nil, err
	}
	if err := b.bundle.Delete(); err != nil {
		return nil, err
	}
	return &runtime.Exit{
		Status:    response.ExitStatus,
		Timestamp: protobuf.FromTimestamp(response.ExitedAt),
		Pid:       response.Pid,
	}, nil
}
