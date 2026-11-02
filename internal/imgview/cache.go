package imgview

import (
	"container/list"
	"sync"
)

// Cache is a concurrency-safe, byte-budgeted LRU. Keys must include file
// identity, transformation and output settings. Values must remain immutable.
type Cache struct {
	mu                     sync.Mutex
	budget, used, reserved int64
	entries                map[string]*list.Element
	order                  list.List
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
	if n > c.budget-c.reserved {
		return false
	}
	for c.used > c.budget-c.reserved-n {
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

// reserve temporarily shares the raster budget with animation decode work.
// Readers may retain already-published immutable rasters; the cache itself
// releases its references before the worker allocates animation canvases.
func (c *Cache) reserve(n int64) (func(), error) {
	c.mu.Lock()
	if n < 0 || n > c.budget-c.reserved {
		c.mu.Unlock()
		return nil, ErrLimit
	}
	c.reserved += n
	for c.used > c.budget-c.reserved {
		c.remove(c.order.Back())
	}
	c.mu.Unlock()
	var once sync.Once
	return func() { once.Do(func() { c.mu.Lock(); c.reserved -= n; c.mu.Unlock() }) }, nil
}
