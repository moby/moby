package local

import (
	"os"
	"time"
)

func getATime(fi os.FileInfo) time.Time {
	return fi.ModTime()
}
