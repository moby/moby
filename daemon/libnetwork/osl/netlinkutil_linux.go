package osl

import (
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

type deviceFlags uint32

var deviceFlagStrings = map[deviceFlags]string{
	unix.IFF_UP:          "IFF_UP",
	unix.IFF_BROADCAST:   "IFF_BROADCAST",
	unix.IFF_DEBUG:       "IFF_DEBUG",
	unix.IFF_LOOPBACK:    "IFF_LOOPBACK",
	unix.IFF_POINTOPOINT: "IFF_POINTOPOINT",
	unix.IFF_RUNNING:     "IFF_RUNNING",
	unix.IFF_NOARP:       "IFF_NOARP",
	unix.IFF_PROMISC:     "IFF_PROMISC",
	unix.IFF_NOTRAILERS:  "IFF_NOTRAILERS",
	unix.IFF_ALLMULTI:    "IFF_ALLMULTI",
	unix.IFF_MASTER:      "IFF_MASTER",
	unix.IFF_SLAVE:       "IFF_SLAVE",
	unix.IFF_MULTICAST:   "IFF_MULTICAST",
	unix.IFF_PORTSEL:     "IFF_PORTSEL",
	unix.IFF_AUTOMEDIA:   "IFF_AUTOMEDIA",
	unix.IFF_DYNAMIC:     "IFF_DYNAMIC",
	unix.IFF_LOWER_UP:    "IFF_LOWER_UP",
	unix.IFF_DORMANT:     "IFF_DORMANT",
	unix.IFF_ECHO:        "IFF_ECHO",
}

func (d deviceFlags) String() string {
	var (
		flags   []string
		unknown uint32
	)

	for i := range uint(32) {
		if d&(1<<i) != 0 {
			if s, ok := deviceFlagStrings[deviceFlags(1<<i)]; ok {
				flags = append(flags, s)
			} else {
				unknown |= 1 << i
			}
		}
	}
	if unknown != 0 {
		flags = append(flags, "0x"+strconv.FormatUint(uint64(unknown), 16))
	}

	return "deviceFlags(" + strings.Join(flags, " | ") + ")"
}
