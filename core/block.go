package core

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/guyu96/go-timbre/crypto"
	"github.com/guyu96/go-timbre/log"
	pb "github.com/guyu96/go-timbre/protobuf"
	"github.com/mbilal92/noise"
)

// Block wraps the protobuf Block struct.
type Block struct {
	PbBlock *pb.Block
}

// NewBlock generates a new block given the previous block and the new transactions, deals, proofs and votes.
func NewBlock(parentBlock *Block, minerPublicKey noise.PublicKey, transactions []*pb.Transaction, deals []*pb.Deal, chPrPair []*pb.ChPrPair, votes []*pb.Vote, podfs []*pb.PoDF, timeOffset time.Duration, roundNum int64) (*Block, error) {
	block := &Block{
		PbBlock: &pb.Block{
			Header: &pb.BlockHeader{
				Height: parentBlock.Height() + 1,
				// Timestamp:      (time.Now().Add(timeOffset)).UnixNano(),
				Timestamp:      time.Now().UnixNano(),
				ParentHash:     parentBlock.Hash(),
				MinerPublicKey: minerPublicKey[:],
				RoundNum:       int64(roundNum),
			},
			Data: &pb.BlockData{
				Transactions: transactions,
				Deals:        deals,
				ChPrPair:     chPrPair,
				Votes:        votes,
				Podfs:        podfs,
			},
			State: &pb.BlockState{},
		},
	}

	return block, nil
}

// NewBareBlock generates a new block given the previous block and the new transactions, deals, proofs and votes.
func NewBareBlock(parentHash []byte, minerPublicKey noise.PublicKey, transactions []*pb.Transaction, deals []*pb.Deal, chPrPair []*pb.ChPrPair, votes []*pb.Vote, podfs []*pb.PoDF, timeOffset time.Duration, roundNum int64) *Block {
	block := &Block{
		PbBlock: &pb.Block{
			Header: &pb.BlockHeader{

				Timestamp:      time.Now().UnixNano(),
				ParentHash:     parentHash,
				MinerPublicKey: minerPublicKey[:],
				RoundNum:       int64(roundNum),
			},
			Data: &pb.BlockData{
				Transactions: transactions,
				Deals:        deals,
				ChPrPair:     chPrPair,
				Votes:        votes,
				Podfs:        podfs,
			},
			State: &pb.BlockState{},
		},
	}

	return block
}

//TestBlockAtHeight creates a test block at a given height
func TestBlockAtHeight(parentBlock *Block, minerPublicKey noise.PublicKey, transactions []*pb.Transaction, deals []*pb.Deal, chPrPair []*pb.ChPrPair, votes []*pb.Vote, podfs []*pb.PoDF, timeOffset time.Duration, roundNum, height int64) (*Block, error) {
	block := &Block{
		PbBlock: &pb.Block{
			Header: &pb.BlockHeader{
				Height:         height,
				Timestamp:      time.Now().UnixNano(),
				ParentHash:     parentBlock.Hash(),
				MinerPublicKey: minerPublicKey[:],
				RoundNum:       int64(roundNum),
			},
			Data: &pb.BlockData{
				Transactions: transactions,
				Deals:        deals,
				ChPrPair:     chPrPair,
				Votes:        votes,
				Podfs:        podfs,
			},
			State: &pb.BlockState{},
		},
	}

	return block, nil
}

//NewBlockWithTs creates a new block with the given timestamp
func NewBlockWithTs(parentBlock *Block, minerPublicKey noise.PublicKey, transactions []*pb.Transaction, deals []*pb.Deal, chPrPair []*pb.ChPrPair, votes []*pb.Vote, podfs []*pb.PoDF, timeOffset time.Duration, roundNum, bTs int64) (*Block, error) {
	block := &Block{
		PbBlock: &pb.Block{
			Header: &pb.BlockHeader{
				Height:         parentBlock.Height() + 1,
				Timestamp:      bTs,
				ParentHash:     parentBlock.Hash(),
				MinerPublicKey: minerPublicKey[:],
				RoundNum:       int64(roundNum),
			},
			Data: &pb.BlockData{
				Transactions: transactions,
				Deals:        deals,
				ChPrPair:     chPrPair,
				Votes:        votes,
				Podfs:        podfs,
			},
			State: &pb.BlockState{},
		},
	}

	return block, nil
}

//HashAndSign put the hash of the block and signs it
func (block *Block) HashAndSign(privateKey noise.PrivateKey) error {
	blockHash, err := block.CalcHash()
	if err != nil {
		log.Info().Msgf("error in blockhash %v", err)
		return err
	}
	block.PbBlock.Header.Hash = blockHash
	block.Sign(privateKey)
	return nil
}

//GetRoundNum returns the round number of the block
func (block *Block) GetRoundNum() int64 {
	return block.PbBlock.GetHeader().GetRoundNum()
}

