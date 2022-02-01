package storage

import (
	"fmt"
	"testing"
)

func TestLRU(t *testing.T) {
	_, err := NewLRU(0)
	if err == nil {
		t.Error("Created LRU with negative capacity.")
	}

	_, err = NewLRU(-1)
	if err == nil {
		t.Error("Created LRU with zero capacity.")
	}

	capacity := 100
	lru, err := NewLRU(capacity)
	if err != nil {
		t.Error(err)
	}

	for i := 0; i < capacity; i++ {
		lru.Add(i, i)
	}
	if lru.Size() != capacity {
		t.Error("Incorrect add when within capacity.")
	}

	for i := 0; i < capacity; i++ {
		var val interface{}
		var ok bool
		if i == capacity-1 {
			val, ok = lru.Peek(i)
		} else {
			val, ok = lru.Get(i)
		}
		if !ok {
			t.Error("Failed to get existing key.")
		}
		if val.(int) != i {
			t.Error("Failed to get correct value for existing key.")
		}
	}

	lru.Add(-1, -1)
	if lru.Size() != lru.Capacity() {
		t.Error("Incorrect add when over capacity.")
	}
	if lru.Contains(capacity - 1) {
		t.Error("Failed to remove least recently used element.")
	}

	for i := 0; i < capacity-1; i++ {
		lru.Remove(i)
		if lru.Contains(i) {
			t.Error("Failed to remove key.")
		}
		if lru.Size() != lru.Capacity()-i-1 {
			t.Error("Incorrect size after remove.")
		}
	}

	lru.Remove(capacity + 1)
	if lru.Size() != 1 {
		t.Error("Incorrect remove when key does not exist.")
	}

	lru.Add(-1, 0)
	if lru.Size() != 1 {
		t.Error("Incorrect size after add (update).")
	}
	val, ok := lru.Get(-1)
	if !ok {
		t.Error("Failed to get existing key.")
	}
	if val.(int) != 0 {
		t.Error("Failed to get correct value for existing key after add (update).")
	}
	lru.Add(1, 99)
	value, err := lru.Pop()
	fmt.Println(value)
	if value.(int) != 0 {
		t.Error("Failed to get correct value for existing key after add (update).")
	}

}
