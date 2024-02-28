package resources

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	resourcestypes "github.com/moby/buildkit/executor/resources/types"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/network"
	"github.com/prometheus/procfs"
)

const (
	cgroupProcsFile       = "cgroup.procs"
	cgroupControllersFile = "cgroup.controllers"
	cgroupSubtreeFile     = "cgroup.subtree_control"
	defaultMountpoint     = "/sys/fs/cgroup"
	initGroup             = "init"
)

var initOnce sync.Once
var isCgroupV2 bool

type cgroupRecord struct {
	once         sync.Once
	ns           string
	sampler      *Sub[*resourcestypes.Sample]
	closeSampler func() error
	samples      []*resourcestypes.Sample
	err          error
	done         chan struct{}
	monitor      *Monitor
	netSampler   NetworkSampler
	startCPUStat *procfs.CPUStat
	sysCPUStat   *resourcestypes.SysCPUStat
}

func (r *cgroupRecord) Wait() error {
	go r.close()
	<-r.done
	return r.err
}

func (r *cgroupRecord) Start() {
	if stat, err := r.monitor.proc.Stat(); err == nil {
		r.startCPUStat = &stat.CPUTotal
	}
	s := NewSampler(2*time.Second, 10, r.sample)
	r.sampler = s.Record()
	r.closeSampler = s.Close
}

func (r *cgroupRecord) Close() {
	r.close()
}

func (r *cgroupRecord) CloseAsync(next func(context.Context) error) error {
	go func() {
		r.close()
		next(context.TODO())
	}()
	return nil
}

func (r *cgroupRecord) close() {
	r.once.Do(func() {
		defer close(r.done)
		go func() {
			r.monitor.mu.Lock()
			delete(r.monitor.records, r.ns)
			r.monitor.mu.Unlock()
		}()
		if r.sampler == nil {
			return
		}
		s, err := r.sampler.Close(true)
		if err != nil {
			r.err = err
		} else {
			r.samples = s
		}
		r.closeSampler()

		if r.startCPUStat != nil {
			stat, err := r.monitor.proc.Stat()
			if err == nil {
				cpu := &resourcestypes.SysCPUStat{
					User:      stat.CPUTotal.User - r.startCPUStat.User,
					Nice:      stat.CPUTotal.Nice - r.startCPUStat.Nice,
					System:    stat.CPUTotal.System - r.startCPUStat.System,
					Idle:      stat.CPUTotal.Idle - r.startCPUStat.Idle,
					Iowait:    stat.CPUTotal.Iowait - r.startCPUStat.Iowait,
					IRQ:       stat.CPUTotal.IRQ - r.startCPUStat.IRQ,
					SoftIRQ:   stat.CPUTotal.SoftIRQ - r.startCPUStat.SoftIRQ,
					Steal:     stat.CPUTotal.Steal - r.startCPUStat.Steal,
					Guest:     stat.CPUTotal.Guest - r.startCPUStat.Guest,
					GuestNice: stat.CPUTotal.GuestNice - r.startCPUStat.GuestNice,
				}
				r.sysCPUStat = cpu
			}
		}
	})
}

func (r *cgroupRecord) sample(tm time.Time) (*resourcestypes.Sample, error) {
	cpu, err := getCgroupCPUStat(filepath.Join(defaultMountpoint, r.ns))
	if err != nil {
		return nil, err
	}
	memory, err := getCgroupMemoryStat(filepath.Join(defaultMountpoint, r.ns))
	if err != nil {
		return nil, err
	}
	io, err := getCgroupIOStat(filepath.Join(defaultMountpoint, r.ns))
	if err != nil {
		return nil, err
	}
	pids, err := getCgroupPIDsStat(filepath.Join(defaultMountpoint, r.ns))
	if err != nil {
		return nil, err
	}
	sample := &resourcestypes.Sample{
		Timestamp_: tm,
		CPUStat:    cpu,
		MemoryStat: memory,
		IOStat:     io,
		PIDsStat:   pids,
	}
	if r.netSampler != nil {
		net, err := r.netSampler.Sample()
		if err != nil {
			return nil, err
		}
		sample.NetStat = net
	}
	return sample, nil
}

func (r *cgroupRecord) Samples() (*resourcestypes.Samples, error) {
	<-r.done
	if r.err != nil {
		return nil, r.err
	}
	return &resourcestypes.Samples{
		Samples:    r.samples,
		SysCPUStat: r.sysCPUStat,
	}, nil
}

type nopRecord struct {
}

func (r *nopRecord) Wait() error {
	return nil
}

func (r *nopRecord) Samples() (*resourcestypes.Samples, error) {
	return nil, nil
}

func (r *nopRecord) Close() {
}

func (r *nopRecord) CloseAsync(next func(context.Context) error) error {
	return next(context.TODO())
}

func (r *nopRecord) Start() {
}

type Monitor struct {
	mu      sync.Mutex
	closed  chan struct{}
	records map[string]*cgroupRecord
	proc    procfs.FS
}

type NetworkSampler interface {
	Sample() (*network.Sample, error)
}

type RecordOpt struct {
	NetworkSampler NetworkSampler
}

func (m *Monitor) RecordNamespace(ns string, opt RecordOpt) (resourcestypes.Recorder, error) {
	isClosed := false
	select {
	case <-m.closed:
		isClosed = true
	default:
	}
	if !isCgroupV2 || isClosed {
		return &nopRecord{}, nil
	}
	r := &cgroupRecord{
		ns:         ns,
		done:       make(chan struct{}),
		monitor:    m,
		netSampler: opt.NetworkSampler,
	}
	m.mu.Lock()
	m.records[ns] = r
	m.mu.Unlock()
	return r, nil
}

func (m *Monitor) Close() error {
	close(m.closed)
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, r := range m.records {
		r.close()
	}
	return nil
}

func NewMonitor() (*Monitor, error) {
	initOnce.Do(func() {
		isCgroupV2 = isCgroup2()
		if !isCgroupV2 {
			return
		}
		if err := prepareCgroupControllers(); err != nil {
			bklog.L.Warnf("failed to prepare cgroup controllers: %+v", err)
		}
	})

	fs, err := procfs.NewDefaultFS()
	if err != nil {
		return nil, err
	}

	return &Monitor{
		closed:  make(chan struct{}),
		records: make(map[string]*cgroupRecord),
		proc:    fs,
	}, nil
}

func prepareCgroupControllers() error {
	v, ok := os.LookupEnv("BUILDKIT_SETUP_CGROUPV2_ROOT")
	if !ok {
		return nil
	}
	if b, _ := strconv.ParseBool(v); !b {
		return nil
	}
	// move current process to init cgroup
	if err := os.MkdirAll(filepath.Join(defaultMountpoint, initGroup), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(defaultMountpoint, cgroupProcsFile), os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	s := bufio.NewScanner(f)
	for s.Scan() {
		if err := os.WriteFile(filepath.Join(defaultMountpoint, initGroup, cgroupProcsFile), s.Bytes(), 0); err != nil {
			return err
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	dt, err := os.ReadFile(filepath.Join(defaultMountpoint, cgroupControllersFile))
	if err != nil {
		return err
	}
	for _, c := range strings.Split(string(dt), " ") {
		if c == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(defaultMountpoint, cgroupSubtreeFile), []byte("+"+c), 0); err != nil {
			// ignore error
			bklog.L.Warnf("failed to enable cgroup controller %q: %+v", c, err)
		}
	}
	return nil
}
