package graph

import (
	"github.com/twmb/algoimpl/go/graph/lite"
	"math/rand"
	"sync"
)

const (
	dequeued = ^(1<<31 - 1)
	unseen   = 0
	seen     = 1
)

// O(V + E). It does not matter to traverse back
// on a bidirectional edge, because any vertex dfs is
// recursing on is marked as visited and won't be visited
// again anyway.
func (g *Graph) dfs(node *node, finishList *[]Node) {
	node.state = seen
	for _, edge := range node.edges {
		if edge.end.state == unseen {
			edge.end.parent = node
			g.dfs(edge.end, finishList)
		}
	}
	*finishList = append(*finishList, node.container)
}

func (g *Graph) dfsReversedEdges(node *node, finishList *[]Node) {
	node.state = seen
	for _, edge := range node.reversedEdges {
		if edge.end.state == unseen {
			edge.end.parent = node
			g.dfsReversedEdges(edge.end, finishList)
		}
	}
	*finishList = append(*finishList, node.container)
}

// TopologicalSort topoligically sorts a directed acyclic graph.
// If the graph is cyclic, the sort order will change
// based on which node the sort starts on.
//
// The StronglyConnectedComponents function can be used to determine if a graph has cycles.
func (g *Graph) TopologicalSort() []Node {
	sorted := make([]Node, 0, len(g.nodes))
	// sort preorder (first jacket, then shirt)
	for _, node := range g.nodes {
		if node.state == unseen {
			g.dfs(node, &sorted)
		}
	}
	// now make post order for correct sort (jacket follows shirt). O(V)
	length := len(sorted)
	for i := 0; i < length/2; i++ {
		sorted[i], sorted[length-i-1] = sorted[length-i-1], sorted[i]
	}
	return sorted
}

// Reverse returns reversed copy of the directed graph g.
// This function can be used to copy an undirected graph.
func (g *Graph) Reverse() *Graph {
	reversed := New(Directed)
	if g.Kind == Undirected {
		reversed = New(Undirected)
	}
	// O(V)
	for _ = range g.nodes {
		reversed.MakeNode()
	}
	// O(V + E)
	for _, node := range g.nodes {
		for _, edge := range node.edges {
			reversed.MakeEdge(reversed.nodes[edge.end.index].container,
				reversed.nodes[node.index].container)
		}
	}
	return reversed
}

// StronglyConnectedComponents returns a slice of strongly connected nodes in a directed graph.
// If used on an undirected graph, this function returns nil.
func (g *Graph) StronglyConnectedComponents() [][]Node {
	if g.Kind == Undirected {
		return nil
	}
	components := make([][]Node, 0)
	finishOrder := g.TopologicalSort()
	for i := range finishOrder {
		finishOrder[i].node.state = unseen
	}
	for _, sink := range finishOrder {
		if g.nodes[sink.node.index].state == unseen {
			component := make([]Node, 0)
			g.dfsReversedEdges(g.nodes[sink.node.index], &component)
			components = append(components, component)
		}
	}
	return components
}

