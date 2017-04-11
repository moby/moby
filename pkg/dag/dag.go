// Package dag provides dag
package dag

import (
	"sort"
)

// Node is a zero-indexed, consecutive integer that denotes a node
type Node int

// Edge is a edge
type Edge struct {
	Depender Node // from
	Dependee Node // to
}

// Graph is SUPPOSED to be a DAG, but there is no guarantee.
// MAY be disconnected graph.
type Graph struct {
	// Nodes needs to be sorted according to NodesSorter
	Nodes []Node
	// Edges needs to be sorted according to EdgesSorter
	Edges []Edge
}

func ComponentRoots(g *Graph) []Node {
	nonRoot := make(map[Node]struct{}, 0)
	for _, edge := range g.Edges {
		nonRoot[edge.Depender] = struct{}{}
	}
	var roots []Node
	for _, n := range g.Nodes {
		_, ok := nonRoot[n]
		if !ok {
			roots = append(roots, n)
		}
	}
	return roots
}

func (g *Graph) HasNode(n Node) bool {
	for _, x := range g.Nodes {
		if x == n {
			return true
		}
	}
	return false
}

type NodesSorter []Node

func (x NodesSorter) Len() int           { return len(x) }
func (x NodesSorter) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }
func (x NodesSorter) Less(i, j int) bool { return x[i] < x[j] }

func (g *Graph) AddNode(n Node) {
	if !g.HasNode(n) {
		g.Nodes = append(g.Nodes, n)
		sort.Sort(NodesSorter(g.Nodes))
	}
}

func (g *Graph) HasEdge(e Edge) bool {
	for _, x := range g.Edges {
		if x.Depender == e.Depender && x.Dependee == e.Dependee {
			return true
		}
	}
	return false
}

type EdgesSorter []Edge

func (x EdgesSorter) Len() int      { return len(x) }
func (x EdgesSorter) Swap(i, j int) { x[i], x[j] = x[j], x[i] }
func (x EdgesSorter) Less(i, j int) bool {
	return x[i].Depender < x[j].Depender ||
		(x[i].Depender == x[j].Depender && x[i].Dependee < x[j].Dependee)
}

func (g *Graph) AddEdge(e Edge) {
	if !g.HasEdge(e) {
		g.Edges = append(g.Edges, e)
		sort.Sort(EdgesSorter(g.Edges))
		g.AddNode(e.Depender)
		g.AddNode(e.Dependee)
	}
}

func Subgraph(g *Graph, root Node) *Graph {
	if !g.HasNode(root) {
		return nil
	}
	h := &Graph{}
	h.AddNode(root)
	for _, _ = range g.Edges {
		for _, edge := range g.Edges {
			for _, n := range h.Nodes {
				if edge.Dependee == n {
					h.AddEdge(edge)
				}
			}
		}
	}
	return h
}

// Dependers returns direct dependers, not indirect ones
func Dependers(g *Graph, dependee Node) []Node {
	var dependers []Node
	for _, e := range g.Edges {
		if e.Dependee == dependee {
			for _, n := range dependers {
				if e.Depender == n {
					continue
				}
			}
			dependers = append(dependers, e.Depender)
		}
	}
	sort.Sort(NodesSorter(dependers))
	return dependers
}

// Dependees returns direct dependees, not indirect ones
func Dependees(g *Graph, depender Node) []Node {
	var dependees []Node
	for _, e := range g.Edges {
		if e.Depender == depender {
			for _, n := range dependees {
				if e.Dependee == n {
					continue
				}
			}
			dependees = append(dependees, e.Dependee)
		}
	}
	sort.Sort(NodesSorter(dependees))
	return dependees
}
