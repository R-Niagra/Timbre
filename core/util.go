package core

import (
	"errors"
	"fmt"
	"time"

	"github.com/guyu96/go-timbre/log"
	pb "github.com/guyu96/go-timbre/protobuf"
	"github.com/mbilal92/noise"
)

// testBlockNonce is a self-incrementing nonce starting from 0.
var testBlockNonce int64

const (
	ofsetTime = 0
)

func init() {
	testBlockNonce = 0
}

// MakeTestBlock generates a header-only test block using testBlockNonce as the timestamp.
func MakeTestBlock(parentBlock *Block, publicKey noise.PublicKey, privateKey noise.PrivateKey) *Block {
	block, _ := NewBlock(parentBlock, publicKey, nil, nil, nil, nil, nil, ofsetTime, 0)
	// Rehash and sign block.
	block.PbBlock.Header.Timestamp = testBlockNonce
	testBlockNonce++
	newHash, _ := block.CalcHash()
	block.PbBlock.Header.Hash = newHash
	block.Sign(privateKey)
	return block
}

//NewTestBlock returns the test block with only data
func NewTestBlock(parentBlock *Block, minerPublicKey noise.PublicKey, transactions []*pb.Transaction, deals []*pb.Deal, chPrPair []*pb.ChPrPair, votes []*pb.Vote, timeOffset time.Duration, roundNum int64) (*Block, error) {
	block := &Block{
		PbBlock: &pb.Block{
			Header: &pb.BlockHeader{
				Height:         0,
				Timestamp:      0,
				ParentHash:     []byte("test"),
				MinerPublicKey: []byte{},
				RoundNum:       0,
			},

			Data: &pb.BlockData{
				Transactions: transactions,
				Deals:        deals,
				ChPrPair:     chPrPair,
				Votes:        votes,
			},
			State: &pb.BlockState{},
		},
	}
	blockHash, err := block.CalcHash()
	if err != nil {
		log.Info().Msgf("error in blockhash %v", err)
		return nil, err
	}
	// fmt.Println("BLOCK TS: ", (block.pbBlock.Header.Timestamp / (int64(time.Second) / int64(time.Nanosecond))))
	block.PbBlock.Header.Hash = blockHash
	return block, nil
}

// CreateNewBlock returns new block with transactions--> This is the most updated function
func CreateNewBlock(parentBlock *Block, publicKey noise.PublicKey, privateKey noise.PrivateKey, transactions []*pb.Transaction, votes []*pb.Vote, deals []*pb.Deal, chPrPair []*pb.ChPrPair, podfs []*pb.PoDF, timeOffset time.Duration, roundNum int64) *Block {
	if parentBlock == nil {
		panic(errors.New("Parent block was nil"))
	}
	var block *Block

	block, _ = NewBlock(parentBlock, publicKey, transactions, deals, chPrPair, votes, podfs, timeOffset, roundNum)

	if block == nil {
		panic(errors.New("Block is weirdly nil"))
	}

	testBlockNonce++

	newHash, _ := block.CalcHash()
	block.PbBlock.Header.Hash = newHash
	block.Sign(privateKey)
	return block
}

// CreateNewBlock returns new block with transactions--> This is the most updated function
func CreateNewBlockAtHeight(parentBlock *Block, publicKey noise.PublicKey, privateKey noise.PrivateKey, transactions []*pb.Transaction, votes []*pb.Vote, deals []*pb.Deal, chPrPair []*pb.ChPrPair, podfs []*pb.PoDF, timeOffset time.Duration, roundNum, height int64) *Block {
	if parentBlock == nil {
		panic(errors.New("Parent block was nil"))
	}
	var block *Block

	block, _ = TestBlockAtHeight(parentBlock, publicKey, transactions, deals, chPrPair, votes, podfs, timeOffset, roundNum, height)

	if block == nil {
		panic(errors.New("Block is weirdly nil"))
	}
	// block.PbBlock.Header.Timestamp = testBlockNonce
	testBlockNonce++
	//Takes the current time and add the offset
	// block.PbBlock.Header.Timestamp = (time.Now().Add(timeOffset)).Unix()
	newHash, _ := block.CalcHash()
	block.PbBlock.Header.Hash = newHash
	block.Sign(privateKey)
	return block
}

//CreateNewBlockWithTs creates a new block with timestamp
func CreateNewBlockWithTs(parentBlock *Block, publicKey noise.PublicKey, privateKey noise.PrivateKey, transactions []*pb.Transaction, votes []*pb.Vote, deals []*pb.Deal, chPrPair []*pb.ChPrPair, podfs []*pb.PoDF, timeOffset time.Duration, roundNum, bTs int64) *Block {
	if parentBlock == nil {
		panic(errors.New("Parent block was nil"))
	}
	var block *Block

	block, _ = NewBlockWithTs(parentBlock, publicKey, transactions, deals, chPrPair, votes, podfs, timeOffset, roundNum, bTs)

	if block == nil {
		panic(errors.New("Block is weirdly nil"))
	}

	testBlockNonce++
	newHash, _ := block.CalcHash()
	block.PbBlock.Header.Hash = newHash
	block.Sign(privateKey)
	return block
}

