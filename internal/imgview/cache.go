package imgview

import (
	"container/list"
	"sync"
)

// Cache is a concurrency-safe, byte-budgeted LRU. Keys must include file
// identity, transformation and output settings. Values must remain immutable.
type Cache struct {
	mu           sync.Mutex
	budget, used int64
	entries      map[string]*list.Element
	order        list.List
}
type cacheEntry struct {
	key   string
	value Decoded
	bytes int64
}

func NewCache(budget int64) *Cache {
	if budget < 0 {
		budget = 0
	}
	return &Cache{budget: budget, entries: make(map[string]*list.Element)}
}
func (c *Cache) Get(key string) (Decoded, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.entries[key]; e != nil {
		c.order.MoveToFront(e)
		return e.Value.(cacheEntry).value, true
	}
	return Decoded{}, false
}
func (c *Cache) remove(e *list.Element) {
	v := e.Value.(cacheEntry)
	delete(c.entries, v.key)
	c.used -= v.bytes
	c.order.Remove(e)
}
func (c *Cache) Put(key string, v Decoded) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.entries[key]; e != nil {
		c.remove(e)
	}
	if v.Pixels == nil {
		return false
	}
	n := int64(len(v.Pixels.Pix))
	if n > c.budget {
		return false
	}
	for c.used > c.budget-n {
		c.remove(c.order.Back())
	}
	c.entries[key] = c.order.PushFront(cacheEntry{key, v, n})
	c.used += n
	return true
}
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.entries[key]; e != nil {
		c.remove(e)
	}
}
func (c *Cache) Bytes() int64 { c.mu.Lock(); defer c.mu.Unlock(); return c.used }
