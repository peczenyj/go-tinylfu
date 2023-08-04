// Package tinylfu is an implementation of the TinyLFU caching algorithm
/*
   http://arxiv.org/abs/1512.00727
*/
package tinylfu

import (
	"container/list"
	"errors"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
)

// LFU interface
type LFU interface {
	Get(key string) (interface{}, bool)
	Add(newItem *Item) error
	Set(newItem *Item)
	Del(key string)
}

// Item type.
type Item struct {
	Key      string
	Value    interface{}
	ExpireAt time.Time
	OnEvict  func()

	listid int
	keyh   uint64
}

func (item Item) expired() bool {
	return !item.ExpireAt.IsZero() && time.Now().After(item.ExpireAt)
}

var _ LFU = (*T)(nil)

// T type.
type T struct {
	w       int
	samples int

	countSketch *cm4
	bouncer     *doorkeeper

	data map[string]*list.Element

	lru  *lruCache
	slru *slruCache
}

// New constructor.
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

	data := make(map[string]*list.Element, size)

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

func (t *T) onEvict(item *Item) {
	if item.OnEvict != nil {
		item.OnEvict()
	}
}

// Get return an item from cache based on key.
func (t *T) Get(key string) (interface{}, bool) {
	t.w++
	if t.w == t.samples {
		t.countSketch.reset()
		t.bouncer.reset()
		t.w = 0
	}

	keyh := xxhash.Sum64String(key)
	t.countSketch.add(keyh)

	val, ok := t.data[key]
	if !ok {
		return nil, false
	}

	item := val.Value.(*Item)
	if item.expired() {
		t.del(val)
		return nil, false
	}

	// Save the value since it is overwritten below.
	value := item.Value

	if item.listid == 0 {
		t.lru.get(val)
	} else {
		t.slru.get(val)
	}

	return value, true
}

// ErrorKeyAlreadyExists will be returned by Add operations if the key already exists.
var ErrKeyAlreadyExists = errors.New("key already exists")

// Add will set an item on cache. If the key already exists the action fails.
func (t *T) Add(newItem *Item) error {
	return t.set(newItem, true)
}

// Set will set an item on cache. If the key already exists the contents are overridden.
func (t *T) Set(newItem *Item) {
	_ = t.set(newItem, false)
}

func (t *T) set(newItem *Item, failIfKeyAlreadyExists bool) error {
	if e, ok := t.data[newItem.Key]; ok {
		if failIfKeyAlreadyExists {
			return ErrKeyAlreadyExists
		}

		// Key is already in our cache.
		// `Set` will act as a `Get` for list movements
		item := e.Value.(*Item)
		item.Value = newItem.Value
		t.countSketch.add(item.keyh)

		if item.listid == 0 {
			t.lru.get(e)
		} else {
			t.slru.get(e)
		}

		return nil
	}

	newItem.keyh = xxhash.Sum64String(newItem.Key)

	oldItem, evicted := t.lru.add(newItem)
	if !evicted {
		return nil
	}

	// estimate count of what will be evicted from slru
	victim := t.slru.victim()
	if victim == nil {
		t.slru.add(oldItem)
		return nil
	}

	if !t.bouncer.allow(oldItem.keyh) {
		t.onEvict(oldItem)
		return nil
	}

	victimCount := t.countSketch.estimate(victim.keyh)
	itemCount := t.countSketch.estimate(oldItem.keyh)

	if itemCount > victimCount {
		t.slru.add(oldItem)
	} else {
		t.onEvict(oldItem)
	}

	return nil
}

// Del remove a key from cache if exists.
func (t *T) Del(key string) {
	if val, ok := t.data[key]; ok {
		t.del(val)
	}
}

func (t *T) del(val *list.Element) {
	item := val.Value.(*Item)
	delete(t.data, item.Key)

	if item.listid == 0 {
		t.lru.Remove(val)
	} else {
		t.slru.Remove(val)
	}

	t.onEvict(item)
}

//------------------------------------------------------------------------------

var _ LFU = (*SyncT)(nil)

type SyncT struct {
	mu sync.RWMutex
	t  *T
}

func NewSync(size int, samples int) *SyncT {
	return &SyncT{
		t: New(size, samples),
	}
}

func (t *SyncT) Get(key string) (interface{}, bool) {
	t.mu.RLock()
	val, ok := t.t.Get(key)
	t.mu.RUnlock()

	return val, ok
}

func (t *SyncT) Set(item *Item) {
	t.mu.Lock()
	t.t.Set(item)
	t.mu.Unlock()
}

func (t *SyncT) Add(item *Item) error {
	t.mu.Lock()
	err := t.t.Add(item)
	t.mu.Unlock()

	return err
}

func (t *SyncT) Del(key string) {
	t.mu.Lock()
	t.t.Del(key)
	t.mu.Unlock()
}
