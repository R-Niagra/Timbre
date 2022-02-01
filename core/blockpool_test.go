package core

// import (
// 	"testing"

// 	"github.com/guyu96/noise/crypto/edwards25519"
// )

// func TestBlockpool(t *testing.T) {
// 	capacity := 50
// 	bp, err := NewBlockpool(capacity)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	publicKey, privateKey, err := edwards25519.GenerateKey(nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	genesis, err := NewGenesisBlock(publicKey, 0, nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	// Add first fork to block pool.
// 	blocks1 := MakeTestBlocks(genesis, 5, publicKey, privateKey)
// 	for _, b1 := range blocks1 {
// 		bp.Add(b1)
// 	}
// 	if bp.Size() != len(blocks1) {
// 		t.Error("Incorrect add when within capacity.")
// 	}
// 	tails1 := bp.GetTails()
// 	if len(tails1) != 1 {
// 		t.Error("Incorrect tail blocks.")
// 	}
// 	chain := bp.GetChainByTail(tails1[0])
// 	if len(chain) != len(blocks1) {
// 		t.Error("Incorrect chain length.")
// 	}

// 	for i := range blocks1 {
// 		if !blocks1[i].IsSameAs(chain[i]) {
// 			LogBlocks(blocks1)
// 			LogBlocks(chain)
// 			t.Error("Incorrect blocks or block order in chain.")
// 		}
// 	}

// 	// Add second fork to blockpool.
// 	blocks2 := MakeTestBlocks(blocks1[1], 10, publicKey, privateKey)
// 	for _, b2 := range blocks2 {
// 		bp.Add(b2)
// 	}
// 	if bp.Size() != len(blocks1)+len(blocks2) {
// 		t.Error("Incorrect add when within capacity.")
// 	}
// 	tails2 := bp.GetTails()
// 	if len(tails2) != 2 {
// 		t.Error("Incorrect tail blocks.")
// 	}

// 	// Add third fork to blockpool in two separate steps.
// 	blocks3 := MakeTestBlocks(genesis, 40, publicKey, privateKey)
// 	for i, b3 := range blocks3 {
// 		if i == len(blocks3)-1 {
// 			break
// 		}
// 		bp.Add(b3)
// 	}
// 	if bp.Size() != capacity {
// 		t.Error("Incorrect add when over capacity")
// 	}
// 	tails3 := bp.GetTails()
// 	if len(tails3) != 3 {
// 		t.Error("Incorrect tail blocks.")
// 	}
// 	// This should completely remove the first fork.
// 	bp.Add(blocks3[len(blocks3)-1])
// 	tails4 := bp.GetTails()
// 	if len(tails4) != 2 {
// 		t.Error("Incorrect tail blocks.")
// 	}

// 	// Verify if the chain lengths are correct.
// 	shortChain, longChain := bp.GetChainByTail(tails4[0]), bp.GetChainByTail(tails4[1])
// 	if len(shortChain) > len(longChain) {
// 		shortChain, longChain = longChain, shortChain
// 	}
// 	if len(shortChain) != 10 || len(longChain) != 40 {
// 		t.Error("Incorrect chain length.")
// 	}

// 	// Remove the second fork.
// 	for i, b2 := range blocks2 {
// 		bp.Remove(b2.Hash())
// 		if bp.Size() != bp.Capacity()-i-1 {
// 			t.Error("Incorrect remove.")
// 		}
// 	}
// 	tails5 := bp.GetTails()
// 	if len(tails5) != 1 {
// 		t.Error("Incorrect tail blocks.")
// 	}
// 	if len(bp.GetChainByTail(tails5[0])) != len(blocks3) {
// 		t.Error("Incorrect chain length.")
// 	}
// }
