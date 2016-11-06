package formatter

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	units "github.com/docker/go-units"
)

const (
	defaultDiskUsageImageTableFormat     = "table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.CreatedSince}} ago\t{{.VirtualSize}}\t{{.SharedSize}}\t{{.UniqueSize}}\t{{.Containers}}"
	defaultDiskUsageContainerTableFormat = "table {{.ID}}\t{{.Image}}\t{{.Command}}\t{{.LocalVolumes}}\t{{.Size}}\t{{.RunningFor}} ago\t{{.Status}}\t{{.Names}}"
	defaultDiskUsageVolumeTableFormat    = "table {{.Name}}\t{{.Links}}\t{{.Size}}"
	defaultDiskUsageTableFormat          = "table {{.Type}}\t{{.TotalCount}}\t{{.Active}}\t{{.Size}}\t{{.Reclaimable}}"

	typeHeader        = "TYPE"
	totalHeader       = "TOTAL"
	activeHeader      = "ACTIVE"
	reclaimableHeader = "RECLAIMABLE"
	containersHeader  = "CONTAINERS"
	sharedSizeHeader  = "SHARED SIZE"
	uniqueSizeHeader  = "UNIQUE SIZE"
)

// DiskUsageContext contains disk usage specific information required by the formatter, encapsulate a Context struct.
type DiskUsageContext struct {
	Context
	Verbose            bool
	LayersSize         int64
	DownloadCacheUsage types.DownloadCacheUsage
	Images             []*types.ImageSummary
	Containers         []*types.Container
	Volumes            []*types.Volume
}

func (ctx *DiskUsageContext) startSubsection(format string) (*template.Template, error) {
	ctx.buffer = bytes.NewBufferString("")
	ctx.header = ""
	ctx.Format = Format(format)
	ctx.preFormat()

	return ctx.parseFormat()
}

func (ctx *DiskUsageContext) Write() {
	if ctx.Verbose == false {
		ctx.buffer = bytes.NewBufferString("")
		ctx.Format = defaultDiskUsageTableFormat
		ctx.preFormat()

		tmpl, err := ctx.parseFormat()
		if err != nil {
			return
		}

		err = ctx.contextFormat(tmpl, &diskUsageImagesContext{
			totalSize: ctx.LayersSize,
			images:    ctx.Images,
		})
		if err != nil {
			return
		}
		err = ctx.contextFormat(tmpl, &diskUsageContainersContext{
			containers: ctx.Containers,
		})
		if err != nil {
			return
		}

		err = ctx.contextFormat(tmpl, &diskUsageVolumesContext{
			volumes: ctx.Volumes,
		})
		if err != nil {
			return
		}

		err = ctx.contextFormat(tmpl, &diskUsageDownloadCacheContext{
			DownloadCacheUsage: ctx.DownloadCacheUsage,
		})
		if err != nil {
			return
		}

		diskUsageContainersCtx := diskUsageContainersContext{containers: []*types.Container{}}
		diskUsageContainersCtx.header = map[string]string{
			"Type":        typeHeader,
			"TotalCount":  totalHeader,
			"Active":      activeHeader,
			"Size":        sizeHeader,
			"Reclaimable": reclaimableHeader,
		}
		ctx.postFormat(tmpl, &diskUsageContainersCtx)

		return
	}

	// First images
	tmpl, err := ctx.startSubsection(defaultDiskUsageImageTableFormat)
	if err != nil {
		return
	}

	ctx.Output.Write([]byte("Images space usage:\n\n"))
	for _, i := range ctx.Images {
		repo := "<none>"
		tag := "<none>"
		if len(i.RepoTags) > 0 && !isDangling(*i) {
			// Only show the first tag
			ref, err := reference.ParseNormalizedNamed(i.RepoTags[0])
			if err != nil {
				continue
			}
			if nt, ok := ref.(reference.NamedTagged); ok {
				repo = reference.FamiliarName(ref)
				tag = nt.Tag()
			}
		}

		err = ctx.contextFormat(tmpl, &imageContext{
			repo:  repo,
			tag:   tag,
			trunc: true,
			i:     *i,
		})
		if err != nil {
			return
		}
	}
	ctx.postFormat(tmpl, newImageContext())

	// Now containers
	ctx.Output.Write([]byte("\nContainers space usage:\n\n"))
	tmpl, err = ctx.startSubsection(defaultDiskUsageContainerTableFormat)
	if err != nil {
		return
	}
	for _, c := range ctx.Containers {
		// Don't display the virtual size
		c.SizeRootFs = 0
		err = ctx.contextFormat(tmpl, &containerContext{
			trunc: true,
			c:     *c,
		})
		if err != nil {
			return
		}
	}
	ctx.postFormat(tmpl, newContainerContext())

	// And volumes
	ctx.Output.Write([]byte("\nLocal Volumes space usage:\n\n"))
	tmpl, err = ctx.startSubsection(defaultDiskUsageVolumeTableFormat)
	if err != nil {
		return
	}
	for _, v := range ctx.Volumes {
		err = ctx.contextFormat(tmpl, &volumeContext{
			v: *v,
		})
		if err != nil {
			return
		}
	}
	ctx.postFormat(tmpl, newVolumeContext())

	// And download cache
	ctx.Output.Write([]byte("\nDownload Cache space usage:\n\n"))
	tmpl, err = ctx.startSubsection(defaultDiskUsageTableFormat)
	err = ctx.contextFormat(tmpl, &diskUsageDownloadCacheContext{
		DownloadCacheUsage: ctx.DownloadCacheUsage,
	})
	if err != nil {
		return
	}
	ctx.postFormat(tmpl, newDiskUsageDownloadCacheContext())
}

