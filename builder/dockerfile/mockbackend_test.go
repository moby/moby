package dockerfile

import (
	"io"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder"
	containerpkg "github.com/docker/docker/container"
	"github.com/docker/docker/image"
	"golang.org/x/net/context"
)

// MockBackend implements the builder.Backend interface for unit testing
type MockBackend struct {
	containerCreateFunc func(config types.ContainerCreateConfig) (container.ContainerCreateCreatedBody, error)
	commitFunc          func(string, *backend.ContainerCommitConfig) (string, error)
	getImageFunc        func(string) (builder.Image, builder.ReleaseableLayer, error)
}

func (m *MockBackend) TagImageWithReference(image.ID, reference.Named) error {
	return nil
}

func (m *MockBackend) ContainerAttachRaw(cID string, stdin io.ReadCloser, stdout, stderr io.Writer, stream bool, attached chan struct{}) error {
	return nil
}

func (m *MockBackend) ContainerCreate(config types.ContainerCreateConfig) (container.ContainerCreateCreatedBody, error) {
	if m.containerCreateFunc != nil {
		return m.containerCreateFunc(config)
	}
	return container.ContainerCreateCreatedBody{}, nil
}

func (m *MockBackend) ContainerRm(name string, config *types.ContainerRmConfig) error {
	return nil
}

func (m *MockBackend) Commit(cID string, cfg *backend.ContainerCommitConfig) (string, error) {
	if m.commitFunc != nil {
		return m.commitFunc(cID, cfg)
	}
	return "", nil
}

func (m *MockBackend) ContainerKill(containerID string, sig uint64) error {
	return nil
}

func (m *MockBackend) ContainerStart(containerID string, hostConfig *container.HostConfig, checkpoint string, checkpointDir string) error {
	return nil
}

func (m *MockBackend) ContainerWait(ctx context.Context, containerID string, condition containerpkg.WaitCondition) (<-chan containerpkg.StateStatus, error) {
	return nil, nil
}

func (m *MockBackend) ContainerCreateWorkdir(containerID string) error {
	return nil
}

func (m *MockBackend) CopyOnBuild(containerID string, destPath string, srcRoot string, srcPath string, decompress bool) error {
	return nil
}

func (m *MockBackend) GetImageAndReleasableLayer(ctx context.Context, refOrID string, opts backend.GetImageAndLayerOptions) (builder.Image, builder.ReleaseableLayer, error) {
	if m.getImageFunc != nil {
		return m.getImageFunc(refOrID)
	}

	return &mockImage{id: "theid"}, &mockLayer{}, nil
}

type mockImage struct {
	id     string
	config *container.Config
}

func (i *mockImage) ImageID() string {
	return i.id
}

func (i *mockImage) RunConfig() *container.Config {
	return i.config
}

type mockImageCache struct {
	getCacheFunc func(parentID string, cfg *container.Config) (string, error)
}

func (mic *mockImageCache) GetCache(parentID string, cfg *container.Config) (string, error) {
	if mic.getCacheFunc != nil {
		return mic.getCacheFunc(parentID, cfg)
	}
	return "", nil
}

type mockLayer struct{}

func (l *mockLayer) Release() error {
	return nil
}

func (l *mockLayer) Mount() (string, error) {
	return "mountPath", nil
}
