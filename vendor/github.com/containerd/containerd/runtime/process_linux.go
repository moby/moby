// +build linux

package runtime

import (
	"io/ioutil"
	"path/filepath"
	"strconv"
)

func (p *process) getPidFromFile() (int, error) {
	data, err := ioutil.ReadFile(filepath.Join(p.root, "pid"))
	if err != nil {
		return -1, err
	}
	i, err := strconv.Atoi(string(data))
	if err != nil {
		return -1, errInvalidPidInt
	}
	p.pid = i
	return i, nil
}
