// +build linux,cgo

package devicemapper

import "C"

import (
	"strings"
)

// Due to the way cgo works this has to be in a separate file, as devmapper.go has
// definitions in the cgo block, which is incompatible with using "//export"

// DevmapperLogCallback exports the devmapper log callback for cgo.
//export DevmapperLogCallback
func DevmapperLogCallback(level C.int, file *C.char, line C.int, dmErrnoOrClass C.int, message *C.char) {
	msg := C.GoString(message)
	if level < 7 {
		if strings.Contains(msg, "busy") {
			dmSawBusy = true
		}

		if strings.Contains(msg, "File exists") {
			dmSawExist = true
		}

		if strings.Contains(msg, "No such device or address") {
			dmSawEnxio = true
		}
	}

	if dmLogger != nil {
		dmLogger.DMLog(int(level), C.GoString(file), int(line), int(dmErrnoOrClass), msg)
	}
}
