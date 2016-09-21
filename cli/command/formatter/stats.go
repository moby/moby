package formatter

import (
	"fmt"
	"sync"

	"github.com/docker/go-units"
)

const (
	defaultStatsTableFormat    = "table {{.Container}}\t{{.CPUPrec}}\t{{.MemUsage}}\t{{.MemPrec}}\t{{.NetIO}}\t{{.BlockIO}}\t{{.PIDs}}"
	winDefaultStatsTableFormat = "table {{.Container}}\t{{.CPUPrec}}\t{{{.MemUsage}}\t{.NetIO}}\t{{.BlockIO}}"
	emptyStatsTableFormat      = "Waiting for statistics..."

	containerHeader  = "CONTAINER"
	cpuPrecHeader    = "CPU %"
	netIOHeader      = "NET I/O"
	blockIOHeader    = "BLOCK I/O"
	winMemPrecHeader = "PRIV WORKING SET"  // Used only on Window
	memPrecHeader    = "MEM %"             // Used only on Linux
	memUseHeader     = "MEM USAGE / LIMIT" // Used only on Linux
	pidsHeader       = "PIDS"              // Used only on Linux
)

// ContainerStatsAttrs represents the statistics data collected from a container.
type ContainerStatsAttrs struct {
	Windows          bool
	Name             string
	CPUPercentage    float64
	Memory           float64 // On Windows this is the private working set
	MemoryLimit      float64 // Not used on Windows
	MemoryPercentage float64 // Not used on Windows
	NetworkRx        float64
	NetworkTx        float64
	BlockRead        float64
	BlockWrite       float64
	PidsCurrent      uint64 // Not used on Windows
}

// ContainerStats represents the containers statistics data.
type ContainerStats struct {
	Mu sync.RWMutex
	ContainerStatsAttrs
	Err error
}

// NewStatsFormat returns a format for rendering an CStatsContext
func NewStatsFormat(source, osType string) Format {
	if source == TableFormatKey {
		if osType == "windows" {
			return Format(winDefaultStatsTableFormat)
		}
		return Format(defaultStatsTableFormat)
	}
	return Format(source)
}

// NewContainerStats returns a new ContainerStats entity and sets in it the given name
func NewContainerStats(name, osType string) *ContainerStats {
	return &ContainerStats{
		ContainerStatsAttrs: ContainerStatsAttrs{
			Name:    name,
			Windows: (osType == "windows"),
		},
	}
}

// ContainerStatsWrite renders the context for a list of containers statistics
func ContainerStatsWrite(ctx Context, containerStats []*ContainerStats) error {
	render := func(format func(subContext subContext) error) error {
		for _, cstats := range containerStats {
			cstats.Mu.RLock()
			cstatsAttrs := cstats.ContainerStatsAttrs
			cstats.Mu.RUnlock()
			containerStatsCtx := &containerStatsContext{
				s: cstatsAttrs,
			}
			if err := format(containerStatsCtx); err != nil {
				return err
			}
		}
		return nil
	}
	return ctx.Write(&containerStatsContext{}, render)
}

type containerStatsContext struct {
	HeaderContext
	s ContainerStatsAttrs
}

func (c *containerStatsContext) Container() string {
	c.AddHeader(containerHeader)
	return c.s.Name
}

func (c *containerStatsContext) CPUPrec() string {
	c.AddHeader(cpuPrecHeader)
	return fmt.Sprintf("%.2f%%", c.s.CPUPercentage)
}

func (c *containerStatsContext) MemUsage() string {
	c.AddHeader(memUseHeader)
	if !c.s.Windows {
		return fmt.Sprintf("%s / %s", units.BytesSize(c.s.Memory), units.BytesSize(c.s.MemoryLimit))
	}
	return fmt.Sprintf("-- / --")
}

func (c *containerStatsContext) MemPrec() string {
	header := memPrecHeader
	if c.s.Windows {
		header = winMemPrecHeader
	}
	c.AddHeader(header)
	return fmt.Sprintf("%.2f%%", c.s.MemoryPercentage)
}

func (c *containerStatsContext) NetIO() string {
	c.AddHeader(netIOHeader)
	return fmt.Sprintf("%s / %s", units.HumanSizeWithPrecision(c.s.NetworkRx, 3), units.HumanSizeWithPrecision(c.s.NetworkTx, 3))
}

func (c *containerStatsContext) BlockIO() string {
	c.AddHeader(blockIOHeader)
	return fmt.Sprintf("%s / %s", units.HumanSizeWithPrecision(c.s.BlockRead, 3), units.HumanSizeWithPrecision(c.s.BlockWrite, 3))
}

func (c *containerStatsContext) PIDs() string {
	c.AddHeader(pidsHeader)
	if !c.s.Windows {
		return fmt.Sprintf("%d", c.s.PidsCurrent)
	}
	return fmt.Sprintf("-")
}
