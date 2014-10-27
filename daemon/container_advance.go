package daemon

import (
	"reflect"

	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/daemon/graphdriver/devmapper"
	"github.com/docker/docker/daemon/sweep"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/broadcastwriter"
	"github.com/docker/docker/pkg/log"
	"github.com/docker/docker/utils"
)

func (container *Container) Sweep(eng *engine.Engine) error {
	cmd := []string{"ip", "link", "delete", "eth0"}
	entrypoint, args := container.daemon.getEntrypointAndArgs(nil, cmd)

	processConfig := execdriver.ProcessConfig{
		Privileged: true,
		User:       "",
		Tty:        false,
		Entrypoint: entrypoint,
		Arguments:  args,
	}

	execConfig := &execConfig{
		ID:            utils.GenerateRandomID(),
		OpenStdin:     false,
		OpenStdout:    false,
		OpenStderr:    false,
		StreamConfig:  StreamConfig{},
		ProcessConfig: processConfig,
		Container:     container,
		Running:       false,
	}

	execConfig.StreamConfig.stderr = broadcastwriter.New()
	execConfig.StreamConfig.stdout = broadcastwriter.New()

	pipes := execdriver.NewPipes(execConfig.StreamConfig.stdin, execConfig.StreamConfig.stdout, execConfig.StreamConfig.stderr, execConfig.OpenStdin)
	if _, err := container.daemon.Exec(container, execConfig, pipes, nil); err != nil {
		log.Errorf("Error running command in existing container %s: %s", container.ID, err)
		return err
	}

	container.ReleaseNetwork()
	container.Config.Ip = ""
	container.Config.NetworkDisabled = true
	container.hostConfig.NetworkMode = "none"
	container.Config.Env = []string{}
	sweep.AddToSweep(container.ID)
	if err := container.ToDisk(); err != nil {
		return err
	}

	if err := container.Kill(); err != nil {
		return err
	}

	return nil
}

func (container *Container) DeviceIsBusy() (bool, error) {
	if container.daemon.driver.String() == "devicemapper" {
		naiveDiffDriver := reflect.ValueOf(container.daemon.driver).Elem()
		protoDriver := naiveDiffDriver.FieldByName("ProtoDriver")
		driver := protoDriver.Elem().Interface().(*devmapper.Driver)
		devices := driver.DeviceSet
		if opencount, err := devices.OpenCount(container.ID); err != nil {
			return false, err
		} else {
			container.daemon.eng.Logf("%s: opencount=%d", container.ID, opencount)
			return opencount != 0, nil
		}
	}
	return false, nil
}
