package dag

import (
	"testing"

	"github.com/docker/docker/pkg/testutil/assert"
)

func TestComponentRoots(t *testing.T) {
	assert.DeepEqual(t, ComponentRoots(
		&Graph{
			Nodes: []Node{0, 1, 2, 3},
			Edges: []Edge{
				{Depender: 2, Dependee: 0},
				{Depender: 3, Dependee: 1},
			},
		}), []Node{0, 1})
}

func TestSubgraph(t *testing.T) {
	g := &Graph{
		Nodes: []Node{0, 1, 2, 3, 4, 5},
		Edges: []Edge{
			{Depender: 2, Dependee: 0},
			{Depender: 3, Dependee: 1},
			{Depender: 4, Dependee: 2},
			{Depender: 5, Dependee: 2},
		},
	}
	assert.DeepEqual(t, Subgraph(g, 2),
		&Graph{
			Nodes: []Node{2, 4, 5},
			Edges: []Edge{
				{Depender: 4, Dependee: 2},
				{Depender: 5, Dependee: 2},
			},
		})
}

func TestDependers(t *testing.T) {
	assert.DeepEqual(t, Dependers(
		&Graph{
			Nodes: []Node{0, 1, 2, 3, 4, 5, 6},
			Edges: []Edge{
				{Depender: 2, Dependee: 0},
				{Depender: 3, Dependee: 1},
				{Depender: 4, Dependee: 2},
				{Depender: 5, Dependee: 2},
				{Depender: 6, Dependee: 5},
			},
		}, 2), []Node{4, 5})
}
