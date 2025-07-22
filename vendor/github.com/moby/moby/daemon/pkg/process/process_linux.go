package process

import (
	"bytes"
	"fmt"
	"os"
)

func zombie(pid int) (bool, error) {
	if pid < 1 {
		return false, nil
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if cols := bytes.SplitN(data, []byte(" "), 4); len(cols) >= 3 && string(cols[2]) == "Z" {
		return true, nil
	}
	return false, nil
}
