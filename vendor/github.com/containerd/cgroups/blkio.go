package cgroups

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func NewBlkio(root string) *blkioController {
	return &blkioController{
		root: filepath.Join(root, string(Blkio)),
	}
}

type blkioController struct {
	root string
}

func (b *blkioController) Name() Name {
	return Blkio
}

func (b *blkioController) Path(path string) string {
	return filepath.Join(b.root, path)
}

func (b *blkioController) Create(path string, resources *specs.LinuxResources) error {
	if err := os.MkdirAll(b.Path(path), defaultDirPerm); err != nil {
		return err
	}
	if resources.BlockIO == nil {
		return nil
	}
	for _, t := range createBlkioSettings(resources.BlockIO) {
		if t.value != nil {
			if err := ioutil.WriteFile(
				filepath.Join(b.Path(path), fmt.Sprintf("blkio.%s", t.name)),
				t.format(t.value),
				defaultFilePerm,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *blkioController) Update(path string, resources *specs.LinuxResources) error {
	return b.Create(path, resources)
}

func (b *blkioController) Stat(path string, stats *Metrics) error {
	stats.Blkio = &BlkIOStat{}
	settings := []blkioStatSettings{
		{
			name:  "throttle.io_serviced",
			entry: &stats.Blkio.IoServicedRecursive,
		},
		{
			name:  "throttle.io_service_bytes",
			entry: &stats.Blkio.IoServiceBytesRecursive,
		},
	}
	// Try to read CFQ stats available on all CFQ enabled kernels first
	if _, err := os.Lstat(filepath.Join(b.Path(path), fmt.Sprintf("blkio.io_serviced_recursive"))); err == nil {
		settings = append(settings,
			blkioStatSettings{
				name:  "sectors_recursive",
				entry: &stats.Blkio.SectorsRecursive,
			},
			blkioStatSettings{
				name:  "io_service_bytes_recursive",
				entry: &stats.Blkio.IoServiceBytesRecursive,
			},
			blkioStatSettings{
				name:  "io_serviced_recursive",
				entry: &stats.Blkio.IoServicedRecursive,
			},
			blkioStatSettings{
				name:  "io_queued_recursive",
				entry: &stats.Blkio.IoQueuedRecursive,
			},
			blkioStatSettings{
				name:  "io_service_time_recursive",
				entry: &stats.Blkio.IoServiceTimeRecursive,
			},
			blkioStatSettings{
				name:  "io_wait_time_recursive",
				entry: &stats.Blkio.IoWaitTimeRecursive,
			},
			blkioStatSettings{
				name:  "io_merged_recursive",
				entry: &stats.Blkio.IoMergedRecursive,
			},
			blkioStatSettings{
				name:  "time_recursive",
				entry: &stats.Blkio.IoTimeRecursive,
			},
		)
	}

	devices, err := getDevices("/dev")
	if err != nil {
		return err
	}

	for _, t := range settings {
		if err := b.readEntry(devices, path, t.name, t.entry); err != nil {
			return err
		}
	}
	return nil
}

func (b *blkioController) readEntry(devices map[deviceKey]string, path, name string, entry *[]*BlkIOEntry) error {
	f, err := os.Open(filepath.Join(b.Path(path), fmt.Sprintf("blkio.%s", name)))
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if err := sc.Err(); err != nil {
			return err
		}
		// format: dev type amount
		fields := strings.FieldsFunc(sc.Text(), splitBlkIOStatLine)
		if len(fields) < 3 {
			if len(fields) == 2 && fields[0] == "Total" {
				// skip total line
				continue
			} else {
				return fmt.Errorf("Invalid line found while parsing %s: %s", path, sc.Text())
			}
		}
		major, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return err
		}
		minor, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return err
		}
		op := ""
		valueField := 2
		if len(fields) == 4 {
			op = fields[2]
			valueField = 3
		}
		v, err := strconv.ParseUint(fields[valueField], 10, 64)
		if err != nil {
			return err
		}
		*entry = append(*entry, &BlkIOEntry{
			Device: devices[deviceKey{major, minor}],
			Major:  major,
			Minor:  minor,
			Op:     op,
			Value:  v,
		})
	}
	return nil
}

