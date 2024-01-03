package controller

import (
	"context"
	"sync"
	"time"

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/progress"
	digest "github.com/opencontainers/go-digest"
)

type Controller struct {
	count   int64
	started *time.Time
	writer  progress.Writer
	mu      sync.Mutex
	id      string

	Digest        digest.Digest
	Name          string
	WriterFactory progress.WriterFactory
	ProgressGroup *pb.ProgressGroup
}

var _ progress.Controller = &Controller{}

func (c *Controller) Start(ctx context.Context) (context.Context, func(error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
	if c.count == 1 {
		if c.started == nil {
			now := time.Now()
			c.started = &now
			c.writer, _, _ = c.WriterFactory(ctx)
			c.id = identity.NewID()
		}

		if c.Digest != "" {
			c.writer.Write(c.id, client.Vertex{
				Digest:        c.Digest,
				Name:          c.Name,
				Started:       c.started,
				ProgressGroup: c.ProgressGroup,
			})
		}
	}
	return progress.WithProgress(ctx, c.writer), func(err error) {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.count--
		if c.count == 0 {
			now := time.Now()
			var errString string
			if err != nil {
				errString = err.Error()
			}
			if c.Digest != "" {
				c.writer.Write(c.id, client.Vertex{
					Digest:        c.Digest,
					Name:          c.Name,
					Started:       c.started,
					Completed:     &now,
					Error:         errString,
					ProgressGroup: c.ProgressGroup,
				})
			}
			c.writer.Close()
			c.started = nil
			c.id = ""
		}
	}
}

func (c *Controller) Status(id string, action string) func() {
	start := time.Now()
	if c.writer != nil {
		c.writer.Write(id, progress.Status{
			Action:  action,
			Started: &start,
		})
	}
	return func() {
		complete := time.Now()
		if c.writer != nil {
			c.writer.Write(id, progress.Status{
				Action:    action,
				Started:   &start,
				Completed: &complete,
			})
		}
	}
}
