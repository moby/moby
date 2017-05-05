package daemon

import (
	"github.com/docker/docker/api/types/runtime"
)

func (daemon *Daemon) ListRuntimes() (runtime.GetRuntimesResponse, error) {
	var resp runtime.GetRuntimesResponse

	runtimes := daemon.runtimeManager.GetAllRuntimes()

	for _, r := range runtimes {
		resp.Runtimes = append(resp.Runtimes, r.Info())
	}

	return resp, nil

}
func (daemon *Daemon) SetDefaultRuntime(runtime string) error {
	return daemon.runtimeManager.SetDefaultRuntime(runtime)
}
