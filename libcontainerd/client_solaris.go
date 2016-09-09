package libcontainerd

import "golang.org/x/net/context"

type client struct {
	clientCommon

	// Platform specific properties below here.
}

func (clnt *client) AddProcess(ctx context.Context, containerID, processFriendlyName string, specp Process) error {
	return nil
}

func (clnt *client) Create(containerID string, checkpoint string, checkpointDir string, spec Spec, options ...CreateOption) (err error) {
	return nil
}

func (clnt *client) Signal(containerID string, sig int) error {
	return nil
}

func (clnt *client) Resize(containerID, processFriendlyName string, width, height int) error {
	return nil
}

func (clnt *client) Pause(containerID string) error {
	return nil
}

func (clnt *client) Resume(containerID string) error {
	return nil
}

func (clnt *client) Stats(containerID string) (*Stats, error) {
	return nil, nil
}

// Restore is the handler for restoring a container
func (clnt *client) Restore(containerID string, unusedOnWindows ...CreateOption) error {
	return nil
}

func (clnt *client) GetPidsForContainer(containerID string) ([]int, error) {
	return nil, nil
}

// Summary returns a summary of the processes running in a container.
func (clnt *client) Summary(containerID string) ([]Summary, error) {
	return nil, nil
}

// UpdateResources updates resources for a running container.
func (clnt *client) UpdateResources(containerID string, resources Resources) error {
	// Updating resource isn't supported on Solaris
	// but we should return nil for enabling updating container
	return nil
}
