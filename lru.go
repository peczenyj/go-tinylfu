package tinylfu

import "container/list"

// Cache is an LRU cache.  It is not safe for concurrent access.
type lruCache struct {
	data map[uint64]*list.Element
	cap  int
	ll   *list.List
}

func newLRU(cap int, data map[uint64]*list.Element) *lruCache {
	return &lruCache{
		data: data,
		cap:  cap,
		ll:   list.New(),
	}
}

// Get returns a value from the cache
func (lru *lruCache) get(v *list.Element) {
	lru.ll.MoveToFront(v)
}

// Set sets a value in the cache
func (lru *lruCache) add(newItem *Item) (_ *Item, evicted bool) {
	if lru.ll.Len() < lru.cap {
		lru.data[newItem.Key] = lru.ll.PushFront(&newItem)
		return &Item{}, false
	}

	// reuse the tail item
	e := lru.ll.Back()
	item := e.Value.(*Item)

	delete(lru.data, item.Key)

	oldItem := *item
	*item = *newItem

	lru.data[item.Key] = e
	lru.ll.MoveToFront(e)

	return &oldItem, true
}

// Len returns the total number of items in the cache
func (lru *lruCache) Len() int {
	return len(lru.data)
}

// Remove removes an item from the cache, returning the item and a boolean indicating if it was found
func (lru *lruCache) Remove(key uint64) (interface{}, bool) {
	v, ok := lru.data[key]
	if !ok {
		return nil, false
	}
	item := v.Value.(*Item)
	lru.ll.Remove(v)
	delete(lru.data, key)
	return item.Value, true
}
