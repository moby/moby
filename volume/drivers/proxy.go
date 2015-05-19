package volumedrivers

// currently created by hand. generation tool would generate this like:
// $ rpc-gen volume/drivers/api.go VolumeDriver > volume/drivers/proxy.go

type volumeDriverRequest struct {
	Name string
}

type volumeDriverResponse struct {
	Mountpoint string `json:",ommitempty"`
	Err        error  `json:",ommitempty"`
}

type volumeDriverProxy struct {
	c client
}

func (pp *volumeDriverProxy) Create(name string) error {
	args := volumeDriverRequest{name}
	var ret volumeDriverResponse
	err := pp.c.Call("VolumeDriver.Create", args, &ret)
	if err != nil {
		return err
	}
	return ret.Err
}

func (pp *volumeDriverProxy) Remove(name string) error {
	args := volumeDriverRequest{name}
	var ret volumeDriverResponse
	err := pp.c.Call("VolumeDriver.Remove", args, &ret)
	if err != nil {
		return err
	}
	return ret.Err
}

func (pp *volumeDriverProxy) Path(name string) (string, error) {
	args := volumeDriverRequest{name}
	var ret volumeDriverResponse
	if err := pp.c.Call("VolumeDriver.Path", args, &ret); err != nil {
		return "", err
	}
	return ret.Mountpoint, ret.Err
}

func (pp *volumeDriverProxy) Mount(name string) (string, error) {
	args := volumeDriverRequest{name}
	var ret volumeDriverResponse
	if err := pp.c.Call("VolumeDriver.Mount", args, &ret); err != nil {
		return "", err
	}
	return ret.Mountpoint, ret.Err
}

func (pp *volumeDriverProxy) Unmount(name string) error {
	args := volumeDriverRequest{name}
	var ret volumeDriverResponse
	err := pp.c.Call("VolumeDriver.Unmount", args, &ret)
	if err != nil {
		return err
	}
	return ret.Err
}