type diskUsageDownloadCacheContext struct {
	HeaderContext
	types.DownloadCacheUsage
}

func newDiskUsageDownloadCacheContext() *diskUsageDownloadCacheContext {
	diskUsageDownloadCacheCtx := &diskUsageDownloadCacheContext{}
	diskUsageDownloadCacheCtx.header = map[string]string{
		"Type":        typeHeader,
		"TotalCount":  totalHeader,
		"Active":      activeHeader,
		"Size":        sizeHeader,
		"Reclaimable": reclaimableHeader,
	}
	return diskUsageDownloadCacheCtx
}

func (c *diskUsageDownloadCacheContext) Type() string {
	return "Download Cache"
}
func (c *diskUsageDownloadCacheContext) TotalCount() string {
	return strconv.FormatInt(c.TotalNumber, 10)
}
func (c *diskUsageDownloadCacheContext) Active() string {
	return strconv.FormatInt(c.ActiveNumber, 10)
}
func (c *diskUsageDownloadCacheContext) Size() string {
	return units.HumanSize(float64(c.TotalBytes))
}
func (c *diskUsageDownloadCacheContext) Reclaimable() string {
	return units.HumanSize(float64(c.TotalBytes - c.ActiveBytes))
}

type diskUsageImagesContext struct {
	HeaderContext
	totalSize int64
	images    []*types.ImageSummary
}

func (c *diskUsageImagesContext) MarshalJSON() ([]byte, error) {
	return marshalJSON(c)
}

func (c *diskUsageImagesContext) Type() string {
	return "Images"
}

func (c *diskUsageImagesContext) TotalCount() string {
	return strconv.FormatInt(int64(len(c.images)), 10)
}

func (c *diskUsageImagesContext) Active() string {
	used := 0
	for _, i := range c.images {
		if i.Containers > 0 {
			used++
		}
	}

	return strconv.FormatInt(int64(used), 10)
}

func (c *diskUsageImagesContext) Size() string {
	return units.HumanSize(float64(c.totalSize))

}

func (c *diskUsageImagesContext) Reclaimable() string {
	var used int64

	for _, i := range c.images {
		if i.Containers != 0 {
			if i.VirtualSize == -1 || i.SharedSize == -1 {
				continue
			}
			used += i.VirtualSize - i.SharedSize
		}
	}

	reclaimable := c.totalSize - used
	if c.totalSize > 0 {
		return fmt.Sprintf("%s (%v%%)", units.HumanSize(float64(reclaimable)), (reclaimable*100)/c.totalSize)
	}
	return units.HumanSize(float64(reclaimable))
}

type diskUsageContainersContext struct {
	HeaderContext
	verbose    bool
	containers []*types.Container
}

func (c *diskUsageContainersContext) MarshalJSON() ([]byte, error) {
	return marshalJSON(c)
}

func (c *diskUsageContainersContext) Type() string {
	return "Containers"
}

func (c *diskUsageContainersContext) TotalCount() string {
	return strconv.FormatInt(int64(len(c.containers)), 10)
}

func (c *diskUsageContainersContext) isActive(container types.Container) bool {
	return strings.Contains(container.State, "running") ||
		strings.Contains(container.State, "paused") ||
		strings.Contains(container.State, "restarting")
}

func (c *diskUsageContainersContext) Active() string {
	used := 0
	for _, container := range c.containers {
		if c.isActive(*container) {
			used++
		}
	}

	return strconv.FormatInt(int64(used), 10)
}

func (c *diskUsageContainersContext) Size() string {
	var size int64

	for _, container := range c.containers {
		size += container.SizeRw
	}

	return units.HumanSize(float64(size))
}

func (c *diskUsageContainersContext) Reclaimable() string {
	var reclaimable int64
	var totalSize int64

	for _, container := range c.containers {
		if !c.isActive(*container) {
			reclaimable += container.SizeRw
		}
		totalSize += container.SizeRw
	}

	if totalSize > 0 {
		return fmt.Sprintf("%s (%v%%)", units.HumanSize(float64(reclaimable)), (reclaimable*100)/totalSize)
	}

	return units.HumanSize(float64(reclaimable))
}

type diskUsageVolumesContext struct {
	HeaderContext
	verbose bool
	volumes []*types.Volume
}

func (c *diskUsageVolumesContext) MarshalJSON() ([]byte, error) {
	return marshalJSON(c)
}

func (c *diskUsageVolumesContext) Type() string {
	return "Local Volumes"
}

func (c *diskUsageVolumesContext) TotalCount() string {
	return strconv.FormatInt(int64(len(c.volumes)), 10)
}

func (c *diskUsageVolumesContext) Active() string {

	used := 0
	for _, v := range c.volumes {
		if v.UsageData.RefCount > 0 {
			used++
		}
	}

	return strconv.FormatInt(int64(used), 10)
}

func (c *diskUsageVolumesContext) Size() string {
	var size int64

	for _, v := range c.volumes {
		if v.UsageData.Size != -1 {
			size += v.UsageData.Size
		}
	}

	return units.HumanSize(float64(size))
}

func (c *diskUsageVolumesContext) Reclaimable() string {
	var reclaimable int64
	var totalSize int64

	for _, v := range c.volumes {
		if v.UsageData.Size != -1 {
			if v.UsageData.RefCount == 0 {
				reclaimable += v.UsageData.Size
			}
			totalSize += v.UsageData.Size
		}
	}

	if totalSize > 0 {
		return fmt.Sprintf("%s (%v%%)", units.HumanSize(float64(reclaimable)), (reclaimable*100)/totalSize)
	}

	return units.HumanSize(float64(reclaimable))
}
