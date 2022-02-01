package storage

import (
	"container/list"
	"errors"
	"fmt"
	"sync"
)

// LRU implements a least recently used cache.
type LRU struct {
	mutex     sync.Mutex
	capacity  int                           // maximum number of elements in LRU cache
	elements  map[interface{}]*list.Element // holds key-value pairs
	evictList *list.List                    // keep track of which element to evict
}

type kvPair struct {
	key   interface{}
	value interface{}
}

// NewLRU returns a new cache instance.
func NewLRU(capacity int) (*LRU, error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("cache capacity must be positive")
	}
	return &LRU{
		capacity:  capacity,
		elements:  make(map[interface{}]*list.Element),
		evictList: list.New(),
	}, nil
}

// Capacity returns the maximum capacity of the cache.
func (lru *LRU) Capacity() int {
	return lru.capacity
}

// Size returns the number of elements stored in cache.
func (lru *LRU) Size() int {
	return len(lru.elements)
}

// Add adds a new key-value pair or update an existing key's value. The key will be placed at the end of evictList.
func (lru *LRU) Add(key, value interface{}) {

	if lru.Contains(key) {
		lru.Remove(key)
	}

	lru.mutex.Lock()
	lru.elements[key] = lru.evictList.PushBack(kvPair{key: key, value: value})
	lru.mutex.Unlock()

	if lru.Size() > lru.Capacity() {
		lru.Remove(lru.evictList.Front().Value.(kvPair).key)
	}
}

//Pop pops element from the front of the list
func (lru *LRU) Pop() (interface{}, error) {
	lru.mutex.Lock()
	defer lru.mutex.Unlock()

	if lru.Size() == 0 {
		return nil, errors.New("Size of cache is zero")
	}
	key := lru.evictList.Front().Value.(kvPair).key
	value, in := lru.Get(key)
	if in == false {
		return nil, errors.New("Failed to get a key value pair")
	}
	lru.Remove(key)
	return value, nil
}

// Remove removes the specified key from cache and fails silently if key is not present.
func (lru *LRU) Remove(key interface{}) {
	lru.mutex.Lock()
	defer lru.mutex.Unlock()

	if element, ok := lru.elements[key]; ok {
		delete(lru.elements, element.Value.(kvPair).key)
		lru.evictList.Remove(element)
	}
}

// Contains returns whether the cache contains the specified key. The key's position in evictList will not be updated.
func (lru *LRU) Contains(key interface{}) bool {
	lru.mutex.Lock()
	defer lru.mutex.Unlock()
	_, ok := lru.elements[key]
	return ok
}

// Get returns the value corresponding to the specified key stored in the cache, and ok indicates whether the key is present.
func (lru *LRU) Get(key interface{}) (interface{}, bool) {
	lru.mutex.Lock()
	defer lru.mutex.Unlock()
	if element, ok := lru.elements[key]; ok {
		lru.evictList.MoveToBack(element)
		return element.Value.(kvPair).value, true
	}
	return nil, false
}

// Peek is the same as Get without updating the "recentness" of the element.
func (lru *LRU) Peek(key interface{}) (interface{}, bool) {
	lru.mutex.Lock()
	defer lru.mutex.Unlock()
	if element, ok := lru.elements[key]; ok {
		return element.Value.(kvPair).value, true
	}
	return nil, false
}

// Keys returns a slice of keys contained in the cache.
func (lru *LRU) Keys() []interface{} {
	lru.mutex.Lock()
	defer lru.mutex.Unlock()

	keys := make([]interface{}, lru.Size())
	i := 0
	for key := range lru.elements {
		keys[i] = key
		i++
	}
	return keys
}
