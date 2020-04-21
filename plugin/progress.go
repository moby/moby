package plugin

import (
	"sync"
	"time"

	"github.com/containerd/containerd/remotes/docker"
)

func newPushJobs(tracker docker.StatusTracker) *pushJobs {
	return &pushJobs{
		names: make(map[string]string),
		t:     tracker,
	}
}

type pushJobs struct {
	t docker.StatusTracker

	mu   sync.Mutex
	jobs []string
	// maps job ref to a name
	names map[string]string
}

func (p *pushJobs) add(id, name string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.names[id]; ok {
		return
	}
	p.jobs = append(p.jobs, id)
	p.names[id] = name
}

func (p *pushJobs) status() []contentStatus {
	statuses := make([]contentStatus, 0, len(p.jobs))

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, j := range p.jobs {
		var s contentStatus
		s.Ref = p.names[j]

		status, err := p.t.GetStatus(j)
		if err != nil {
			s.Status = "Waiting"
		} else {
			s.Total = status.Total
			s.Offset = status.Offset
			s.StartedAt = status.StartedAt
			s.UpdatedAt = status.UpdatedAt
			if status.UploadUUID == "" {
				s.Status = "Upload complete"
			} else {
				s.Status = "Uploading"
			}
		}
		statuses = append(statuses, s)
	}

	return statuses
}

type contentStatus struct {
	Status    string
	Total     int64
	Offset    int64
	StartedAt time.Time
	UpdatedAt time.Time
	Ref       string
}
