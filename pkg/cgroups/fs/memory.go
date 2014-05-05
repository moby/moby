package fs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type memoryGroup struct {
}

func (s *memoryGroup) Set(d *data) error {
	dir, err := d.join("memory")
	// only return an error for memory if it was not specified
	if err != nil && (d.c.Memory != 0 || d.c.MemoryReservation != 0 || d.c.MemorySwap != 0) {
		return err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(dir)
		}
	}()

	// Only set values if some config was specified.
	if d.c.Memory != 0 || d.c.MemoryReservation != 0 || d.c.MemorySwap != 0 {
		if d.c.Memory != 0 {
			if err := writeFile(dir, "memory.limit_in_bytes", strconv.FormatInt(d.c.Memory, 10)); err != nil {
				return err
			}
		}
		if d.c.MemoryReservation != 0 {
			if err := writeFile(dir, "memory.soft_limit_in_bytes", strconv.FormatInt(d.c.MemoryReservation, 10)); err != nil {
				return err
			}
		}
		// By default, MemorySwap is set to twice the size of RAM.
		// If you want to omit MemorySwap, set it to `-1'.
		if d.c.MemorySwap != -1 {
			if err := writeFile(dir, "memory.memsw.limit_in_bytes", strconv.FormatInt(d.c.Memory*2, 10)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *memoryGroup) Remove(d *data) error {
	return removePath(d.path("memory"))
}

func (s *memoryGroup) Stats(d *data) (map[string]float64, error) {
	paramData := make(map[string]float64)
	path, err := d.path("memory")
	if err != nil {
		return nil, err
	}

	// Set stats from memory.stat.
	statsFile, err := os.Open(filepath.Join(path, "memory.stat"))
	if err != nil {
		return nil, err
	}
	defer statsFile.Close()

	sc := bufio.NewScanner(statsFile)
	for sc.Scan() {
		t, v, err := getCgroupParamKeyValue(sc.Text())
		if err != nil {
			return nil, err
		}
		paramData[t] = v
	}

	// Set memory usage and max historical usage.
	params := []string{
		"usage_in_bytes",
		"max_usage_in_bytes",
	}
	for _, param := range params {
		value, err := getCgroupParamFloat64(path, fmt.Sprintf("memory.%s", param))
		if err != nil {
			return nil, err
		}
		paramData[param] = value
	}

	return paramData, nil
}
