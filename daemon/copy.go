package daemon

import "io"

func (daemon *Daemon) ContainerCopy(name string, res string) (io.ReadCloser, error) {
	container, err := daemon.Get(name)
	if err != nil {
		return nil, err
	}

	if res[0] == '/' {
		res = res[1:]
	}

	return container.Copy(res)
}
