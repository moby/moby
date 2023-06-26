//go:build !windows

package homedir // import "github.com/docker/docker/pkg/homedir"

const (
	envKeyName   = "HOME"
	homeShortCut = "~"
)
