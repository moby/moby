//go:build linux

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package v1

import (
	"sync"
	"sync/atomic"

	"golang.org/x/sys/unix"

	cgroups "github.com/containerd/cgroups/v3/cgroup1"
	"github.com/containerd/containerd/v2/pkg/sys"
	"github.com/containerd/log"
	metrics "github.com/docker/go-metrics"
	"github.com/prometheus/client_golang/prometheus"
)

func newOOMCollector(ns *metrics.Namespace) (*oomCollector, error) {
	fd, err := unix.EpollCreate1(unix.EPOLL_CLOEXEC)
	if err != nil {
		return nil, err
	}
	var desc *prometheus.Desc
	if ns != nil {
		desc = ns.NewDesc("memory_oom", "The number of times a container has received an oom event", metrics.Total, "container_id", "namespace")
	}
	c := &oomCollector{
		fd:   fd,
		desc: desc,
		set:  make(map[uintptr]*oom),
	}
	if ns != nil {
		ns.Add(c)
	}
	go c.start()
	return c, nil
}

type oomCollector struct {
	mu sync.Mutex

	desc *prometheus.Desc
	fd   int
	set  map[uintptr]*oom
}

type oom struct {
	// count needs to stay the first member of this struct to ensure 64bits
	// alignment on a 32bits machine (e.g. arm32). This is necessary as we use
	// the sync/atomic operations on this field.
	count     atomic.Int64
	id        string
	namespace string
	c         cgroups.Cgroup
	triggers  []Trigger
}

func (o *oomCollector) Add(id, namespace string, cg cgroups.Cgroup, triggers ...Trigger) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	fd, err := cg.OOMEventFD()
	if err != nil {
		return err
	}
	o.set[fd] = &oom{
		id:        id,
		c:         cg,
		triggers:  triggers,
		namespace: namespace,
	}
	event := unix.EpollEvent{
		Fd:     int32(fd),
		Events: unix.EPOLLHUP | unix.EPOLLIN | unix.EPOLLERR,
	}
	return unix.EpollCtl(o.fd, unix.EPOLL_CTL_ADD, int(fd), &event)
}

func (o *oomCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- o.desc
}

func (o *oomCollector) Collect(ch chan<- prometheus.Metric) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, t := range o.set {
		c := t.count.Load()
		ch <- prometheus.MustNewConstMetric(o.desc, prometheus.CounterValue, float64(c), t.id, t.namespace)
	}
}

// Close closes the epoll fd
func (o *oomCollector) Close() error {
	return unix.Close(o.fd)
}

func (o *oomCollector) start() {
	var (
		n      int
		err    error
		events [128]unix.EpollEvent
	)
	for {
		if err := sys.IgnoringEINTR(func() error {
			n, err = unix.EpollWait(o.fd, events[:], -1)
			return err
		}); err != nil {
			log.L.WithError(err).Error("cgroups: epoll wait failed, OOM notifications disabled")
			return
		}

		for i := 0; i < n; i++ {
			o.process(uintptr(events[i].Fd))
		}
	}
}

func (o *oomCollector) process(fd uintptr) {
	// make sure to always flush the eventfd
	flushEventfd(fd)

	o.mu.Lock()
	info, ok := o.set[fd]
	if !ok {
		o.mu.Unlock()
		return
	}
	o.mu.Unlock()
	// if we received an event but it was caused by the cgroup being deleted and the fd
	// being closed make sure we close our copy and remove the container from the set
	if info.c.State() == cgroups.Deleted {
		o.mu.Lock()
		delete(o.set, fd)
		o.mu.Unlock()
		unix.Close(int(fd))
		return
	}
	info.count.Add(1)
	for _, t := range info.triggers {
		t(info.id, info.namespace, info.c)
	}
}

func flushEventfd(efd uintptr) error {
	// Buffer must be >= 8 bytes for eventfd reads
	// https://man7.org/linux/man-pages/man2/eventfd.2.html
	var buf [8]byte
	_, err := unix.Read(int(efd), buf[:])
	return err
}
