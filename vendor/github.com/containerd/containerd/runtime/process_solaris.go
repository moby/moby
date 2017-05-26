// +build solaris

package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"

	runtimespec "github.com/opencontainers/runtime-spec/specs-go"
)

// On Solaris we already have a state file maintained by the framework.
// This is read by runz state. We just call that instead of maintaining
// a separate file.
func (p *process) getPidFromFile() (int, error) {
	//we get this information from runz state
	cmd := exec.Command("runc", "state", p.container.ID())
	outBuf, errBuf := new(bytes.Buffer), new(bytes.Buffer)
	cmd.Stdout, cmd.Stderr = outBuf, errBuf

	if err := cmd.Run(); err != nil {
		// TODO: Improve logic
		return -1, errContainerNotFound
	}
	response := runtimespec.State{}
	decoder := json.NewDecoder(outBuf)
	if err := decoder.Decode(&response); err != nil {
		return -1, fmt.Errorf("unable to decode json response: %+v", err)
	}
	p.pid = response.Pid
	return p.pid, nil
}
