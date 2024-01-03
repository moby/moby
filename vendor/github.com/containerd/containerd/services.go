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

package containerd

import (
	"fmt"

	containersapi "github.com/containerd/containerd/api/services/containers/v1"
	"github.com/containerd/containerd/api/services/diff/v1"
	imagesapi "github.com/containerd/containerd/api/services/images/v1"
	introspectionapi "github.com/containerd/containerd/api/services/introspection/v1"
	namespacesapi "github.com/containerd/containerd/api/services/namespaces/v1"
	"github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/sandbox"
	srv "github.com/containerd/containerd/services"
	"github.com/containerd/containerd/services/introspection"
	"github.com/containerd/containerd/snapshots"
)

type services struct {
	contentStore         content.Store
	imageStore           images.Store
	containerStore       containers.Store
	namespaceStore       namespaces.Store
	snapshotters         map[string]snapshots.Snapshotter
	taskService          tasks.TasksClient
	diffService          DiffService
	eventService         EventService
	leasesService        leases.Manager
	introspectionService introspection.Service
	sandboxStore         sandbox.Store
	sandboxController    sandbox.Controller
}

// ServicesOpt allows callers to set options on the services
type ServicesOpt func(c *services)

// WithContentStore sets the content store.
func WithContentStore(contentStore content.Store) ServicesOpt {
	return func(s *services) {
		s.contentStore = contentStore
	}
}

// WithImageClient sets the image service to use using an images client.
func WithImageClient(imageService imagesapi.ImagesClient) ServicesOpt {
	return func(s *services) {
		s.imageStore = NewImageStoreFromClient(imageService)
	}
}

// WithImageStore sets the image store.
func WithImageStore(imageStore images.Store) ServicesOpt {
	return func(s *services) {
		s.imageStore = imageStore
	}
}

// WithSnapshotters sets the snapshotters.
func WithSnapshotters(snapshotters map[string]snapshots.Snapshotter) ServicesOpt {
	return func(s *services) {
		s.snapshotters = make(map[string]snapshots.Snapshotter)
		for n, sn := range snapshotters {
			s.snapshotters[n] = sn
		}
	}
}

// WithContainerClient sets the container service to use using a containers client.
func WithContainerClient(containerService containersapi.ContainersClient) ServicesOpt {
	return func(s *services) {
		s.containerStore = NewRemoteContainerStore(containerService)
	}
}

// WithContainerStore sets the container store.
func WithContainerStore(containerStore containers.Store) ServicesOpt {
	return func(s *services) {
		s.containerStore = containerStore
	}
}

// WithTaskClient sets the task service to use from a tasks client.
func WithTaskClient(taskService tasks.TasksClient) ServicesOpt {
	return func(s *services) {
		s.taskService = taskService
	}
}

// WithDiffClient sets the diff service to use from a diff client.
func WithDiffClient(diffService diff.DiffClient) ServicesOpt {
	return func(s *services) {
		s.diffService = NewDiffServiceFromClient(diffService)
	}
}

// WithDiffService sets the diff store.
func WithDiffService(diffService DiffService) ServicesOpt {
	return func(s *services) {
		s.diffService = diffService
	}
}

// WithEventService sets the event service.
func WithEventService(eventService EventService) ServicesOpt {
	return func(s *services) {
		s.eventService = eventService
	}
}

// WithNamespaceClient sets the namespace service using a namespaces client.
func WithNamespaceClient(namespaceService namespacesapi.NamespacesClient) ServicesOpt {
	return func(s *services) {
		s.namespaceStore = NewNamespaceStoreFromClient(namespaceService)
	}
}

// WithNamespaceService sets the namespace service.
func WithNamespaceService(namespaceService namespaces.Store) ServicesOpt {
	return func(s *services) {
		s.namespaceStore = namespaceService
	}
}

// WithLeasesService sets the lease service.
func WithLeasesService(leasesService leases.Manager) ServicesOpt {
	return func(s *services) {
		s.leasesService = leasesService
	}
}

