package local

import (
	"os"
	"time"
)

func getStartTime(fi os.FileInfo) time.Time {
	return fi.ModTime()
}
