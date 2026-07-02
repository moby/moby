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

package introspection

import (
	context "context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/google/uuid"
	"google.golang.org/genproto/googleapis/rpc/code"
	rpc "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/status"

	api "github.com/containerd/containerd/api/services/introspection/v1"
	"github.com/containerd/containerd/api/types"
	"github.com/containerd/errdefs"
	"github.com/containerd/errdefs/pkg/errgrpc"
	"github.com/containerd/plugin"
	"github.com/containerd/plugin/registry"
	"github.com/containerd/typeurl/v2"

	"github.com/containerd/containerd/v2/core/introspection"
	"github.com/containerd/containerd/v2/pkg/filters"
	"github.com/containerd/containerd/v2/pkg/protobuf"
	ptypes "github.com/containerd/containerd/v2/pkg/protobuf/types"
	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/containerd/v2/plugins/services"
	"github.com/containerd/containerd/v2/plugins/services/warning"
)

func init() {
	registry.Register(&plugin.Registration{
		Type:     plugins.ServicePlugin,
		ID:       services.IntrospectionService,
		Requires: []plugin.Type{plugins.WarningPlugin},
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			i, err := ic.GetByID(plugins.WarningPlugin, plugins.DeprecationsPlugin)
			if err != nil {
				return nil, err
			}

			warningClient, ok := i.(warning.Service)
			if !ok {
				return nil, errors.New("could not create a local client for warning service")
			}

			// this service fetches all plugins through the plugin set of the plugin context
			return &Local{
				plugins:       ic.Plugins(),
				root:          ic.Properties[plugins.PropertyRootDir],
				warningClient: warningClient,
			}, nil
		},
	})
}

// Local is a local implementation of the introspection service
type Local struct {
	mu            sync.Mutex
	root          string
	plugins       *plugin.Set
	pluginCache   []*api.Plugin
	warningClient warning.Service
}

var _ = (introspection.Service)(&Local{})

// UpdateLocal updates the local introspection service
func (l *Local) UpdateLocal(root string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.root = root
}

// Plugins returns the locally defined plugins
func (l *Local) Plugins(ctx context.Context, fs ...string) (*api.PluginsResponse, error) {
	filter, err := filters.ParseAll(fs...)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errdefs.ErrInvalidArgument, err)
	}

	var plugins []*api.Plugin
	allPlugins := l.getPlugins()
	for _, p := range allPlugins {
		if filter.Match(adaptPlugin(p)) {
			plugins = append(plugins, p)
		}
	}

	return &api.PluginsResponse{
		Plugins: plugins,
	}, nil
}

func (l *Local) getPlugins() []*api.Plugin {
	l.mu.Lock()
	defer l.mu.Unlock()
	plugins := l.plugins.GetAll()
	if l.pluginCache == nil || len(plugins) != len(l.pluginCache) {
		l.pluginCache = pluginsToPB(plugins)
	}
	return l.pluginCache
}

// Server returns the local server information
func (l *Local) Server(ctx context.Context) (*api.ServerResponse, error) {
	u, err := l.getUUID()
	if err != nil {
		return nil, err
	}
	pid := os.Getpid()
	var pidns uint64
	if runtime.GOOS == "linux" {
		pidns, err = statPIDNS(pid)
		if err != nil {
			return nil, err
		}
	}
	return &api.ServerResponse{
		UUID:         u,
		Pid:          uint64(pid),
		Pidns:        pidns,
		Deprecations: l.getWarnings(ctx),
	}, nil
}

func (l *Local) getUUID() (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := os.ReadFile(l.uuidPath())
	if err != nil {
		if os.IsNotExist(err) {
			return l.generateUUID()
		}
		return "", err
	}
	if len(data) == 0 {
		return l.generateUUID()
	}
	u := string(data)
	if _, err := uuid.Parse(u); err != nil {
		return "", err
	}
	return u, nil
}

func (l *Local) generateUUID() (string, error) {
	u, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	path := l.uuidPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	uu := u.String()
	if err := os.WriteFile(path, []byte(uu), 0666); err != nil {
		return "", err
	}
	return uu, nil
}

func (l *Local) uuidPath() string {
	return filepath.Join(l.root, "uuid")
}

func (l *Local) getWarnings(ctx context.Context) []*api.DeprecationWarning {
	return warningsPB(ctx, l.warningClient.Warnings())
}

