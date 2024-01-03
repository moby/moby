package operatingsystem // import "github.com/docker/docker/pkg/parsers/operatingsystem"

import (
	"fmt"
	"strings"
)

type windowsOSRelease struct {
	IsServer       bool
	DisplayVersion string
	Build          uint32
	UBR            uint64
}

// String formats the OS release data similar to what is displayed by
// winver.exe.
func (r *windowsOSRelease) String() string {
	var b strings.Builder
	b.WriteString("Microsoft Windows")
	if r.IsServer {
		b.WriteString(" Server")
	}
	if r.DisplayVersion != "" {
		b.WriteString(" Version ")
		b.WriteString(r.DisplayVersion)
	}
	_, _ = fmt.Fprintf(&b, " (OS Build %d", r.Build)
	if r.UBR > 0 {
		_, _ = fmt.Fprintf(&b, ".%d", r.UBR)
	}
	b.WriteByte(')')
	return b.String()
}
