package task

import (
	"container/heap"
	"math"
)

type item struct{ node, dist int }
type pq []item

func (p pq) Len() int            { return len(p) }
func (p pq) Less(i, j int) bool  { return p[i].dist < p[j].dist }
func (p pq) Swap(i, j int)       { p[i], p[j] = p[j], p[i] }
func (p *pq) Push(x interface{}) { *p = append(*p, x.(item)) }
func (p *pq) Pop() interface{} {
	old := *p
	it := old[len(old)-1]
	*p = old[:len(old)-1]
	return it
}

func ShortestPath(n int, edges [][3]int, src, dst int) (int, []int, bool) {
	adj := make([][][2]int, n)
	for _, e := range edges {
		adj[e[0]] = append(adj[e[0]], [2]int{e[1], e[2]})
	}
	const inf = math.MaxInt
	dist := make([]int, n)
	prev := make([]int, n)
	for i := range dist {
		dist[i], prev[i] = inf, -1
	}
	dist[src] = 0
	h := &pq{{src, 0}}
	for h.Len() > 0 {
		it := heap.Pop(h).(item)
		if it.dist > dist[it.node] {
			continue
		}
		for _, e := range adj[it.node] {
			if nd := it.dist + e[1]; nd < dist[e[0]] {
				dist[e[0]], prev[e[0]] = nd, it.node
				heap.Push(h, item{e[0], nd})
			}
		}
	}
	if dist[dst] == inf {
		return 0, nil, false
	}
	var path []int
	for v := dst; v != -1; v = prev[v] {
		path = append([]int{v}, path...)
	}
	return dist[dst], path, true
}