func adaptPlugin(o interface{}) filters.Adaptor {
	obj := o.(*api.Plugin)
	return filters.AdapterFunc(func(fieldpath []string) (string, bool) {
		if len(fieldpath) == 0 {
			return "", false
		}

		switch fieldpath[0] {
		case "type":
			return obj.Type, len(obj.Type) > 0
		case "id":
			return obj.ID, len(obj.ID) > 0
		case "platforms":
			// TODO(stevvooe): Another case here where have multiple values.
			// May need to refactor the filter system to allow filtering by
			// platform, if this is required.
		case "capabilities":
			// TODO(stevvooe): Need a better way to match against
			// collections. We can only return "the value" but really it
			// would be best if we could return a set of values for the
			// path, any of which could match.
		}

		return "", false
	})
}

func pluginToPB(p *plugin.Plugin) *api.Plugin {
	var requires []string
	for _, r := range p.Registration.Requires {
		requires = append(requires, r.String())
	}

	var initErr *rpc.Status
	if err := p.Err(); err != nil {
		st, ok := status.FromError(errgrpc.ToGRPC(err))
		if ok {
			var details []*ptypes.Any
			for _, d := range st.Proto().Details {
				details = append(details, &ptypes.Any{
					TypeUrl: d.TypeUrl,
					Value:   d.Value,
				})
			}
			initErr = &rpc.Status{
				Code:    int32(st.Code()),
				Message: st.Message(),
				Details: details,
			}
		} else {
			initErr = &rpc.Status{
				Code:    int32(code.Code_UNKNOWN),
				Message: err.Error(),
			}
		}
	}

	return &api.Plugin{
		Type:         p.Registration.Type.String(),
		ID:           p.Registration.ID,
		Requires:     requires,
		Platforms:    types.OCIPlatformToProto(p.Meta.Platforms),
		Capabilities: p.Meta.Capabilities,
		Exports:      p.Meta.Exports,
		InitErr:      initErr,
	}
}

func pluginsToPB(plugins []*plugin.Plugin) []*api.Plugin {
	pluginsPB := make([]*api.Plugin, 0, len(plugins))
	for _, p := range plugins {
		pluginsPB = append(pluginsPB, pluginToPB(p))
	}

	return pluginsPB
}

func warningsPB(ctx context.Context, warnings []warning.Warning) []*api.DeprecationWarning {
	var pb []*api.DeprecationWarning

	for _, w := range warnings {
		pb = append(pb, &api.DeprecationWarning{
			ID:             string(w.ID),
			Message:        w.Message,
			LastOccurrence: protobuf.ToTimestamp(w.LastOccurrence),
		})
	}
	return pb
}

type pluginInfoProvider interface {
	PluginInfo(context.Context, interface{}) (interface{}, error)
}

func (l *Local) PluginInfo(ctx context.Context, pluginType, id string, options any) (*api.PluginInfoResponse, error) {
	p := l.plugins.Get(plugin.Type(pluginType), id)
	if p == nil {
		return nil, fmt.Errorf("plugin %s.%s not found: %w", pluginType, id, errdefs.ErrNotFound)
	}

	resp := &api.PluginInfoResponse{
		Plugin: pluginToPB(p),
	}

	// Request additional info from plugin instance
	if options != nil {
		if p.Err() != nil {
			return resp, fmt.Errorf("cannot get extra info, plugin not successfully loaded: %w", errdefs.ErrFailedPrecondition)
		}
		inst, err := p.Instance()
		if err != nil {
			return resp, fmt.Errorf("failed to get plugin instance: %w", errdefs.ErrFailedPrecondition)
		}
		pi, ok := inst.(pluginInfoProvider)
		if !ok {
			return resp, fmt.Errorf("plugin does not provided extra information: %w", errdefs.ErrNotImplemented)
		}

		info, err := pi.PluginInfo(ctx, options)
		if err != nil {
			return resp, errgrpc.ToGRPC(err)
		}
		ai, err := typeurl.MarshalAny(info)
		if err != nil {
			return resp, fmt.Errorf("failed to marshal plugin info: %w", err)
		}
		resp.Extra = &ptypes.Any{
			TypeUrl: ai.GetTypeUrl(),
			Value:   ai.GetValue(),
		}
	}
	return resp, nil
}
