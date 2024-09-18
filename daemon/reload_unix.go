//go:build linux || freebsd

package daemon // import "github.com/docker/docker/daemon"

import (
	"bytes"
	"strconv"

	"github.com/docker/docker/daemon/config"
)

// reloadPlatform updates configuration with platform specific options
// and updates the passed attributes
func (daemon *Daemon) reloadPlatform(txn *reloadTxn, newCfg *configStore, conf *config.Config, attributes map[string]string) error {
	if conf.DefaultRuntime != "" {
		newCfg.DefaultRuntime = conf.DefaultRuntime
	}
	if conf.IsValueSet("runtimes") {
		newCfg.Config.Runtimes = conf.Runtimes
	}
	var err error
	newCfg.Runtimes, err = setupRuntimes(&newCfg.Config)
	if err != nil {
		return err
	}

	if conf.IsValueSet("default-shm-size") {
		newCfg.ShmSize = conf.ShmSize
	}

	if conf.CgroupNamespaceMode != "" {
		newCfg.CgroupNamespaceMode = conf.CgroupNamespaceMode
	}

	if conf.IpcMode != "" {
		newCfg.IpcMode = conf.IpcMode
	}

	// Update attributes
	var runtimeList bytes.Buffer
	for name, rt := range newCfg.Config.Runtimes {
		if runtimeList.Len() > 0 {
			runtimeList.WriteRune(' ')
		}
		runtimeList.WriteString(name + ":" + rt.Path)
	}

	attributes["runtimes"] = runtimeList.String()
	attributes["default-runtime"] = newCfg.DefaultRuntime
	attributes["default-shm-size"] = strconv.FormatInt(int64(newCfg.ShmSize), 10)
	attributes["default-ipc-mode"] = newCfg.IpcMode
	attributes["default-cgroupns-mode"] = newCfg.CgroupNamespaceMode
	return nil
}
