package wazevoapi

import (
	"fmt"
	"os"
	"strconv"
	"sync"
)

var PerfMap *Perfmap

func init() {
	if PerfMapEnabled {
		pid := os.Getpid()
		filename := "/tmp/perf-" + strconv.Itoa(pid) + ".map"

		fh, err := os.OpenFile(filename, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0o644)
		if err != nil {
			panic(err)
		}

		PerfMap = &Perfmap{fh: fh}
	}
}

// Perfmap holds perfmap entries to be flushed into a perfmap file.
type Perfmap struct {
	entries []entry
	mux     sync.Mutex
	fh      *os.File
}

type entry struct {
	index  int
	offset int64
	size   uint64
	name   string
}

func (f *Perfmap) Lock() {
	f.mux.Lock()
}

func (f *Perfmap) Unlock() {
	f.mux.Unlock()
}

// AddModuleEntry adds a perfmap entry into the perfmap file.
// index is the index of the function in the module, offset is the offset of the function in the module,
// size is the size of the function, and name is the name of the function.
//
// Note that the entries are not flushed into the perfmap file until Flush is called,
// and the entries are module-scoped; Perfmap must be locked until Flush is called.
func (f *Perfmap) AddModuleEntry(index int, offset int64, size uint64, name string) {
	e := entry{index: index, offset: offset, size: size, name: name}
	if f.entries == nil {
		f.entries = []entry{e}
		return
	}
	f.entries = append(f.entries, e)
}

// Flush writes the perfmap entries into the perfmap file where the entries are adjusted by the given `addr` and `functionOffsets`.
func (f *Perfmap) Flush(addr uintptr, functionOffsets []int) {
	defer func() {
		_ = f.fh.Sync()
	}()

	for _, e := range f.entries {
		if _, err := f.fh.WriteString(fmt.Sprintf("%x %s %s\n",
			uintptr(e.offset)+addr+uintptr(functionOffsets[e.index]),
			strconv.FormatUint(e.size, 16),
			e.name,
		)); err != nil {
			panic(err)
		}
	}
	f.entries = f.entries[:0]
}

// Clear clears the perfmap entries not yet flushed.
func (f *Perfmap) Clear() {
	f.entries = f.entries[:0]
}

// AddEntry writes a perfmap entry directly into the perfmap file, not using the entries.
func (f *Perfmap) AddEntry(addr uintptr, size uint64, name string) {
	_, err := f.fh.WriteString(fmt.Sprintf("%x %s %s\n",
		addr,
		strconv.FormatUint(size, 16),
		name,
	))
	if err != nil {
		panic(err)
	}
}
