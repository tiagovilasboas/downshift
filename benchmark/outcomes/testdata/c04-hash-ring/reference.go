package task

import (
	"hash/crc32"
	"sort"
	"strconv"
)

type Ring struct {
	replicas int
	points   []uint32
	owner    map[uint32]string
}

func NewRing(replicas int) *Ring { return &Ring{replicas: replicas, owner: map[uint32]string{}} }

func (r *Ring) Add(node string) {
	for i := 0; i < r.replicas; i++ {
		h := crc32.ChecksumIEEE([]byte(strconv.Itoa(i) + node))
		if _, taken := r.owner[h]; !taken {
			r.owner[h] = node
			r.points = append(r.points, h)
		}
	}
	sort.Slice(r.points, func(i, j int) bool { return r.points[i] < r.points[j] })
}

func (r *Ring) Remove(node string) {
	kept := r.points[:0]
	for _, p := range r.points {
		if r.owner[p] == node {
			delete(r.owner, p)
			continue
		}
		kept = append(kept, p)
	}
	r.points = kept
}

func (r *Ring) Get(key string) string {
	if len(r.points) == 0 {
		return ""
	}
	h := crc32.ChecksumIEEE([]byte(key))
	i := sort.Search(len(r.points), func(i int) bool { return r.points[i] >= h })
	if i == len(r.points) {
		i = 0
	}
	return r.owner[r.points[i]]
}
