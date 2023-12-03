package process

import (
	"gotest.tools/v3/poll"

	procpkg "github.com/docker/docker/pkg/process"
)

// NotAlive verifies the process doesn't exist (finished or never started).
func NotAlive(pid int) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		if !procpkg.Alive(pid) {
			return poll.Success()
		}

		return poll.Continue("waiting for process to finish")
	}
}
