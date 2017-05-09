package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/containerd/containerd/specs"
	ocs "github.com/opencontainers/runtime-spec/specs-go"
)

func getRootIDs(s *specs.Spec) (int, int, error) {
	return 0, 0, nil
}

func (c *container) OOM() (OOM, error) {
	return nil, nil
}

func (c *container) Pids() ([]int, error) {
	var pids []int

	// TODO: This could be racy. Needs more investigation.
	//we get this information from runz state
	cmd := exec.Command(c.runtime, "state", c.id)
	outBuf, errBuf := new(bytes.Buffer), new(bytes.Buffer)
	cmd.Stdout, cmd.Stderr = outBuf, errBuf

	if err := cmd.Run(); err != nil {
		if strings.Contains(errBuf.String(), "Container not found") {
			return nil, errContainerNotFound
		}
		return nil, fmt.Errorf("Error is: %+v\n", err)
	}
	response := ocs.State{}
	decoder := json.NewDecoder(outBuf)
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("unable to decode json response: %+v", err)
	}
	pids = append(pids, response.Pid)
	return pids, nil
}

func (c *container) UpdateResources(r *Resource) error {
	return nil
}
