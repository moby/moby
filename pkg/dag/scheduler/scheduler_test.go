package scheduler

import (
	"testing"

	"github.com/docker/docker/pkg/dag"
	"github.com/docker/docker/pkg/testutil/assert"
)

func TestDetermineSchedule1(t *testing.T) {
	g := &dag.Graph{
		Nodes: []dag.Node{0, 1, 2, 3, 4, 5},
		Edges: []dag.Edge{
			{Depender: 2, Dependee: 0},
			{Depender: 3, Dependee: 1},
			{Depender: 4, Dependee: 2},
			{Depender: 5, Dependee: 2},
		},
	}
	expected := &ScheduleRoot{
		Children: []*Schedule{
			{
				Node: dag.Node(0),
				Children: []*Schedule{
					{
						Node: dag.Node(2),
						Children: []*Schedule{
							{Node: dag.Node(4)},
							{Node: dag.Node(5)},
						},
					},
				},
			},
			{
				Node: dag.Node(1),
				Children: []*Schedule{
					{
						Node: dag.Node(3),
					},
				},
			},
		},
	}
	t.Logf("expected: %s", expected.String())
	actual := DetermineSchedule(g)
	t.Logf("actual: %s", actual.String())
	assert.DeepEqual(t, actual, expected)
}

func TestDetermineSchedule2(t *testing.T) {
	g := &dag.Graph{
		Nodes: []dag.Node{0, 1, 2},
		Edges: []dag.Edge{
			{Depender: 2, Dependee: 0},
		},
	}
	expected := &ScheduleRoot{
		Children: []*Schedule{
			{
				Node: dag.Node(0),
				Children: []*Schedule{
					{
						Node: dag.Node(2),
					},
				},
			},
			{
				Node: dag.Node(1),
			},
		},
	}
	t.Logf("expected: %s", expected.String())
	actual := DetermineSchedule(g)
	t.Logf("actual: %s", actual.String())
	assert.DeepEqual(t, actual, expected)
}

func TestDetermineSchedule3(t *testing.T) {
	g := &dag.Graph{
		Nodes: []dag.Node{0, 1, 2},
		Edges: []dag.Edge{
			{Depender: 2, Dependee: 0},
			{Depender: 2, Dependee: 1},
		},
	}
	expected := &ScheduleRoot{
		Children: []*Schedule{
			{
				Node: dag.Node(0),
				Children: []*Schedule{
					{
						Node: dag.Node(2),
					},
				},
			},
			{
				Node: dag.Node(1),
				Children: []*Schedule{
					{
						// duplicated; executor should skip duplicated node.
						Node: dag.Node(2),
					},
				},
			},
		},
	}
	t.Logf("expected: %s", expected.String())
	actual := DetermineSchedule(g)
	t.Logf("actual: %s", actual.String())
	assert.DeepEqual(t, actual, expected)
}
