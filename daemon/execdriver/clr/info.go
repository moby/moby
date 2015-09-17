// +build linux

package clr

import (
	"errors"
	"strconv"
	"strings"
)

var (
	ErrCannotParse = errors.New("cannot parse raw input")
)

type clrInfo struct {
	Running bool
	Pid     int
}

func parseClrInfo(name, raw string) (*clrInfo, error) {
	if raw == "" {
		return nil, ErrCannotParse
	}
	var (
		err  error
		info = &clrInfo{}
	)

	fields := strings.Fields(strings.TrimSpace(raw))

	// The format is expected to be:
	//
	// <pid> <name> <state>
	//
	if len(fields) != 3 {
		return nil, ErrCannotParse
	}

	info.Pid, err = strconv.Atoi(fields[0])
	if err != nil {
		return nil, ErrCannotParse
	}

	if fields[1] != name {
		return nil, ErrCannotParse
	}

	info.Running = fields[2] == "running"

	return info, nil
}