// RandMinimumCut runs Kargers algorithm to find a random minimum cut
// on the graph. If iterations is < 1, this will return an empty slice.
// Otherwise, it returns a slice of the edges crossing the best minimum
// cut found in any iteration. Call rand.Seed() before using this function.
//
// This function takes a number of iterations to start concurrently. If
// concurrent is <= 1, it will run one iteration at a time.
//
// If the graph is Directed, this will return a cut of edges in both directions.
// If the graph is Undirected, this will return a proper min cut.
func (g *Graph) RandMinimumCut(iterations, concurrent int) []Edge {
	var mutex sync.Mutex
	doneChan := make(chan struct{}, iterations)
	if concurrent < 1 {
		concurrent = 1
	}
	sem := make(chan struct{}, concurrent)
	for i := 0; i < concurrent; i++ {
		sem <- struct{}{}
	}
	// make a lite slice of the edges
	var baseAllEdges []lite.Edge
	for n := range g.nodes {
		for _, edge := range g.nodes[n].edges {
			if g.Kind == Undirected && n < edge.end.index {
				continue
			}
			baseAllEdges = append(baseAllEdges, lite.Edge{Start: n,
				End: edge.end.index, S: g.nodes[n], E: edge})
		}
	}

	minCutLite := make([]lite.Edge, len(baseAllEdges))

	for iter := 0; iter < iterations; iter++ {
		<-sem
		go func() {
			nodecount := len(g.nodes)
			allEdges := make([]lite.Edge, len(baseAllEdges))
			copy(allEdges, baseAllEdges)
			// shuffle for random edge removal order
			shuffle(allEdges)
			for nodecount > 2 {
				// remove first edge, keep the start node, collapse the end node
				// anything that points to the collapsing node now points to the keep node
				// anything that starts at the collapsing node now starts at the keep node
				keep := allEdges[len(allEdges)-1].Start
				remove := allEdges[len(allEdges)-1].End
				allEdges = allEdges[:len(allEdges)-1]
				for e := 0; e < len(allEdges); e++ {
					if allEdges[e].Start == remove {
						allEdges[e].Start = keep
					}
					if allEdges[e].End == remove {
						allEdges[e].End = keep
					}
					// remove the edge if it self looped
					if allEdges[e].Start == allEdges[e].End {
						allEdges[e] = allEdges[len(allEdges)-1]
						allEdges = allEdges[:len(allEdges)-1]
						e--
					}
				}
				// every edge removed removes a node
				nodecount--
			}

			mutex.Lock()
			if iter == 0 || len(allEdges) < len(minCutLite) {
				minCutLite = make([]lite.Edge, len(allEdges))
				copy(minCutLite, allEdges)
			}
			mutex.Unlock()

			doneChan <- struct{}{}
			sem <- struct{}{}
		}()
	}
	for iter := 0; iter < iterations; iter++ {
		<-doneChan
	}

	minCut := make([]Edge, len(minCutLite))
	for i := range minCutLite {
		start := minCutLite[i].S.(*node)
		edge := minCutLite[i].E.(edge)
		minCut[i] = Edge{Weight: edge.weight, Start: start.container,
			End: edge.end.container}
	}
	return minCut
}

// Fischer-Yates shuffle
func shuffle(edges []lite.Edge) {
	for i := len(edges) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		edges[j], edges[i] = edges[i], edges[j]
	}
}

// MinimumSpanningTree will return the edges corresponding to the
// minimum spanning tree in the graph based off of edge weight values.
// This will return nil for a directed graph.
func (g *Graph) MinimumSpanningTree() []Edge {
	if g.Kind == Directed {
		return nil
	}
	// create priority queue for vertices
	// node.data is the index into the heap
	// node.state is the weight of an edge to its parent
	nodesBase := nodeSlice(make([]*node, len(g.nodes)))
	copy(nodesBase, g.nodes)
	for i := range nodesBase {
		nodesBase[i].state = 1<<31 - 1
		nodesBase[i].data = i
	}
	nodesBase[0].state = 0
	nodesBase[0].parent = nil
	nodes := &nodesBase
	nodes.heapInit()

	for len(*nodes) > 0 {
		min := nodes.pop()
		for _, edge := range min.edges {
			v := edge.end // get the other side of the edge
			if nodes.heapContains(v) && edge.weight < v.state {
				v.parent = min
				nodes.update(v.data, edge.weight)
			}
		}
	}

	mst := make([]Edge, 0, len(g.nodes) - 1)
	for i := range g.nodes {
		if g.nodes[i].parent != nil {
			mst = append(mst, Edge{Weight: g.nodes[i].state,
				Start: g.nodes[i].container, End: g.nodes[i].parent.container})
		}
	}

	return mst
}
