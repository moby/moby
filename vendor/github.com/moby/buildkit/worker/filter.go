package worker

import (
	"strings"

	"github.com/containerd/containerd/filters"
)

func adaptWorker(w Worker) filters.Adaptor {
	return filters.AdapterFunc(func(fieldpath []string) (string, bool) {
		if len(fieldpath) == 0 {
			return "", false
		}

		switch fieldpath[0] {
		case "id":
			return w.ID(), len(w.ID()) > 0
		case "labels":
			return checkMap(fieldpath[1:], w.Labels())
		}

		return "", false
	})
}

func checkMap(fieldpath []string, m map[string]string) (string, bool) {
	if len(m) == 0 {
		return "", false
	}

	value, ok := m[strings.Join(fieldpath, ".")]
	return value, ok
}