// WithIntrospectionClient sets the introspection service using an introspection client.
func WithIntrospectionClient(in introspectionapi.IntrospectionClient) ServicesOpt {
	return func(s *services) {
		s.introspectionService = introspection.NewIntrospectionServiceFromClient(in)
	}
}

// WithIntrospectionService sets the introspection service.
func WithIntrospectionService(in introspection.Service) ServicesOpt {
	return func(s *services) {
		s.introspectionService = in
	}
}

// WithSandboxStore sets the sandbox store.
func WithSandboxStore(client sandbox.Store) ServicesOpt {
	return func(s *services) {
		s.sandboxStore = client
	}
}

// WithSandboxController sets the sandbox controller.
func WithSandboxController(client sandbox.Controller) ServicesOpt {
	return func(s *services) {
		s.sandboxController = client
	}
}

// WithInMemoryServices is suitable for cases when there is need to use containerd's client from
// another (in-memory) containerd plugin (such as CRI).
func WithInMemoryServices(ic *plugin.InitContext) ClientOpt {
	return func(c *clientOpts) error {
		var opts []ServicesOpt
		for t, fn := range map[plugin.Type]func(interface{}) ServicesOpt{
			plugin.EventPlugin: func(i interface{}) ServicesOpt {
				return WithEventService(i.(EventService))
			},
			plugin.LeasePlugin: func(i interface{}) ServicesOpt {
				return WithLeasesService(i.(leases.Manager))
			},
			plugin.SandboxStorePlugin: func(i interface{}) ServicesOpt {
				return WithSandboxStore(i.(sandbox.Store))
			},
			plugin.SandboxControllerPlugin: func(i interface{}) ServicesOpt {
				return WithSandboxController(i.(sandbox.Controller))
			},
		} {
			i, err := ic.Get(t)
			if err != nil {
				return fmt.Errorf("failed to get %q plugin: %w", t, err)
			}
			opts = append(opts, fn(i))
		}

		plugins, err := ic.GetByType(plugin.ServicePlugin)
		if err != nil {
			return fmt.Errorf("failed to get service plugin: %w", err)
		}
		for s, fn := range map[string]func(interface{}) ServicesOpt{
			srv.ContentService: func(s interface{}) ServicesOpt {
				return WithContentStore(s.(content.Store))
			},
			srv.ImagesService: func(s interface{}) ServicesOpt {
				return WithImageClient(s.(imagesapi.ImagesClient))
			},
			srv.SnapshotsService: func(s interface{}) ServicesOpt {
				return WithSnapshotters(s.(map[string]snapshots.Snapshotter))
			},
			srv.ContainersService: func(s interface{}) ServicesOpt {
				return WithContainerClient(s.(containersapi.ContainersClient))
			},
			srv.TasksService: func(s interface{}) ServicesOpt {
				return WithTaskClient(s.(tasks.TasksClient))
			},
			srv.DiffService: func(s interface{}) ServicesOpt {
				return WithDiffClient(s.(diff.DiffClient))
			},
			srv.NamespacesService: func(s interface{}) ServicesOpt {
				return WithNamespaceClient(s.(namespacesapi.NamespacesClient))
			},
			srv.IntrospectionService: func(s interface{}) ServicesOpt {
				return WithIntrospectionClient(s.(introspectionapi.IntrospectionClient))
			},
		} {
			p := plugins[s]
			if p == nil {
				return fmt.Errorf("service %q not found", s)
			}
			i, err := p.Instance()
			if err != nil {
				return fmt.Errorf("failed to get instance of service %q: %w", s, err)
			}
			if i == nil {
				return fmt.Errorf("instance of service %q not found", s)
			}
			opts = append(opts, fn(i))
		}

		c.services = &services{}
		for _, o := range opts {
			o(c.services)
		}
		return nil
	}
}