// NewBlockFromBytes deserializes the given byte representation of the block.
func NewBlockFromBytes(blockBytes []byte) (*Block, error) {
	block := new(Block)
	block.PbBlock = new(pb.Block)
	if err := proto.Unmarshal(blockBytes, block.PbBlock); err != nil {
		return nil, err
	}
	return block, nil
}

// NewGenesisBlock generates the genesis block.
func NewGenesisBlock(parentHash []byte, minerPublicKey noise.PublicKey, timeOffset time.Duration, transactions []*pb.Transaction, h int64) (*Block, error) {
	block := &Block{
		PbBlock: &pb.Block{
			Header: &pb.BlockHeader{
				Height:         h,
				Timestamp:      time.Now().UnixNano(),
				ParentHash:     parentHash,
				MinerPublicKey: minerPublicKey[:],
				RoundNum:       1, //It has to be produced in Genesis
				ChainId:        DefaultConfig.ChainID,
			},
			Data: &pb.BlockData{
				Transactions: transactions, //Entertain delegate registration transaction
			},
			State: &pb.BlockState{},
		},
	}
	blockHash, err := block.CalcHash()
	if err != nil {
		return nil, err
	}
	block.PbBlock.Header.Hash = blockHash
	return block, nil
}

//NewGenesisBlockWithTs creates the genesis block with the provided TS
func NewGenesisBlockWithTs(parentHash []byte, minerPublicKey noise.PublicKey, timeOffset time.Duration, transactions []*pb.Transaction, bTs int64, h int64) (*Block, error) {
	block := &Block{
		PbBlock: &pb.Block{
			Header: &pb.BlockHeader{
				Height: h,
				// Timestamp:      (time.Now().Add(timeOffset)).UnixNano(),
				Timestamp:      bTs,
				ParentHash:     parentHash,
				MinerPublicKey: minerPublicKey[:],
				RoundNum:       1, //It has to be produced in Genesis block
				ChainId:        DefaultConfig.ChainID,
			},
			Data: &pb.BlockData{
				Transactions: transactions, //Entertain delegate registration transaction
			},
			State: &pb.BlockState{},
		},
	}
	blockHash, err := block.CalcHash()
	if err != nil {
		fmt.Println("hash error: ", err.Error())
		return nil, err
	}

	block.PbBlock.Header.Hash = blockHash
	return block, nil
}

//GetMinersFromDigestBlock returns the miners in the digestBlock
func (block *Block) GetMinersFromDigestBlock() []string {
	if block.PbBlock.GetState().GetMiners() == nil {
		return []string{}
	}
	return block.PbBlock.GetState().GetMiners()
}

//ResetTimestamp resets the timestamp of the block
func (block *Block) ResetTimestamp(ts int64) {
	block.PbBlock.Header.Timestamp = ts

}

//ResetHash resets the hash of the block
func (block *Block) ResetHash() {
	newHash, _ := block.CalcHash()
	block.PbBlock.Header.Hash = newHash
}

// Hash returns the hash of the block.
func (block *Block) Hash() []byte {
	return block.PbBlock.Header.GetHash()
}

// ParentHash returns the hash of the parent block.
func (block *Block) ParentHash() []byte {
	return block.PbBlock.GetHeader().GetParentHash()
}

// MinerPublicKey returns the public key of the block miner.
func (block *Block) MinerPublicKey() noise.PublicKey {
	var Key noise.PublicKey
	copy(Key[:], block.PbBlock.Header.MinerPublicKey)
	return Key
}

//PublickeyBytes returns the miner public key in bytes
func (block *Block) PublickeyBytes() []byte {
	return block.PbBlock.Header.MinerPublicKey
}

//GetHeader returns the header of the block
func (block *Block) GetHeader() *pb.BlockHeader {
	return block.PbBlock.GetHeader()
}

// Height returns the height of the block.
func (block *Block) Height() int64 {
	return block.PbBlock.GetHeader().GetHeight()
}

// HeightInt returns the height of the block as type int.
func (block *Block) HeightInt() int {
	return int(block.Height())
}

// HasValidHash verifies if the block hash is the correct hash of block header and block data.
func (block *Block) HasValidHash() (bool, error) {
	PbBlock := block.GetProtoBlock()
	if PbBlock == nil {
		return false, errors.New("Block is nil")
	}
	trueHash, err := block.GetValidatingHash()
	if err != nil {
		return false, err
	}
	return bytes.Equal(PbBlock.Header.GetHash(), trueHash), nil
}

// HasValidSig verifies if the block signature is valid.
func (block *Block) HasValidSig() (bool, error) {
	publicKey := block.MinerPublicKey()
	message, err := block.serializeIncHeader(true)
	if err != nil {
		return false, err
	}

	//Appending the chain-Id to check if the block is meant for the same network
	message = append(message, []byte(DefaultConfig.ChainID)...)

	return publicKey.VerifyB(message, block.PbBlock.Header.MinerSig), nil
}

