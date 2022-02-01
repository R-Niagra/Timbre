package core

import (
	"sync"

	"github.com/guyu96/go-timbre/log"
	"github.com/guyu96/go-timbre/storage"
)

const (
	// DefaultBlockpoolCapacity is the default capacity for blockpool.
	DefaultBlockpoolCapacity = 128
)

// Blockpool is a thread-safe LRU cache of incoming blocks that cannot be directly added to the blockchain yet.
type Blockpool struct {
	mtx sync.Mutex
	lru *storage.LRU
}

// NewBlockpool creates a new blockpool.
func NewBlockpool(capacity int) (*Blockpool, error) {
	lru, err := storage.NewLRU(capacity)
	if err != nil {
		return nil, err
	}
	return &Blockpool{lru: lru}, nil
}

// Capacity returns the capacity of blockpool.
func (bp *Blockpool) Capacity() int {
	bp.mtx.Lock()
	capacity := bp.lru.Capacity()
	bp.mtx.Unlock()
	return capacity
}

// Size returns the size of blockpool.
func (bp *Blockpool) Size() int {
	bp.mtx.Lock()
	size := bp.lru.Size()
	bp.mtx.Unlock()
	return size
}

// Add adds a block to blockpool.
func (bp *Blockpool) Add(block *Block) {
	bp.mtx.Lock()
	bp.lru.Add(string(block.Hash()), block)
	bp.mtx.Unlock()
}

// Remove removes the block with the specified hash from blockpool.
func (bp *Blockpool) Remove(blockHash []byte) {
	bp.mtx.Lock()
	bp.lru.Remove(string(blockHash))
	bp.mtx.Unlock()
}

func (bp *Blockpool) Contains(block *Block) bool {
	bp.mtx.Lock()
	defer bp.mtx.Unlock()
	return bp.lru.Contains(string(block.Hash()))
}

func (bp *Blockpool) ContainsBlockByHash(blockHash []byte) bool {
	bp.mtx.Lock()
	defer bp.mtx.Unlock()
	return bp.lru.Contains(string(blockHash))
}

// GetTails returns a slice of tail blocks in blockpool. A tail block is a block with no child blocks in blockpool.
func (bp *Blockpool) GetTails() []*Block {
	bp.mtx.Lock()
	defer bp.mtx.Unlock()

	tails := []*Block{}
	parentChild := make(map[string]*Block)
	for _, blockHash := range bp.lru.Keys() {
		val, _ := bp.lru.Peek(blockHash)
		block := val.(*Block)
		parentChild[string(block.ParentHash())] = block
	}
	for _, child := range parentChild {
		// The child is a tail block if it is not a parent.
		if _, exist := parentChild[string(child.Hash())]; !exist {
			tails = append(tails, child)
		}
	}
	return tails
}

func (bp *Blockpool) PrintBlockPool() {
	for _, blockHash := range bp.lru.Keys() {
		val, _ := bp.lru.Peek(blockHash)
		if val != nil {
			block := val.(*Block)
			log.Info().Msgf("Block in pool %s", block.String())
		} else {
			log.Info().Msgf("Block Nill in pool for blockHash %s", blockHash)
		}
	}
}

// GetChainByTail returns the contiguous chain of blocks ending with the specified tail block in blockpool.
func (bp *Blockpool) GetChainByTail(tail *Block) []*Block {
	bp.mtx.Lock()
	defer bp.mtx.Unlock()

	tailHashStr := string(tail.Hash())
	if !bp.lru.Contains(tailHashStr) {
		return nil
	}

	chain := []*Block{tail}
	parentHashStr := string(tail.ParentHash())
	for bp.lru.Contains(parentHashStr) {
		val, _ := bp.lru.Peek(parentHashStr)
		parent := val.(*Block)
		chain = append(chain, parent)
		parentHashStr = string(parent.ParentHash())
	}

	// Reverse the order of chain slice.
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}
