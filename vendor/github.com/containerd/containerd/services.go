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
	"github.com/containerd/containerd/api/services/leases/v1"
	namespacesapi "github.com/containerd/containerd/api/services/namespaces/v1"
	"github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/snapshots"
)

type services struct {
	contentStore   content.Store
	imageStore     images.Store
	containerStore containers.Store
	namespaceStore namespaces.Store
	snapshotters   map[string]snapshots.Snapshotter
	taskService    tasks.TasksClient
	diffService    DiffService
	eventService   EventService
	leasesService  leases.LeasesClient
}

// ServicesOpt allows callers to set options on the services
type ServicesOpt func(c *services)

// WithContentStore sets the content store.
func WithContentStore(contentStore content.Store) ServicesOpt {
	return func(s *services) {
		s.contentStore = contentStore
	}
}

// WithImageService sets the image service.
func WithImageService(imageService imagesapi.ImagesClient) ServicesOpt {
	return func(s *services) {
		s.imageStore = NewImageStoreFromClient(imageService)
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

// WithContainerService sets the container service.
func WithContainerService(containerService containersapi.ContainersClient) ServicesOpt {
	return func(s *services) {
		s.containerStore = NewRemoteContainerStore(containerService)
	}
}

// WithTaskService sets the task service.
func WithTaskService(taskService tasks.TasksClient) ServicesOpt {
	return func(s *services) {
		s.taskService = taskService
	}
}

// WithDiffService sets the diff service.
func WithDiffService(diffService diff.DiffClient) ServicesOpt {
	return func(s *services) {
		s.diffService = NewDiffServiceFromClient(diffService)
	}
}

// WithEventService sets the event service.
func WithEventService(eventService EventService) ServicesOpt {
	return func(s *services) {
		s.eventService = eventService
	}
}

// WithNamespaceService sets the namespace service.
func WithNamespaceService(namespaceService namespacesapi.NamespacesClient) ServicesOpt {
	return func(s *services) {
		s.namespaceStore = NewNamespaceStoreFromClient(namespaceService)
	}
}

// WithLeasesService sets the lease service.
func WithLeasesService(leasesService leases.LeasesClient) ServicesOpt {
	return func(s *services) {
		s.leasesService = leasesService
	}
}
