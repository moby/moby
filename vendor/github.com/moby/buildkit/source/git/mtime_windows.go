//go:build windows

package git

import "time"

func lchtimes(_ string, _ time.Time) error {
	return nil
}
