// Package tinylfu is an implementation of the TinyLFU caching algorithm
/*
   http://arxiv.org/abs/1512.00727
*/
package tinylfu

import (
	"container/list"
	"sync"
	"time"
)

type T struct {
	w       int
	samples int

	countSketch *cm4
	bouncer     *doorkeeper

	data map[uint64]*list.Element

	lru  *lruCache
	slru *slruCache
}

func New(size int, samples int) *T {

	const lruPct = 1

	lruSize := (lruPct * size) / 100
	if lruSize < 1 {
		lruSize = 1
	}
	slruSize := int(float64(size) * ((100.0 - lruPct) / 100.0))
	if slruSize < 1 {
		slruSize = 1

	}
	slru20 := int(0.2 * float64(slruSize))
	if slru20 < 1 {
		slru20 = 1
	}

	data := make(map[uint64]*list.Element, size)

	return &T{
		w:       0,
		samples: samples,

		countSketch: newCM4(size),
		bouncer:     newDoorkeeper(samples, 0.01),

		data: data,

		lru:  newLRU(lruSize, data),
		slru: newSLRU(slru20, slruSize-slru20, data),
	}
}

func (t *T) Get(key uint64) (interface{}, bool) {
	t.w++
	if t.w == t.samples {
		t.countSketch.reset()
		t.bouncer.reset()
		t.w = 0
	}

	val, ok := t.data[key]
	if !ok {
		t.countSketch.add(key)
		return nil, false
	}

	item := val.Value.(*Item)
	t.countSketch.add(item.Key)

	if item.listid == 0 {
		t.lru.get(val)
	} else {
		t.slru.get(val)
	}

	if item.ExpireAt.IsZero() || time.Now().Before(item.ExpireAt) {
		return item.Value, true
	}

	return nil, false
}

func (t *T) Set(newItem *Item) {
	oldItem, evicted := t.lru.add(newItem)
	if !evicted {
		return
	}

	// estimate count of what will be evicted from slru
	victim := t.slru.victim()
	if victim == nil {
		t.slru.add(oldItem)
		return
	}

	if !t.bouncer.allow(oldItem.Key) {
		return
	}

	vcount := t.countSketch.estimate(victim.Key)
	ocount := t.countSketch.estimate(oldItem.Key)

	if ocount < vcount {
		return
	}

	t.slru.add(oldItem)
}

func (t *T) Del(key uint64) {
	val, ok := t.data[key]
	if ok {
		item := val.Value.(*Item)
		item.Value = nil
		item.ExpireAt = time.Unix(0, 0)
	}
}

//------------------------------------------------------------------------------

type SyncT struct {
	mu sync.Mutex
	t  *T
}

func NewSync(size int, samples int) *SyncT {
	return &SyncT{
		t: New(size, samples),
	}
}

func (t *SyncT) Get(key uint64) (interface{}, bool) {
	t.mu.Lock()
	val, ok := t.t.Get(key)
	t.mu.Unlock()
	return val, ok
}

func (t *SyncT) Set(item *Item) {
	t.mu.Lock()
	t.t.Set(item)
	t.mu.Unlock()
}

func (t *SyncT) Del(key uint64) {
	t.mu.Lock()
	t.t.Del(key)
	t.mu.Unlock()
}
