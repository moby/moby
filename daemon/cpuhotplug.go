// +build linux

package daemon

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/daemon/cpuhotplug"
	"github.com/sirupsen/logrus"
)

const cpusetDir = "/sys/fs/cgroup/cpuset"

type containerUpdate struct {
	cparent string     // Cgroup parent to update
	done    chan error // Channel for synch
}

type CpuHotPlug struct {
	UdevChan chan struct{}        // Channel for  cpu events
	ContChan chan containerUpdate // Channel for started containers
}

func (cpuhotplug *CpuHotPlug) Close() {
	close(cpuhotplug.ContChan)
}

func (daemon *Daemon) updateRecCgroup(path, cpuset string) error {

	file := filepath.Join(path, "cpuset.cpus")

	if _, err := os.Stat(file); os.IsNotExist(err) {
		//If the parent does not exist nothing to update
		return nil
	}

	if err := ioutil.WriteFile(file, []byte(cpuset), 0777); err != nil {
		logrus.Warnf("Error update cgroup in %s", file)
		return err
	}
	entries, err := ioutil.ReadDir(path)

	if err != nil {
		logrus.Warnf("Error update recursively cgroup parent %s", err)
	}

	if entries == nil {
		return nil
	}

	// We update recursively until we find that the directory belongs
	// to a container or the cgroup parent branch is finshed
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		// The directory belongs to a container.
		// We don't update at this point in time the container.
		if daemon.Exists(e.Name()) {
			continue
		}

		if err := daemon.updateRecCgroup(filepath.Join(path, e.Name()), cpuset); err != nil {
			return err
		}
	}
	return nil
}

// updateParentCgroups updates the cgroup parent
func (daemon *Daemon) updateParentCgroups(parent string) error {
	cpuset, err := cpuhotplug.ReadCurrentCpuset()
	if err != nil {
		return err
	}
	//Default parent cgroup name is docker
	if parent == "" {
		parent = "docker"
	}
	logrus.Debugf("cpuhotplug: updated parent cgroup %s", parent)

	return daemon.updateRecCgroup(filepath.Join(cpusetDir, parent), cpuset)
}

// updateContainerCpuset updates the cpuset of a container
func (daemon *Daemon) updateContainerCpuset(id, cpuset string) error {
	resources := container.Resources{
		CpusetCpus: cpuset,
	}
	return daemon.containerd.UpdateResources(id, toContainerdResources(resources))
}

// syncUpdateParentCgroup synchronizes the update for the cgroup parent when a container
// is started. When a container is started its cgroup parent needs to be update as well.
// If the cgroup parent was already created but no containers were presented, it may not be
// correctly updated.
func (daemon *Daemon) syncUpdateParentCgroup(cparent string) error {
	done := make(chan error, 1)
	c := strings.Split(cparent, "/")[0]
	daemon.cpuHotplug.ContChan <- containerUpdate{
		cparent: c,
		done:    done,
	}
	err := <-done
	if err != nil {
		logrus.Errorf("Cgroup parent error %s", err)
	}

	return err

}

// performCpuHotplug listens to the cpu events and update the cpuset accordingly
func (daemon *Daemon) performCpuHotplug() {
	//Enable CPU event filter
	daemon.cpuHotplug.UdevChan = make(chan struct{})
	cpuhotplug.ListenToCpuEvent(daemon.cpuHotplug.UdevChan)
	logrus.Infof("Started cpuhotplug")

	//Initialize channel for incoming started container
	daemon.cpuHotplug.ContChan = make(chan containerUpdate)
	var oldCgroup = " "

	go func() {
		for {
			// Priority queue high to a started container over a cpu event
			// High priority queue: started container
			select {
			case w := <-daemon.cpuHotplug.ContChan:
				// if no cpu went online and we have already update in a
				// previous iteration the cgroup parent tree
				// we just skip it
				if oldCgroup != w.cparent {
					err := daemon.updateParentCgroups(w.cparent)
					oldCgroup = w.cparent
					w.done <- err
				}
				w.done <- nil
			default:
			}

			// Check if a cpu went online or a container is started
			select {
			// Cgroup parent update for started container
			case w := <-daemon.cpuHotplug.ContChan:
				if oldCgroup != w.cparent {
					err := daemon.updateParentCgroups(w.cparent)
					oldCgroup = w.cparent
					w.done <- err
				}
				w.done <- nil
			// CPU events
			case <-daemon.cpuHotplug.UdevChan:
				daemon.updateCpusetContainers()
				oldCgroup = " "
			}
		}
	}()

}

// updateRestrictedCpuset updates the cpuset of a restricted container.
func (daemon *Daemon) updateRestrictedCpuset(id, originalCpuset string) error {
	//Read which cpus are online
	currentCpusSet, err := cpuhotplug.ReadCurrentCpuset()
	if err != nil {
		return err
	}

	return daemon.updateContainerCpuset(id, cpuhotplug.NewCpusetRestrictedCont(currentCpusSet, originalCpuset))
}

// updateCpusetContainers updates all Docker containers and their cgroup
// parent.
func (daemon *Daemon) updateCpusetContainers() {

	// List of cgroup parent to update
	var listParentCgroup []string
	for _, c := range daemon.containers.List() {

		if !c.IsRunning() {
			continue
		}

		containerJson, err := daemon.ContainerInspectCurrent(c.Name, false)
		if err != nil {
			continue
		}
		cgroupParent := containerJson.ContainerJSONBase.HostConfig.CgroupParent
		// Find all the Parent cgroup of the container
		// Avoid duplicate
		// We just need the first directory then the cgroup parent tree will be
		// recursively updated
		s := strings.Split(cgroupParent, "/")[0]

		found := false
		// Check if the cgroup parent needs to be inserted
		for _, e := range listParentCgroup {
			if e == s {
				found = true
				break
			}
		}

		if !found {
			listParentCgroup = append(listParentCgroup, s)
		}

	}

	// Update cgroup parent
	for _, e := range listParentCgroup {
		if err := daemon.updateParentCgroups(e); err != nil {
			logrus.Errorf("Error %s updating cgroup parent %s", err, e)
		}
	}

	// Update the running containers
	for _, c := range daemon.containers.List() {

		if !c.IsRunning() {
			continue
		}
		//Get original cpuset
		containerJson, err := daemon.ContainerInspectCurrent(c.Name, false)
		if err != nil {
			logrus.Warnf("A problem has occured with container %s: %s\n", err, c.ID)
			continue
		}

		cpuset := containerJson.ContainerJSONBase.HostConfig.Resources.CpusetCpus

		//Unrestricted container
		if cpuset == "" {
			updatedCpuset, err := cpuhotplug.ReadCurrentCpuset()
			if err != nil {
				logrus.Warnf("Container %s  err: %s", c.ID, err)
			}
			if err := daemon.updateContainerCpuset(c.ID, updatedCpuset); err == nil {
				logrus.Debugf("Container %s updated succesfully", c.ID)
			} else {
				logrus.Warnf("Container %s  err: %s", c.ID, err)
			}
			continue
		}

		//Restricted container
		if err := daemon.updateRestrictedCpuset(c.ID, cpuset); err == nil {
			logrus.Debugf("Container %s updated succesfully", c.ID)
		} else {
			logrus.Warnf("Restricted container %s  err: %s", c.ID, err)
		}
	}
}
