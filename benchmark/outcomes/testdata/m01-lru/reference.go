package task

import (
	"container/list"
)

type entry struct {
	key   string
	value int
}

type LRU struct {
	cap   int
	order *list.List
	items map[string]*list.Element
}

func NewLRU(capacity int) *LRU {
	return &LRU{cap: capacity, order: list.New(), items: map[string]*list.Element{}}
}

func (c *LRU) Get(key string) (int, bool) {
	el, ok := c.items[key]
	if !ok {
		return 0, false
	}
	c.order.MoveToFront(el)
	return el.Value.(*entry).value, true
}

func (c *LRU) Put(key string, value int) {
	if el, ok := c.items[key]; ok {
		el.Value.(*entry).value = value
		c.order.MoveToFront(el)
		return
	}
	if c.cap <= 0 {
		return
	}
	if c.order.Len() >= c.cap {
		old := c.order.Back()
		c.order.Remove(old)
		delete(c.items, old.Value.(*entry).key)
	}
	c.items[key] = c.order.PushFront(&entry{key, value})
}
