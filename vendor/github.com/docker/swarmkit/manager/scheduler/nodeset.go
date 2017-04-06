package scheduler

import (
	"container/heap"
	"errors"
	"strings"
	"time"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/manager/constraint"
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
	if n.ActiveTasksCountByService == nil {
		n.ActiveTasksCountByService = make(map[string]int)
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

func (ns *nodeSet) tree(serviceID string, preferences []*api.PlacementPreference, maxAssignments int, meetsConstraints func(*NodeInfo) bool, nodeLess func(*NodeInfo, *NodeInfo) bool) decisionTree {
	var root decisionTree

	if maxAssignments == 0 {
		return root
	}

	for _, node := range ns.nodes {
		tree := &root
		for _, pref := range preferences {
			// Only spread is supported so far
			spread := pref.GetSpread()
			if spread == nil {
				continue
			}

			descriptor := spread.SpreadDescriptor
			var value string
			switch {
			case len(descriptor) > len(constraint.NodeLabelPrefix) && strings.EqualFold(descriptor[:len(constraint.NodeLabelPrefix)], constraint.NodeLabelPrefix):
				if node.Spec.Annotations.Labels != nil {
					value = node.Spec.Annotations.Labels[descriptor[len(constraint.NodeLabelPrefix):]]
				}
			case len(descriptor) > len(constraint.EngineLabelPrefix) && strings.EqualFold(descriptor[:len(constraint.EngineLabelPrefix)], constraint.EngineLabelPrefix):
				if node.Description != nil && node.Description.Engine != nil && node.Description.Engine.Labels != nil {
					value = node.Description.Engine.Labels[descriptor[len(constraint.EngineLabelPrefix):]]
				}
			// TODO(aaronl): Support other items from constraint
			// syntax like node ID, hostname, os/arch, etc?
			default:
				continue
			}

			// If value is still uninitialized, the value used for
			// the node at this level of the tree is "". This makes
			// sure that the tree structure is not affected by
			// which properties nodes have and don't have.

			if node.ActiveTasksCountByService != nil {
				tree.tasks += node.ActiveTasksCountByService[serviceID]
			}

			if tree.next == nil {
				tree.next = make(map[string]*decisionTree)
			}
			next := tree.next[value]
			if next == nil {
				next = &decisionTree{}
				tree.next[value] = next
			}
			tree = next
		}

		if node.ActiveTasksCountByService != nil {
			tree.tasks += node.ActiveTasksCountByService[serviceID]
		}

		if tree.nodeHeap.lessFunc == nil {
			tree.nodeHeap.lessFunc = nodeLess
		}

		if tree.nodeHeap.Len() < maxAssignments {
			if meetsConstraints(&node) {
				heap.Push(&tree.nodeHeap, node)
			}
		} else if nodeLess(&node, &tree.nodeHeap.nodes[0]) {
			if meetsConstraints(&node) {
				tree.nodeHeap.nodes[0] = node
				heap.Fix(&tree.nodeHeap, 0)
			}
		}
	}

	return root
}
