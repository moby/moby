package archive // import "github.com/docker/docker/pkg/archive"

import "time"

const maxSeconds = (1<<63 - 1) / int64(1e9)

func sanitizeTime(t time.Time) time.Time {
	if ut := t.Unix(); ut < 0 || ut > maxSeconds {
		return time.Unix(0, 0)
	}
	return t
}
