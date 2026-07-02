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

package server

import (
	"context"
	"os"

	"github.com/containerd/cgroups/v3"
	cgroup1 "github.com/containerd/cgroups/v3/cgroup1"
	cgroupsv2 "github.com/containerd/cgroups/v3/cgroup2"
	srvconfig "github.com/containerd/containerd/v2/cmd/containerd/server/config"
	"github.com/containerd/containerd/v2/internal/wintls"
	"github.com/containerd/containerd/v2/pkg/sys"
	"github.com/containerd/log"
	"github.com/containerd/otelttrpc"
	"github.com/containerd/ttrpc"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// apply sets config settings on the server process
func apply(ctx context.Context, config *srvconfig.Config) error {
	if config.OOMScore != 0 {
		log.G(ctx).Debugf("changing OOM score to %d", config.OOMScore)
		if err := sys.SetOOMScore(os.Getpid(), config.OOMScore); err != nil {
			log.G(ctx).WithError(err).Errorf("failed to change OOM score to %d", config.OOMScore)
		}
	}
	if config.Cgroup.Path != "" {
		if cgroups.Mode() == cgroups.Unified {
			cg, err := cgroupsv2.Load(config.Cgroup.Path)
			if err != nil {
				return err
			}
			if err := cg.AddProc(uint64(os.Getpid())); err != nil {
				return err
			}
		} else {
			cg, err := cgroup1.Load(cgroup1.StaticPath(config.Cgroup.Path))
			if err != nil {
				if err != cgroup1.ErrCgroupDeleted {
					return err
				}
				if cg, err = cgroup1.New(cgroup1.StaticPath(config.Cgroup.Path), &specs.LinuxResources{}); err != nil {
					return err
				}
			}
			if err := cg.AddProc(uint64(os.Getpid())); err != nil {
				return err
			}
		}
	}
	return nil
}

func newTTRPCServer() (*ttrpc.Server, error) {
	return ttrpc.NewServer(
		ttrpc.WithServerHandshaker(ttrpc.UnixSocketRequireSameUser()),
		ttrpc.WithUnaryServerInterceptor(otelttrpc.UnaryServerInterceptor()),
	)
}

// TLS resource helpers are no-ops on Linux.
func setTLSResource(r wintls.CertResource) {}
func cleanupTLSResources()                 {}
