package formatter

import (
	"fmt"
	"sync"

	units "github.com/docker/go-units"
)

const (
	winOSType                  = "windows"
	defaultStatsTableFormat    = "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}\t{{.NetIO}}\t{{.BlockIO}}\t{{.PIDs}}"
	winDefaultStatsTableFormat = "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}"

	containerHeader = "CONTAINER"
	cpuPercHeader   = "CPU %"
	netIOHeader     = "NET I/O"
	blockIOHeader   = "BLOCK I/O"
	memPercHeader   = "MEM %"             // Used only on Linux
	winMemUseHeader = "PRIV WORKING SET"  // Used only on Windows
	memUseHeader    = "MEM USAGE / LIMIT" // Used only on Linux
	pidsHeader      = "PIDS"              // Used only on Linux
)

// StatsEntry represents represents the statistics data collected from a container
type StatsEntry struct {
	Container        string
	Name             string
	ID               string
	CPUPercentage    float64
	Memory           float64 // On Windows this is the private working set
	MemoryLimit      float64 // Not used on Windows
	MemoryPercentage float64 // Not used on Windows
	NetworkRx        float64
	NetworkTx        float64
	BlockRead        float64
	BlockWrite       float64
	PidsCurrent      uint64 // Not used on Windows
	IsInvalid        bool
}

// ContainerStats represents an entity to store containers statistics synchronously
type ContainerStats struct {
	mutex sync.Mutex
	StatsEntry
	err error
}

// GetError returns the container statistics error.
// This is used to determine whether the statistics are valid or not
func (cs *ContainerStats) GetError() error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	return cs.err
}

// SetErrorAndReset zeroes all the container statistics and store the error.
// It is used when receiving time out error during statistics collecting to reduce lock overhead
func (cs *ContainerStats) SetErrorAndReset(err error) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	cs.CPUPercentage = 0
	cs.Memory = 0
	cs.MemoryPercentage = 0
	cs.MemoryLimit = 0
	cs.NetworkRx = 0
	cs.NetworkTx = 0
	cs.BlockRead = 0
	cs.BlockWrite = 0
	cs.PidsCurrent = 0
	cs.err = err
	cs.IsInvalid = true
}

// SetError sets container statistics error
func (cs *ContainerStats) SetError(err error) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	cs.err = err
	if err != nil {
		cs.IsInvalid = true
	}
}

// SetStatistics set the container statistics
func (cs *ContainerStats) SetStatistics(s StatsEntry) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	s.Container = cs.Container
	cs.StatsEntry = s
}

// GetStatistics returns container statistics with other meta data such as the container name
func (cs *ContainerStats) GetStatistics() StatsEntry {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	return cs.StatsEntry
}

// NewStatsFormat returns a format for rendering an CStatsContext
func NewStatsFormat(source, osType string) Format {
	if source == TableFormatKey {
		if osType == winOSType {
			return Format(winDefaultStatsTableFormat)
		}
		return Format(defaultStatsTableFormat)
	}
	return Format(source)
}

// NewContainerStats returns a new ContainerStats entity and sets in it the given name
func NewContainerStats(container, osType string) *ContainerStats {
	return &ContainerStats{
		StatsEntry: StatsEntry{Container: container},
	}
}

// ContainerStatsWrite renders the context for a list of containers statistics
func ContainerStatsWrite(ctx Context, containerStats []StatsEntry, osType string) error {
	render := func(format func(subContext subContext) error) error {
		for _, cstats := range containerStats {
			containerStatsCtx := &containerStatsContext{
				s:  cstats,
				os: osType,
			}
			if err := format(containerStatsCtx); err != nil {
				return err
			}
		}
		return nil
	}
	memUsage := memUseHeader
	if osType == winOSType {
		memUsage = winMemUseHeader
	}
	containerStatsCtx := containerStatsContext{}
	containerStatsCtx.header = map[string]string{
		"Container": containerHeader,
		"Name":      nameHeader,
		"ID":        containerIDHeader,
		"CPUPerc":   cpuPercHeader,
		"MemUsage":  memUsage,
		"MemPerc":   memPercHeader,
		"NetIO":     netIOHeader,
		"BlockIO":   blockIOHeader,
		"PIDs":      pidsHeader,
	}
	containerStatsCtx.os = osType
	return ctx.Write(&containerStatsCtx, render)
}

type containerStatsContext struct {
	HeaderContext
	s  StatsEntry
	os string
}

func (c *containerStatsContext) MarshalJSON() ([]byte, error) {
	return marshalJSON(c)
}

func (c *containerStatsContext) Container() string {
	return c.s.Container
}

func (c *containerStatsContext) Name() string {
	if len(c.s.Name) > 1 {
		return c.s.Name[1:]
	}
	return "--"
}

func (c *containerStatsContext) ID() string {
	return c.s.ID
}

func (c *containerStatsContext) CPUPerc() string {
	if c.s.IsInvalid {
		return fmt.Sprintf("--")
	}
	return fmt.Sprintf("%.2f%%", c.s.CPUPercentage)
}

func (c *containerStatsContext) MemUsage() string {
	if c.s.IsInvalid {
		return fmt.Sprintf("-- / --")
	}
	if c.os == winOSType {
		return fmt.Sprintf("%s", units.BytesSize(c.s.Memory))
	}
	return fmt.Sprintf("%s / %s", units.BytesSize(c.s.Memory), units.BytesSize(c.s.MemoryLimit))
}

func (c *containerStatsContext) MemPerc() string {
	if c.s.IsInvalid || c.os == winOSType {
		return fmt.Sprintf("--")
	}
	return fmt.Sprintf("%.2f%%", c.s.MemoryPercentage)
}

func (c *containerStatsContext) NetIO() string {
	if c.s.IsInvalid {
		return fmt.Sprintf("--")
	}
	return fmt.Sprintf("%s / %s", units.HumanSizeWithPrecision(c.s.NetworkRx, 3), units.HumanSizeWithPrecision(c.s.NetworkTx, 3))
}

func (c *containerStatsContext) BlockIO() string {
	if c.s.IsInvalid {
		return fmt.Sprintf("--")
	}
	return fmt.Sprintf("%s / %s", units.HumanSizeWithPrecision(c.s.BlockRead, 3), units.HumanSizeWithPrecision(c.s.BlockWrite, 3))
}

func (c *containerStatsContext) PIDs() string {
	if c.s.IsInvalid || c.os == winOSType {
		return fmt.Sprintf("--")
	}
	return fmt.Sprintf("%d", c.s.PidsCurrent)
}
