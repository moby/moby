package dockerfile

import (
	"io"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/image"
	"golang.org/x/net/context"
)

// MockBackend implements the builder.Backend interface for unit testing
type MockBackend struct {
	getImageOnBuildFunc func(string) (builder.Image, error)
}

func (m *MockBackend) GetImageOnBuild(name string) (builder.Image, error) {
	if m.getImageOnBuildFunc != nil {
		return m.getImageOnBuildFunc(name)
	}
	return &mockImage{id: "theid"}, nil
}

func (m *MockBackend) TagImageWithReference(image.ID, reference.Named) error {
	return nil
}

func (m *MockBackend) PullOnBuild(ctx context.Context, name string, authConfigs map[string]types.AuthConfig, output io.Writer) (builder.Image, error) {
	return nil, nil
}

func (m *MockBackend) ContainerAttachRaw(cID string, stdin io.ReadCloser, stdout, stderr io.Writer, stream bool) error {
	return nil
}

func (m *MockBackend) ContainerCreate(config types.ContainerCreateConfig) (container.ContainerCreateCreatedBody, error) {
	return container.ContainerCreateCreatedBody{}, nil
}

func (m *MockBackend) ContainerRm(name string, config *types.ContainerRmConfig) error {
	return nil
}

func (m *MockBackend) Commit(string, *backend.ContainerCommitConfig) (string, error) {
	return "", nil
}

func (m *MockBackend) ContainerKill(containerID string, sig uint64) error {
	return nil
}

func (m *MockBackend) ContainerStart(containerID string, hostConfig *container.HostConfig, checkpoint string, checkpointDir string) error {
	return nil
}

func (m *MockBackend) ContainerWait(containerID string, timeout time.Duration) (int, error) {
	return 0, nil
}

func (m *MockBackend) ContainerUpdateCmdOnBuild(containerID string, cmd []string) error {
	return nil
}

func (m *MockBackend) ContainerCreateWorkdir(containerID string) error {
	return nil
}

func (m *MockBackend) CopyOnBuild(containerID string, destPath string, src builder.FileInfo, decompress bool) error {
	return nil
}

func (m *MockBackend) HasExperimental() bool {
	return false
}

func (m *MockBackend) SquashImage(from string, to string) (string, error) {
	return "", nil
}

func (m *MockBackend) MountImage(name string) (string, func() error, error) {
	return "", func() error { return nil }, nil
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
