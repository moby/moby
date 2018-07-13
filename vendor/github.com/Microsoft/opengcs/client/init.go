// +build windows

package client

import (
	"os"
	"strconv"
)

var (
	logDataFromUVM int64
)

func init() {
	bytes := os.Getenv("OPENGCS_LOG_DATA_FROM_UVM")
	if len(bytes) == 0 {
		return
	}
	u, err := strconv.ParseUint(bytes, 10, 32)
	if err != nil {
		return
	}
	logDataFromUVM = int64(u)
}
