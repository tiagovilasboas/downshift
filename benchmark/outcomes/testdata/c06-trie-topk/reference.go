package task

import (
	"sort"
)

type node struct {
	kids   map[rune]*node
	word   string
	weight int
	end    bool
}

type Trie struct{ root *node }

func NewTrie() *Trie { return &Trie{root: &node{kids: map[rune]*node{}}} }

func (t *Trie) Insert(word string, weight int) {
	n := t.root
	for _, r := range word {
		c, ok := n.kids[r]
		if !ok {
			c = &node{kids: map[rune]*node{}}
			n.kids[r] = c
		}
		n = c
	}
	n.end, n.word, n.weight = true, word, weight
}

func (t *Trie) Top(prefix string, k int) []string {
	n := t.root
	for _, r := range prefix {
		if n = n.kids[r]; n == nil {
			return nil
		}
	}
	var found []*node
	var walk func(*node)
	walk = func(x *node) {
		if x.end {
			found = append(found, x)
		}
		for _, c := range x.kids {
			walk(c)
		}
	}
	walk(n)
	sort.Slice(found, func(i, j int) bool {
		if found[i].weight != found[j].weight {
			return found[i].weight > found[j].weight
		}
		return found[i].word < found[j].word
	})
	var out []string
	for i := 0; i < len(found) && i < k; i++ {
		out = append(out, found[i].word)
	}
	return out
}
