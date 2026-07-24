//go:build windows && go1.26

package archive

// windows_O_FILE_FLAG_SEQUENTIAL_SCAN matches [golang.org/x/sys/windows.O_FILE_FLAG_SEQUENTIAL_SCAN].
// Starting in Go 1.26, os.OpenFile supports passing this flag through.
//
// TODO(thaJeztah): use windows.O_FILE_FLAG_SEQUENTIAL_SCAN once we drop Go <1.26.
const windows_O_FILE_FLAG_SEQUENTIAL_SCAN = 0x08000000
