package fs

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dotcloud/docker/pkg/cgroups"
)

type blkioGroup struct {
}

func (s *blkioGroup) Set(d *data) error {
	// we just want to join this group even though we don't set anything
	if _, err := d.join("blkio"); err != nil && err != cgroups.ErrNotFound {
		return err
	}
	return nil
}

func (s *blkioGroup) Remove(d *data) error {
	return removePath(d.path("blkio"))
}

/*
examples:

    blkio.sectors
    8:0 6792

    blkio.io_service_bytes
    8:0 Read 1282048
    8:0 Write 2195456
    8:0 Sync 2195456
    8:0 Async 1282048
    8:0 Total 3477504
    Total 3477504

    blkio.io_serviced
    8:0 Read 124
    8:0 Write 104
    8:0 Sync 104
    8:0 Async 124
    8:0 Total 228
    Total 228

    blkio.io_queued
    8:0 Read 0
    8:0 Write 0
    8:0 Sync 0
    8:0 Async 0
    8:0 Total 0
    Total 0
*/
func (s *blkioGroup) Stats(d *data) (map[string]float64, error) {
	var (
		paramData = make(map[string]float64)
		params    = []string{
			"io_service_bytes_recursive",
			"io_serviced_recursive",
			"io_queued_recursive",
		}
	)

	path, err := d.path("blkio")
	if err != nil {
		return nil, err
	}

	k, v, err := s.getSectors(path)
	if err != nil {
		return nil, err
	}
	paramData[fmt.Sprintf("blkio.sectors_recursive:%s", k)] = v

	for _, param := range params {
		f, err := os.Open(filepath.Join(path, fmt.Sprintf("blkio.%s", param)))
		if err != nil {
			return nil, err
		}
		defer f.Close()

		sc := bufio.NewScanner(f)
		for sc.Scan() {
			// format: dev type amount
			fields := strings.Fields(sc.Text())
			switch len(fields) {
			case 3:
				v, err := strconv.ParseFloat(fields[2], 64)
				if err != nil {
					return nil, err
				}
				paramData[fmt.Sprintf("%s:%s:%s", param, fields[0], fields[1])] = v
			case 2:
				// this is the total line, skip
			default:
				return nil, ErrNotValidFormat
			}
		}
	}
	return paramData, nil
}

func (s *blkioGroup) getSectors(path string) (string, float64, error) {
	f, err := os.Open(filepath.Join(path, "blkio.sectors_recursive"))
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return "", 0, err
	}
	return getCgroupParamKeyValue(string(data))
}
