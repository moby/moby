package scheduler

import (
	"container/heap"
	"errors"
	"time"

	"github.com/docker/swarmkit/api"
)

var errNodeNotFound = errors.New("node not found in scheduler dataset")

type nodeSet struct {
	nodes map[string]NodeInfo // map from node id to node info
}

func (ns *nodeSet) alloc(n int) {
	ns.nodes = make(map[string]NodeInfo, n)
}

// nodeInfo returns the NodeInfo struct for a given node identified by its ID.
func (ns *nodeSet) nodeInfo(nodeID string) (NodeInfo, error) {
	node, ok := ns.nodes[nodeID]
	if ok {
		return node, nil
	}
	return NodeInfo{}, errNodeNotFound
}

// addOrUpdateNode sets the number of tasks for a given node. It adds the node
// to the set if it wasn't already tracked.
func (ns *nodeSet) addOrUpdateNode(n NodeInfo) {
	if n.Tasks == nil {
		n.Tasks = make(map[string]*api.Task)
	}
	if n.DesiredRunningTasksCountByService == nil {
		n.DesiredRunningTasksCountByService = make(map[string]int)
	}
	if n.recentFailures == nil {
		n.recentFailures = make(map[string][]time.Time)
	}

	ns.nodes[n.ID] = n
}

// updateNode sets the number of tasks for a given node. It ignores the update
// if the node isn't already tracked in the set.
func (ns *nodeSet) updateNode(n NodeInfo) {
	_, ok := ns.nodes[n.ID]
	if ok {
		ns.nodes[n.ID] = n
	}
}

func (ns *nodeSet) remove(nodeID string) {
	delete(ns.nodes, nodeID)
}

type nodeMaxHeap struct {
	nodes    []NodeInfo
	lessFunc func(*NodeInfo, *NodeInfo) bool
	length   int
}

func (h nodeMaxHeap) Len() int {
	return h.length
}

func (h nodeMaxHeap) Swap(i, j int) {
	h.nodes[i], h.nodes[j] = h.nodes[j], h.nodes[i]
}

func (h nodeMaxHeap) Less(i, j int) bool {
	// reversed to make a max-heap
	return h.lessFunc(&h.nodes[j], &h.nodes[i])
}

func (h *nodeMaxHeap) Push(x interface{}) {
	h.nodes = append(h.nodes, x.(NodeInfo))
	h.length++
}

func (h *nodeMaxHeap) Pop() interface{} {
	h.length--
	// return value is never used
	return nil
}

// findBestNodes returns n nodes (or < n if fewer nodes are available) that
// rank best (lowest) according to the sorting function.
func (ns *nodeSet) findBestNodes(n int, meetsConstraints func(*NodeInfo) bool, nodeLess func(*NodeInfo, *NodeInfo) bool) []NodeInfo {
	if n == 0 {
		return []NodeInfo{}
	}

	nodeHeap := nodeMaxHeap{lessFunc: nodeLess}

	// TODO(aaronl): Is is possible to avoid checking constraints on every
	// node? Perhaps we should try to schedule with n*2 nodes that weren't
	// prescreened, and repeat the selection if there weren't enough nodes
	// meeting the constraints.
	for _, node := range ns.nodes {
		// If there are fewer then n nodes in the heap, we add this
		// node if it meets the constraints. Otherwise, the heap has
		// n nodes, and if this node is better than the worst node in
		// the heap, we replace the worst node and then fix the heap.
		if nodeHeap.Len() < n {
			if meetsConstraints(&node) {
				heap.Push(&nodeHeap, node)
			}
		} else if nodeLess(&node, &nodeHeap.nodes[0]) {
			if meetsConstraints(&node) {
				nodeHeap.nodes[0] = node
				heap.Fix(&nodeHeap, 0)
			}
		}
	}

	// Popping every element orders the nodes from best to worst. The
	// first pop gets the worst node (since this a max-heap), and puts it
	// at position n-1. Then the next pop puts the next-worst at n-2, and
	// so on.
	for nodeHeap.Len() > 0 {
		heap.Pop(&nodeHeap)
	}

	return nodeHeap.nodes
}
