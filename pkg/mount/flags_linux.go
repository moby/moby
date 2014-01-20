package mount

import (
	"strings"
	"syscall"
)

// Parse fstab type mount options into mount() flags
// and device specific data
func parseOptions(options string) (int, string) {
	var (
		flag int
		data []string
	)

	flags := map[string]struct {
		clear bool
		flag  int
	}{
		"defaults":      {false, 0},
		"ro":            {false, syscall.MS_RDONLY},
		"rw":            {true, syscall.MS_RDONLY},
		"suid":          {true, syscall.MS_NOSUID},
		"nosuid":        {false, syscall.MS_NOSUID},
		"dev":           {true, syscall.MS_NODEV},
		"nodev":         {false, syscall.MS_NODEV},
		"exec":          {true, syscall.MS_NOEXEC},
		"noexec":        {false, syscall.MS_NOEXEC},
		"sync":          {false, syscall.MS_SYNCHRONOUS},
		"async":         {true, syscall.MS_SYNCHRONOUS},
		"dirsync":       {false, syscall.MS_DIRSYNC},
		"remount":       {false, syscall.MS_REMOUNT},
		"mand":          {false, syscall.MS_MANDLOCK},
		"nomand":        {true, syscall.MS_MANDLOCK},
		"atime":         {true, syscall.MS_NOATIME},
		"noatime":       {false, syscall.MS_NOATIME},
		"diratime":      {true, syscall.MS_NODIRATIME},
		"nodiratime":    {false, syscall.MS_NODIRATIME},
		"bind":          {false, syscall.MS_BIND},
		"rbind":         {false, syscall.MS_BIND | syscall.MS_REC},
		"relatime":      {false, syscall.MS_RELATIME},
		"norelatime":    {true, syscall.MS_RELATIME},
		"strictatime":   {false, syscall.MS_STRICTATIME},
		"nostrictatime": {true, syscall.MS_STRICTATIME},
	}

	for _, o := range strings.Split(options, ",") {
		// If the option does not exist in the flags table then it is a
		// data value for a specific fs type
		if f, exists := flags[o]; exists {
			if f.clear {
				flag &= ^f.flag
			} else {
				flag |= f.flag
			}
		} else {
			data = append(data, o)
		}
	}
	return flag, strings.Join(data, ",")
}