func createBlkioSettings(blkio *specs.LinuxBlockIO) []blkioSettings {
	settings := []blkioSettings{
		{
			name:   "weight",
			value:  blkio.Weight,
			format: uintf,
		},
		{
			name:   "leaf_weight",
			value:  blkio.LeafWeight,
			format: uintf,
		},
	}
	for _, wd := range blkio.WeightDevice {
		settings = append(settings,
			blkioSettings{
				name:   "weight_device",
				value:  wd,
				format: weightdev,
			},
			blkioSettings{
				name:   "leaf_weight_device",
				value:  wd,
				format: weightleafdev,
			})
	}
	for _, t := range []struct {
		name string
		list []specs.LinuxThrottleDevice
	}{
		{
			name: "throttle.read_bps_device",
			list: blkio.ThrottleReadBpsDevice,
		},
		{
			name: "throttle.read_iops_device",
			list: blkio.ThrottleReadIOPSDevice,
		},
		{
			name: "throttle.write_bps_device",
			list: blkio.ThrottleWriteBpsDevice,
		},
		{
			name: "throttle.write_iops_device",
			list: blkio.ThrottleWriteIOPSDevice,
		},
	} {
		for _, td := range t.list {
			settings = append(settings, blkioSettings{
				name:   t.name,
				value:  td,
				format: throttleddev,
			})
		}
	}
	return settings
}

type blkioSettings struct {
	name   string
	value  interface{}
	format func(v interface{}) []byte
}

type blkioStatSettings struct {
	name  string
	entry *[]*BlkIOEntry
}

func uintf(v interface{}) []byte {
	return []byte(strconv.FormatUint(uint64(*v.(*uint16)), 10))
}

func weightdev(v interface{}) []byte {
	wd := v.(specs.LinuxWeightDevice)
	return []byte(fmt.Sprintf("%d:%d %d", wd.Major, wd.Minor, wd.Weight))
}

func weightleafdev(v interface{}) []byte {
	wd := v.(specs.LinuxWeightDevice)
	return []byte(fmt.Sprintf("%d:%d %d", wd.Major, wd.Minor, wd.LeafWeight))
}

func throttleddev(v interface{}) []byte {
	td := v.(specs.LinuxThrottleDevice)
	return []byte(fmt.Sprintf("%d:%d %d", td.Major, td.Minor, td.Rate))
}

func splitBlkIOStatLine(r rune) bool {
	return r == ' ' || r == ':'
}

type deviceKey struct {
	major, minor uint64
}

// getDevices makes a best effort attempt to read all the devices into a map
// keyed by major and minor number. Since devices may be mapped multiple times,
// we err on taking the first occurrence.
func getDevices(path string) (map[deviceKey]string, error) {
	// TODO(stevvooe): We are ignoring lots of errors. It might be kind of
	// challenging to debug this if we aren't mapping devices correctly.
	// Consider logging these errors.
	devices := map[deviceKey]string{}
	if err := filepath.Walk(path, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		switch {
		case fi.IsDir():
			switch fi.Name() {
			case "pts", "shm", "fd", "mqueue", ".lxc", ".lxd-mounts":
				return filepath.SkipDir
			default:
				return nil
			}
		case fi.Name() == "console":
			return nil
		default:
			if fi.Mode()&os.ModeDevice == 0 {
				// skip non-devices
				return nil
			}

			st, ok := fi.Sys().(*syscall.Stat_t)
			if !ok {
				return fmt.Errorf("%s: unable to convert to system stat", p)
			}

			key := deviceKey{major(st.Rdev), minor(st.Rdev)}
			if _, ok := devices[key]; ok {
				return nil // skip it if we have already populated the path.
			}

			devices[key] = p
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return devices, nil
}

func major(devNumber uint64) uint64 {
	return (devNumber >> 8) & 0xfff
}

func minor(devNumber uint64) uint64 {
	return (devNumber & 0xff) | ((devNumber >> 12) & 0xfff00)
}
