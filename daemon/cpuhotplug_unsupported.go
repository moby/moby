// +build !linux

package daemon

func (cpuhotplug *CpuHotPlug) Close() {
}

func (daemon *Daemon) updateRecCgroup(path, cpuset string) error {
	return nil
}

func (daemon *Daemon) updateParentCgroups(parent string) error {
	return nil
}

func (daemon *Daemon) updateCpusetContainers() {

}
