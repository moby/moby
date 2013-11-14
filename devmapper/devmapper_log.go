package devmapper

import "C"

// Due to the way cgo works this has to be in a separate file, as devmapper.go has
// definitions in the cgo block, which is incompatible with using "//export"

//export DevmapperLogCallback
func DevmapperLogCallback(level C.int, file *C.char, line C.int, dm_errno_or_class C.int, message *C.char) {
	if dmLogger != nil {
		dmLogger.log(int(level), C.GoString(file), int(line), int(dm_errno_or_class), C.GoString(message))
	}
}
