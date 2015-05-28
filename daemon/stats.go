package daemon

import (
	"encoding/json"
	"io"

	"github.com/docker/docker/daemon/execdriver"
)

func (daemon *Daemon) ContainerStats(name string, stream bool, out io.Writer) error {
	updates, err := daemon.SubscribeToContainerStats(name)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(out)
	for v := range updates {
		update := v.(*execdriver.ResourceStats)
		ss := convertToAPITypes(update.Stats)
		ss.MemoryStats.Limit = uint64(update.MemoryLimit)
		ss.Read = update.Read
		ss.CpuStats.SystemUsage = update.SystemUsage
		if err := enc.Encode(ss); err != nil {
			// TODO: handle the specific broken pipe
			daemon.UnsubscribeToContainerStats(name, updates)
			return err
		}
		if !stream {
			break
		}
	}
	return nil
}
