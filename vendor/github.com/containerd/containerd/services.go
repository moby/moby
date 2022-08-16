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