//CreateEmptyBlock creates an empty block which could be filled and then should be later on hashed and signed
func CreateEmptyBlock(parent *Block, publicKey noise.PublicKey, timeOffset time.Duration, roundNum, bTs int64, accEntries int) (*Block, error) {
	if parent == nil {
		return nil, errors.New("Parent is nil")
	}
	var block *Block

	block = &Block{
		PbBlock: &pb.Block{
			Header: &pb.BlockHeader{
				Height:         parent.Height() + 1,
				Timestamp:      bTs,
				ParentHash:     parent.Hash(),
				MinerPublicKey: publicKey[:],
				RoundNum:       int64(roundNum),
			},
			Data: &pb.BlockData{
				Transactions: []*pb.Transaction{},
				Deals:        []*pb.Deal{},
				ChPrPair:     []*pb.ChPrPair{},
				Votes:        []*pb.Vote{},
				Podfs:        []*pb.PoDF{},
			},
			State: &pb.BlockState{
				// AccountState: make(map[string]int64),
				AccountStates: make([]*pb.AccState, accEntries),
				Miners:        []string{},
				// Epoch:        curEpoch,
			},
		},
	}

	testBlockNonce++

	return block, nil
}

//CreateTestBlock creates  test block without parent block check
func CreateTestBlock(parentBlock *Block, publicKey noise.PublicKey, privateKey noise.PrivateKey, transactions []*pb.Transaction, votes []*pb.Vote, deals []*pb.Deal, chPrPair []*pb.ChPrPair, timeOffset time.Duration, roundNum int64) *Block {
	var block *Block

	block, _ = NewTestBlock(parentBlock, publicKey, transactions, deals, chPrPair, votes, timeOffset, roundNum)

	if block == nil {
		panic(errors.New("Block is weirdly nil"))
	}
	testBlockNonce++

	newHash, _ := block.CalcHash()
	block.PbBlock.Header.Hash = newHash
	block.Sign(privateKey)
	return block
}

//TODO: Refactor
//Check this functon to fix the time offset
func MakeNewBlock(parentBlock *Block, minerPublicKey noise.PublicKey, privateKey noise.PrivateKey, transactions []*pb.Transaction, deals []*pb.Deal, chPrPair []*pb.ChPrPair, votes []*pb.Vote, podfs []*pb.PoDF) *Block {
	block, _ := NewBlock(parentBlock, minerPublicKey, transactions, deals, chPrPair, votes, podfs, 0, 0)
	// Rehash and sign block.
	block.PbBlock.Header.Timestamp = testBlockNonce
	testBlockNonce++
	newHash, _ := block.CalcHash()
	block.PbBlock.Header.Hash = newHash
	block.Sign(privateKey)
	return block
}

//TODO: Refactor
// MakeTestBlocks generates a contiguous chain of header-only test blocks with incrementing nonce.
func MakeTestBlocks(parentBlock *Block, length int, publicKey noise.PublicKey, privateKey noise.PrivateKey) []*Block {
	blocks := make([]*Block, length)
	blocks[0] = MakeTestBlock(parentBlock, publicKey, privateKey)
	for i := 1; i < length; i++ {
		blocks[i] = MakeTestBlock(blocks[i-1], publicKey, privateKey)
	}
	return blocks
}

//TODO:- Refactor
// SameBlockchains tests if two blockchains are the same based on genesis and tail blocks.
func SameBlockchains(bc1, bc2 *Blockchain) bool {
	if !bc1.Genesis().IsSameAs(bc2.Genesis()) {
		return false
	}
	if len(bc1.tails) != len(bc2.tails) {
		return false
	}
	// Map block hash to block, and map block hash to tail index, both for tail blocks in bc1.
	bc1HashTailMap := make(map[string]*Block)
	bc1HashIndexMap := make(map[string]int)
	for i, tail1 := range bc1.tails {
		tail1HashString := string(tail1.Hash())
		bc1HashTailMap[tail1HashString] = tail1
		bc1HashIndexMap[tail1HashString] = i
	}
	// Check if tail block in tail2 in also a tail block in bc1. Assuming no duplicate tail blocks in bc2, this check ensures a bijection between bc1.tails and bc2.tails.
	for j, tail2 := range bc2.tails {
		tail2HashString := string(tail2.Hash())
		tail1, exist := bc1HashTailMap[tail2HashString]
		if !exist || !tail1.IsSameAs(tail2) {
			return false
		}
		i, exist := bc1HashIndexMap[tail2HashString]
		if !exist || bc1.forkLengths[i] != bc2.forkLengths[j] {
			return false
		}
	}
	return true
}

//TODO:- Refactor
// LogBlocks prints a list of blocks with each block on a new line.
func LogBlocks(blocks []*Block) {
	for _, block := range blocks {
		fmt.Println(block)
	}
}

//MakeTestDeal makes the test deal with the given timestamp
func MakeTestDeal(ts int64) *pb.Deal {

	deal := &pb.Deal{
		Spid:       []byte{0, 0, 0},
		Timestamp:  ts,
		ExpiryTime: ts + 20*1e9,
	}

	return deal
}

//ByteToPublicKey convert byte to publickey
func ByteToPublicKey(pk []byte) noise.PublicKey {
	var Key noise.PublicKey
	copy(Key[:], pk)
	return Key
}
