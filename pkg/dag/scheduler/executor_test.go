package scheduler

import (
	"math/rand"
	"testing"
	"time"

	"github.com/docker/docker/pkg/dag"
	"github.com/docker/docker/pkg/testutil/assert"
)

func TestExecuteSchedule(t *testing.T) {
	testExecuteSchedule(t, 0)
}

func testExecuteSchedule(t *testing.T, parallelism int) []dag.Node {
	g := &dag.Graph{
		Nodes: []dag.Node{0, 1, 2, 3, 4, 5},
		Edges: []dag.Edge{
			{Depender: 2, Dependee: 0},
			{Depender: 3, Dependee: 1},
			{Depender: 4, Dependee: 2},
			{Depender: 5, Dependee: 2},
			{Depender: 5, Dependee: 3},
		},
	}
	schedRoot := DetermineSchedule(g)
	// c := make(chan dag.Node, 6)
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	err := ExecuteSchedule(g, schedRoot, parallelism, func(n dag.Node) error {
		t.Logf("executing node %d", n)
		time.Sleep(time.Duration(rnd.Int63n(int64(100 * time.Millisecond))))
		// c <- n
		return nil
	})
	assert.NilError(t, err)
	var got []dag.Node
	// close(c)
	// for n := range c {
	// 	got = append(got, n)
	// }
	// assert.Equal(t, len(got), 6)
	// assert.Equal(t, indexOf(got, 0) < indexOf(got, 2), true)
	// assert.Equal(t, indexOf(got, 2) < indexOf(got, 4), true)
	// assert.Equal(t, indexOf(got, 2) < indexOf(got, 5) || indexOf(got, 3) < indexOf(got, 5), true)
	// assert.Equal(t, indexOf(got, 1) < indexOf(got, 3), true)
	return got
}

func indexOf(nodes []dag.Node, node dag.Node) int {
	for i, n := range nodes {
		if n == node {
			return i
		}
	}
	panic("node not found")
}
