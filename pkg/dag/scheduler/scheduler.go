package scheduler

import (
	"fmt"

	"github.com/docker/docker/pkg/dag"
)

// Schedule denotes a schedule
type Schedule struct {
	Node dag.Node
	// Children are executed in parallel after executing Node.
	// Sequential schedule can be expressed as a chain of single-children schedules.
	Children []*Schedule
}

func (sched *Schedule) String() string {
	if len(sched.Children) > 0 {
		s := fmt.Sprintf("seq{%d;par{", sched.Node)
		for _, c := range sched.Children {
			s += c.String()
		}
		s += "}}"
		return s
	} else {
		return fmt.Sprintf("%d;", sched.Node)
	}
}

type ScheduleRoot struct {
	Children []*Schedule
}

func (sched *ScheduleRoot) String() string {
	s := "par{"
	for _, c := range sched.Children {
		s += c.String()
	}
	s += "}"
	return s
}

func DetermineSchedule(g *dag.Graph) *ScheduleRoot {
	schedRoot := &ScheduleRoot{}
	for _, compoRoot := range dag.ComponentRoots(g) {
		subg := dag.Subgraph(g, compoRoot)
		if subg != nil {
			schedRoot.Children = append(schedRoot.Children, determineSchedule(subg, compoRoot))
		}
	}
	return schedRoot
}

func determineSchedule(subg *dag.Graph, subgRoot dag.Node) *Schedule {
	s := &Schedule{
		Node: subgRoot,
	}
	for _, depender := range dag.Dependers(subg, subgRoot) {
		subsubg := dag.Subgraph(subg, depender)
		child := determineSchedule(subsubg, depender)
		if child != nil {
			s.Children = append(s.Children, child)
		}
	}
	return s
}