//HasValidSigWithID validates the hash of the block
func (block *Block) HasValidSigWithID(chainID string) bool {
	publicKey := block.MinerPublicKey()
	bBytes, err := block.serializeIncHeader(true)
	if err != nil {
		return false
	}
	// //Appending the chain-Id to check
	bBytes = append(bBytes, []byte(chainID)...)

	return publicKey.VerifyB(bBytes, block.PbBlock.Header.MinerSig)
}

//PrintState prints the state of the block
func (block *Block) PrintState() {
	bState := block.PbBlock.GetState()
	accMap := bState.GetAccountStates()

	for k, v := range accMap {
		fmt.Println("k: ", k, " v", v)
	}
}

// IsValid verifies if the block is valid.
// TODO: add checks for compoenents in block data.
func (block *Block) IsValid() bool {
	// Check block hash.
	if hasValidHash, err := block.HasValidHash(); !hasValidHash || err != nil {
		log.Error().Msgf("In valid hash Error %v", err)

		return false
	}
	// Check Miner signature.
	if hasValidSig, err := block.HasValidSig(); !hasValidSig || err != nil {
		log.Error().Msgf("signature Error %v", err)

		return false
	}
	// Check genesis status consistency.
	if block.Height() == 0 {

	} else {
		if block.ParentHash() == nil {
			log.Error().Msgf("block.ParentHash() is nil")
			return false
		}
	}

	return true
}

// IsGenesis checks if the block is the genesis block.
func (block *Block) IsGenesis() bool {
	return block.Height() == 0
}

//IsEmpty checks if block is empty or not
func (block *Block) IsEmpty() bool {
	bTrans, bVotes, bDeals, bProof := block.GetTransactions(), block.GetVotes(), block.GetDeals(), block.GetChPrPair()
	if len(bTrans) == 0 && len(bVotes) == 0 && len(bDeals) == 0 && len(bProof) == 0 && block.IsDigestBlock() == false { //If all variables are zero then block is empty
		return true
	}
	return false
}

// IsValidParentOf checks if the block is a valid parent of childBlock.
func (block *Block) IsValidParentOf(childBlock *Block) bool {
	return bytes.Equal(block.Hash(), childBlock.ParentHash()) && block.Height() == childBlock.Height()-1
}

// IsSameAs checks if two blocks are the same by comparing their hashes. It assumes that their hashes are valid.
func (block *Block) IsSameAs(other *Block) bool {
	return bytes.Equal(block.Hash(), other.Hash())
}

// ToBytes serializes the block to bytes.
func (block *Block) ToBytes() ([]byte, error) {
	return proto.Marshal(block.PbBlock)
}

// Sign adds the miner's signature to the block given the miner's private key.
func (block *Block) Sign(minerPrivateKey noise.PrivateKey) error {
	message, err := block.serializeHeader()
	if err != nil {
		return err
	}
	//Appending the chain-id so that only nodes on my chain-id can identify it
	message = append(message, []byte(DefaultConfig.ChainID)...)

	block.PbBlock.Header.MinerSig = minerPrivateKey.SignB(message)

	return nil
}

// CalcHash returns the SHA-256 hash of block header and block data.
func (block *Block) CalcHash() ([]byte, error) {
	message, err := block.serializeBlock()
	if err != nil {
		return nil, err
	}
	return crypto.Sha256(message), nil
}

//GetValidatingHash get the hash of incoming block for the validation
func (block *Block) GetValidatingHash() ([]byte, error) {
	message, err := block.serializeIncBlock(true, true)
	if err != nil {
		return nil, err
	}
	return crypto.Sha256(message), nil
}

// serializeIncBlock serializes header, data and state of incoming into one byte slice. It has hash and sign set to nil
func (block *Block) serializeIncBlock(nilHash, nilSig bool) ([]byte, error) {
	ans := []byte{}

	tHash := make([]byte, len(block.PbBlock.Header.Hash))
	tSig := make([]byte, len(block.PbBlock.Header.MinerSig))
	copy(tHash, block.PbBlock.Header.Hash)
	copy(tSig, block.PbBlock.Header.MinerSig)

	if nilHash == true {
		block.PbBlock.Header.Hash = nil //Should be set to nil to nil for incoming block is hashed the same way
	}
	if nilSig == true {
		block.PbBlock.Header.MinerSig = nil
	}

	headerBytes, err := proto.Marshal(block.PbBlock.GetHeader())

	if err != nil {
		return nil, fmt.Errorf("error marshaling Block header: %s", err)
	}
	dataBytes, err := proto.Marshal(block.PbBlock.GetData())

	if err != nil {
		return nil, fmt.Errorf("error marshaling Block data: %s", err)
	}
	ans = append(headerBytes, dataBytes...)

	stateBytes, err := proto.Marshal(block.PbBlock.GetState())

	if err != nil {
		return nil, fmt.Errorf("error marshaling Block state: %s", err)
	}

	block.PbBlock.Header.Hash = tHash
	block.PbBlock.Header.MinerSig = tSig

	return append(ans, stateBytes...), nil
}

