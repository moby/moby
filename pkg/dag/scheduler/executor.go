package scheduler

import (
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/dag"
	"golang.org/x/sync/errgroup"
)

type locker struct {
	sync.Mutex
	done  map[dag.Node]struct{}
	onces map[dag.Node]*sync.Once
}

type executor struct {
	g    *dag.Graph
	cond *sync.Cond
	l    *locker
}

// TODO: context.WithTimeout
func (x *executor) waitUntilDependeesCompletion(n dag.Node) {
	dependees := dag.Dependees(x.g, n)
	completedDependees := make(map[dag.Node]struct{}, 0)
	for len(dependees) != len(completedDependees) {
		for _, dep := range dependees {
			ok := false
			for !ok {
				x.l.Lock()
				_, ok = x.l.done[dep]
				x.l.Unlock()
			}
			completedDependees[dep] = struct{}{}
		}
	}
}

func (x *executor) executeSchedule(sched *Schedule, exe func(dag.Node) error) error {
	x.waitUntilDependeesCompletion(sched.Node)

	var err error
	x.l.onces[sched.Node].Do(func() {
		err = exe(sched.Node)
		x.l.Lock()
		x.l.done[sched.Node] = struct{}{}
		x.l.Unlock()
	})
	if err != nil {
		return err
	}

	var (
		eg errgroup.Group
	)
	for _, c := range sched.Children {
		c := c
		eg.Go(func() error {
			return x.executeSchedule(c, exe)
		})
	}
	return eg.Wait()
}

func ExecuteSchedule(g *dag.Graph, root *ScheduleRoot, parallelism int, exe func(dag.Node) error) error {
	nodes := countNodes(root)
	if parallelism == 0 {
		parallelism = len(nodes)
	}
	logrus.Warnf("parallelism (currently ignored) : %d", parallelism)
	l := &locker{
		done:  make(map[dag.Node]struct{}, 0),
		onces: make(map[dag.Node]*sync.Once, 0),
	}
	for _, n := range nodes {
		l.onces[n] = new(sync.Once)
	}
	x := &executor{
		cond: sync.NewCond(l),
		g:    g,
		l:    l,
	}
	var (
		eg errgroup.Group
	)
	for _, c := range root.Children {
		c := c
		eg.Go(func() error {
			return x.executeSchedule(c, exe)
		})
	}
	return eg.Wait()
}

func _countNodes(m map[dag.Node]struct{}, s *Schedule) {
	m[s.Node] = struct{}{}
	for _, c := range s.Children {
		_countNodes(m, c)
	}
}

func countNodes(root *ScheduleRoot) []dag.Node {
	m := make(map[dag.Node]struct{}, 0)
	for _, c := range root.Children {
		_countNodes(m, c)
	}
	var res []dag.Node
	for n := range m {
		res = append(res, n)
	}
	return res
}
