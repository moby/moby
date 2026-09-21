package ebpf

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/cilium/ebpf/internal/efw"
	"github.com/cilium/ebpf/internal/sys"
	"github.com/cilium/ebpf/internal/unix"
)

func loadCollectionFromNativeImage(file string) (_ *Collection, err error) {
	mapFds := make([]efw.FD, 32)
	programFds := make([]efw.FD, 32)
	var maps map[string]*Map
	var programs map[string]*Program

	defer func() {
		if err == nil {
			return
		}

		for _, fd := range append(mapFds, programFds...) {
			// efW never uses fd 0.
			if fd != 0 {
				_ = efw.EbpfCloseFd(int(fd))
			}
		}

		for _, m := range maps {
			_ = m.Close()
		}

		for _, p := range programs {
			_ = p.Close()
		}
	}()

	nMaps, nPrograms, err := efw.EbpfObjectLoadNativeFds(file, mapFds, programFds)
	if errors.Is(err, efw.EBPF_NO_MEMORY) && (nMaps > len(mapFds) || nPrograms > len(programFds)) {
		mapFds = make([]efw.FD, nMaps)
		programFds = make([]efw.FD, nPrograms)

		nMaps, nPrograms, err = efw.EbpfObjectLoadNativeFds(file, mapFds, programFds)
	}
	if err != nil {
		return nil, err
	}

	mapFds = mapFds[:nMaps]
	programFds = programFds[:nPrograms]

	// The maximum length of a name is only 16 bytes on Linux, longer names
	// are truncated. This is not a problem when loading from an ELF, since
	// we get the full object name from the symbol table.
	// When loading a native image we do not have this luxury. Use an efW native
	// API to retrieve up to 64 bytes of the object name.

	maps = make(map[string]*Map, len(mapFds))
	for _, raw := range mapFds {
		fd, err := sys.NewFD(int(raw))
		if err != nil {
			return nil, err
		}

		m, mapErr := newMapFromFD(fd)
		if mapErr != nil {
			_ = fd.Close()
			return nil, mapErr
		}

		var efwMapInfo efw.BpfMapInfo
		size := uint32(unsafe.Sizeof(efwMapInfo))
		_, err = efw.EbpfObjectGetInfoByFd(m.FD(), unsafe.Pointer(&efwMapInfo), &size)
		if err != nil {
			_ = m.Close()
			return nil, err
		}

		if size >= uint32(unsafe.Offsetof(efwMapInfo.Name)+unsafe.Sizeof(efwMapInfo.Name)) {
			m.name = unix.ByteSliceToString(efwMapInfo.Name[:])
		}

		if m.name == "" {
			_ = m.Close()
			return nil, fmt.Errorf("unnamed map")
		}

		if _, ok := maps[m.name]; ok {
			return nil, fmt.Errorf("duplicate map with the same name: %s", m.name)
		}

		maps[m.name] = m
	}

	programs = make(map[string]*Program, len(programFds))
	for _, raw := range programFds {
		fd, err := sys.NewFD(int(raw))
		if err != nil {
			return nil, err
		}

		program, err := newProgramFromFD(fd)
		if err != nil {
			_ = fd.Close()
			return nil, err
		}

		var efwProgInfo efw.BpfProgInfo
		size := uint32(unsafe.Sizeof(efwProgInfo))
		_, err = efw.EbpfObjectGetInfoByFd(program.FD(), unsafe.Pointer(&efwProgInfo), &size)
		if err != nil {
			_ = program.Close()
			return nil, err
		}

		if size >= uint32(unsafe.Offsetof(efwProgInfo.Name)+unsafe.Sizeof(efwProgInfo.Name)) {
			program.name = unix.ByteSliceToString(efwProgInfo.Name[:])
		}

		if program.name == "" {
			_ = program.Close()
			return nil, fmt.Errorf("unnamed program")
		}

		if _, ok := programs[program.name]; ok {
			_ = program.Close()
			return nil, fmt.Errorf("duplicate program with the same name: %s", program.name)
		}

		programs[program.name] = program
	}

	return &Collection{programs, maps, nil}, nil
}
