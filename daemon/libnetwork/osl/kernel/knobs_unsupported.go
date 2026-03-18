//go:build !linux

package kernel

// ApplyOSTweaks applies the configuration values passed as arguments
func ApplyOSTweaks(osConfig map[string]*OSValue) {
}