// serializeBlock serializes header, data and state into one byte slice.
func (block *Block) serializeBlock() ([]byte, error) {
	ans := []byte{}

	headerBytes, err := proto.Marshal(block.PbBlock.GetHeader())

	if err != nil {
		return nil, fmt.Errorf("error marshaling block header: %s", err)
	}
	dataBytes, err := proto.Marshal(block.PbBlock.GetData())

	if err != nil {
		return nil, fmt.Errorf("error marshaling block data: %s", err)
	}
	ans = append(headerBytes, dataBytes...)

	stateBytes, err := proto.Marshal(block.PbBlock.GetState())
	if err != nil {
		return nil, fmt.Errorf("error marshaling block state: %s", err)
	}

	return append(ans, stateBytes...), nil
}

//serializeHeader serializes the block header
func (block *Block) serializeHeader() ([]byte, error) {
	headerBytes, err := proto.Marshal(block.PbBlock.GetHeader())
	if err != nil {
		return nil, fmt.Errorf("error marshaling block header: %s", err)
	}
	return headerBytes, nil
}

//serializeHeader serializes the incoming block header-> It requires to remove signature and hash
func (block *Block) serializeIncHeader(nilSig bool) ([]byte, error) {

	tSig := make([]byte, len(block.PbBlock.Header.MinerSig))
	copy(tSig, block.PbBlock.Header.MinerSig)

	if nilSig { //Should be set to nil becasue while generating block sign was done with nil minerSig field
		block.PbBlock.Header.MinerSig = nil
	}

	headerBytes, err := proto.Marshal(block.PbBlock.GetHeader())
	if err != nil {
		return nil, fmt.Errorf("error marshaling block header: %s", err)
	}
	block.PbBlock.Header.MinerSig = tSig

	return headerBytes, nil
}

func (block *Block) String() string {
	blockHeight := block.Height()
	blockHash := hex.EncodeToString(block.Hash())

	return fmt.Sprintf("Block #%d <%s>", blockHeight, blockHash)
}

//GetTransactions returns the transactions included in the block
func (block *Block) GetTransactions() []*pb.Transaction {
	return block.PbBlock.Data.GetTransactions()
}

//GetVotes returns the votes in the block
func (block *Block) GetVotes() []*pb.Vote {
	return block.PbBlock.Data.GetVotes()
}

//GetDeals returns the deals in the block
func (block *Block) GetDeals() []*pb.Deal {
	return block.PbBlock.Data.GetDeals()
}

//GetPodf returns proof of double forgery
func (block *Block) GetPodf() []*pb.PoDF {
	return block.PbBlock.GetData().GetPodfs()
}

//GetChPrPair returns the ChPrPair carried by the block
func (block *Block) GetChPrPair() []*pb.ChPrPair {
	return block.PbBlock.Data.GetChPrPair()
}

//GetTimestamp  returns the timestamp of the block
func (block *Block) GetTimestamp() int64 {
	return block.PbBlock.Header.GetTimestamp()
}

//GetProtoBlock returns the proto block in the block struct
func (block *Block) GetProtoBlock() *pb.Block {
	return block.PbBlock
}

//IsDigestBlock checks if the block has the state or not
func (block *Block) IsDigestBlock() bool {
	if block.PbBlock.State != nil {
		if len(block.PbBlock.State.GetAccountStates()) != 0 {
			return true
		}
	}
	return false
}

//GetEpochNum return the epoch in the chain
func (block *Block) GetEpochNum() int32 {
	if block.PbBlock.State != nil {
		return block.PbBlock.State.Epoch
	}
	return 0
}

//ValidateHeaderSign validates the header of podf
func ValidateHeaderSign(h *pb.BlockHeader) error {
	podfBytes, err := proto.Marshal(h)
	if err != nil {
		return err
	}

	var ch = new(pb.BlockHeader)
	err = proto.Unmarshal(podfBytes, ch)
	if err != nil {
		return err
	}

	//setting signature to nil becasue signature was gienrated with nil
	ch.MinerSig = nil

	headerBytes, err := proto.Marshal(ch)
	if err != nil {
		return fmt.Errorf("error marshaling block header: %s", err)
	}

	minerPk := h.GetMinerPublicKey()
	var Key noise.PublicKey
	copy(Key[:], minerPk)

	valid := Key.VerifyB(headerBytes, h.GetMinerSig())
	if valid == false {
		return errors.New("Sign is not valid")
	}

	return nil
}
