package core

import (
	"bytes"
	"container/list"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/guyu96/go-timbre/crypto"
	"github.com/guyu96/go-timbre/log"
	pb "github.com/guyu96/go-timbre/protobuf"
	"github.com/guyu96/go-timbre/storage"
	"github.com/mbilal92/noise"
)

// Blockchain implements functions for persisting and updating the blockchain.
type Blockchain struct {
	db            *storage.Database // database for blockchain persistence
	genesis       *Block            // the genesis block
	mainTailIndex int               // the index of the main tail block that has the maximum fork lengths in tails

	tails  []*Block // a list of tail blocks corresponding to different forks
	muTail sync.RWMutex

	forkLengths   []int // a list of fork lengths, one for each tail block in tails
	muForkLengths sync.RWMutex

	mainForkMap  map[int64][]byte //A key value pair key-> Block height Value-> The hash
	muMainforMap sync.RWMutex

	IrreversibleBlock             int64
	Last2by3MinedCommitteeMembers *list.List
}

// InitBlockchain initializes a blockchain
func InitBlockchain(db *storage.Database) (*Blockchain, error) {

	if db.HasBucket(storage.BlockBucket) == false {
		log.Info().Msgf("Initializing block bucket")
		db.NewBucket(storage.BlockBucket)
	}

	bc := &Blockchain{
		db:                            db,
		forkLengths:                   []int{0},
		mainTailIndex:                 -1,
		mainForkMap:                   make(map[int64][]byte),
		IrreversibleBlock:             int64(0),
		Last2by3MinedCommitteeMembers: list.New(),
	}

	return bc, nil
}

func (bc *Blockchain) ResetBlockChain(db *storage.Database, genesis *Block) (*Blockchain, error) {
	db.DeleteBucket(storage.BlockBucket) //Deleting bucket to make node every time:- TODO:Change it to restore state
	err := db.NewBucket(storage.BlockBucket)
	if err != nil {
		log.Info().Err(err)
		return nil, err
	}

	return InitBlockchain(db)
}

// CreateBlockchain creates a new blockchain. This method should be called to start a new blockchain with a new genesis block.
func CreateBlockchain(db *storage.Database, minerPublicKey noise.PublicKey, minerPrivateKey noise.PrivateKey) (*Blockchain, error) {
	genesis, err := NewGenesisBlock(nil, minerPublicKey, 0, []*pb.Transaction{}, 0)
	if err != nil {
		return nil, errors.New("error creating genesis block")
	}
	if err := genesis.Sign(minerPrivateKey); err != nil {
		return nil, errors.New("error signing genesis block")
	}
	return InitBlockchain(db)
}

//TODO: populate MainFOrkMap in this function (done)
// LoadBlockchain loads the blockchain from disk.
func LoadBlockchain(db *storage.Database) (*Blockchain, error) {
	bc := &Blockchain{db: db}
	// Find the genesis block and populate the parent-child, child-parent, and visited maps.
	parentChild := make(map[string][]byte) // byte slice cannot be used as map keys
	childParent := make(map[string][]byte)
	visited := make(map[string]bool)
	var genesis *Block
	err := db.ForEach(storage.BlockBucket, func(blockHash, blockBytes []byte) error {
		block, err := NewBlockFromBytes(blockBytes)
		if err != nil {
			return err
		}
		if !block.IsValid() {
			return errors.New("error loading blockchain - invalid block")
		}
		blockHashStr := string(blockHash)
		parentHashStr := string(block.ParentHash())
		if block.IsGenesis() {
			if genesis != nil {
				return fmt.Errorf("error setting genesis block - duplicate genesis blocks")
			}
			genesis = block
		} else {
			parentChild[parentHashStr] = blockHash
			childParent[blockHashStr] = block.ParentHash()
		}
		visited[blockHashStr] = false
		return nil
	})
	if err != nil {
		return nil, err
	}
	if genesis == nil {
		return nil, errors.New("error loading blockchain - no genesis block")
	}
	// Find tail blocks
	tails := make([]*Block, 0)
	for _, child := range parentChild {
		// The child is a tail block if it is not a parent.
		if _, exist := parentChild[string(child)]; !exist {
			block, err := bc.GetBlockByHash(child)
			if err != nil {
				return nil, err
			}
			tails = append(tails, block)
		}
	}
	// Blockchain only has genesis block.
	if len(tails) == 0 {
		tails = append(tails, genesis)
	}
	// Find the longest fork. Choose the first one in case of ties.
	forkLengths := make([]int, len(tails))
	mainTailIndex := -1
	mainForkLength := -1
	for i, tail := range tails {
		blockHash := tail.Hash()
		blockHashStr := string(blockHash)
		visited[blockHashStr] = true
		currentForkLength := 0
		for {
			parentHash, exist := childParent[string(blockHash)]
			if !exist {
				break
			}
			blockHash = parentHash
			blockHashStr = string(blockHash)
			visited[blockHashStr] = true
			currentForkLength++
		}
		if !bytes.Equal(blockHash, genesis.Hash()) {
			return nil, errors.New("error loading blockchain - tail block does not have genesis block as root")
		}
		forkLengths[i] = currentForkLength
		if currentForkLength > mainForkLength {
			mainForkLength = currentForkLength
			mainTailIndex = i
		}
	}
	for blockHash, visited := range visited {
		if !visited {
			block, _ := bc.GetBlockByHash([]byte(blockHash))
			return nil, fmt.Errorf("error loading blockchain - %s not in any fork", block)
		}
	}
	bc.muTail.Lock()
	bc.genesis = genesis
	bc.tails = tails
	bc.muTail.Unlock()

	bc.muForkLengths.Lock()
	bc.forkLengths = forkLengths
	bc.muForkLengths.Unlock()

	bc.setMainTailIndex(mainTailIndex)
	bc.mainForkMap = make(map[int64][]byte)
	bc.updateMainForkMap(bc.GetMainTail())
	bc.IrreversibleBlock = int64(0)
	bc.Last2by3MinedCommitteeMembers = list.New()
	return bc, nil
}

func (bc *Blockchain) setMainTailIndex(i int) {

	bc.muTail.Lock()
	bc.mainTailIndex = i
	bc.muTail.Unlock()
}

func (bc *Blockchain) updateMainForkMap(tail *Block) error {
	bc.muForkLengths.Lock()
	defer bc.muForkLengths.Unlock()

	var err error
	if tail == nil {
		return errors.New("Tail is currently empty")
	}
	walk := tail

	bc.muMainforMap.Lock()
	defer bc.muMainforMap.Unlock()

	for {
		if bytes.Equal(bc.mainForkMap[walk.Height()], walk.Hash()) {
			break
		}
		bc.mainForkMap[walk.Height()] = walk.Hash()
		if walk.ParentHash() == nil {
			break
		}
		walk, err = bc.GetBlockByHash(walk.ParentHash())
		if err != nil {
			log.Info().Msgf("Could not find block.")
			return err
		}
	}
	return nil
}

// func (bc *Blockchain) Validate(blocks []*Block) error {    //TODO:- Bilal plz remove these ints and try to compare error to update caller func(hard to understand what these ints are meant for)
func (bc *Blockchain) Validate(block *Block) (error, int) {

	if !block.IsValid() {
		return fmt.Errorf("error - is invalid"), 2
	}
	if block.IsGenesis() && bc.IsEmpty() {
		//if it is the first block in blockchain
		return nil, 0
	} else if block.IsGenesis() && !bc.IsEmpty() && block.ParentHash() != nil {
		//If it is not the first block in the chain but is still genesis(happens if node is revolting)
		parentChainID, err := DefaultConfig.GetParentChainID()
		if err != nil {
			return err, 0
		}
		parentBlock, err := bc.GetBlockByHash(block.ParentHash())
		if err != nil {
			return err, 0
		}
		//Validating block signature with the parent chain-id
		if !parentBlock.HasValidSigWithID(parentChainID) {
			return fmt.Errorf("Wrong genesis block"), 0
		}

		return nil, 0
	}

	if bc.Contains(block) {
		return fmt.Errorf("error - already exists"), 3
	}

	parentBlock, err := bc.GetBlockByHash(block.ParentHash())

	if err != nil { //May not have parent in chain if bootstraping from epoch

		if bc.CanBootstrapFromEpoch(block) {
			return nil, 0 //WIll initiate bootstrap from epoch if parent doesn't exist
		}
		return err, 0
	}

	if err != nil || !parentBlock.IsValidParentOf(block) {

		return fmt.Errorf("error - failed to get valid parent block of incoming head %s %s", block, parentBlock), 1
	}
	if parentBlock.Height() != block.Height()-1 {
		return fmt.Errorf("Block heigth is not 1 more than parent height"), 0
	}

	return nil, 0
}

//CanBootstrapFromEpoch checks condition for bootstrapping from epoch
func (bc *Blockchain) CanBootstrapFromEpoch(b *Block) bool {
	//Check if block is digest block and its parent doesn't exist in checkByHash and checkByHeight
	if b.IsDigestBlock() {
		_, err := bc.GetBlockByHash(b.ParentHash())
		if err != nil {
			_, err := bc.GetBlockByHeight(b.Height() - 1)
			if err != nil {
				log.Info().Msgf("Should bootstrap from epoch --")
				return true
			}
		}
	}
	return false
}

//IsRevoltingTip checks if the mainTail of the chain is the revolting tip
func (bc *Blockchain) IsRevoltingTip() bool {

	b := bc.GetMainTail()
	if b == nil {
		log.Error().Msgf(fmt.Errorf("No block to check revolt tip").Error())
		return false
	}
	//Getting the parent chain-id from where revolt will branch off
	parentChainID, err := DefaultConfig.GetParentChainID()
	if err != nil {
		log.Error().Msgf(err.Error())
		return false
	}

	//Validating block signature with the parent chain-id
	if !b.HasValidSigWithID(parentChainID) {
		log.Error().Msgf(fmt.Errorf("Not a revoltTip- Coz: Tail's sign is not correct according to parent chain-id").Error())
		return false
	}

	if !bytes.Equal(b.Hash(), DefaultConfig.RevoltBlockHash) {
		log.Error().Msgf("Not a revoltTip- Coz: Maintail hash doesn't match with the config")
		return false
	}

	//Should it also match the maintail height with the config hight

	return true
}

//IsEmpty returns true if blockchain is empty. It doesn't have its first block
func (bc *Blockchain) IsEmpty() bool {
	if bc.genesis == nil {
		return true
	}
	return false
}

// Append appends blocks to the blockchain. The input blocks should be a contiguous chain of blocks sorted in ascending order by block height.
// func (bc *Blockchain) Append(blocks []*Block) error {
func (bc *Blockchain) Append(block *Block) error {

	if block == nil {
		return nil
	}

	if block.IsGenesis() && (block.ParentHash() == nil || bc.IsEmpty()) { //Only need to put_block. As per old protocol
		bc.genesis = block
		bc.tails = append(bc.tails, block)
		bc.mainTailIndex = 0
		return bc.putBlock(block)
	}

	headBlock := block
	tailBlock := block

	parentBlock, err := bc.GetBlockByHash(headBlock.ParentHash())
	if err != nil && bc.CanBootstrapFromEpoch(block) {
		err = nil
	}

	if err != nil {
		return err
	}

	if err = bc.putBlock(block); err != nil {
		return err
	}
	parentTailIndex := bc.indexInTails(parentBlock)
	if parentTailIndex == -1 && bc.CanBootstrapFromEpoch(block) {
		//Problemetic if there are forks. All epoch blocks will try to append to maintail
		parentTailIndex = bc.mainTailIndex //should append to main tail if epoch block parent doesn't exist
	}
	//Block height is already validated. 1 more than parent is ensured
	newForkLength := block.HeightInt()

	if parentTailIndex == -1 {
		bc.muTail.Lock()
		bc.tails = append(bc.tails, tailBlock)
		bc.muTail.Unlock()
		bc.muForkLengths.Lock()
		bc.forkLengths = append(bc.forkLengths, newForkLength)
		bc.muForkLengths.Unlock()
	} else {
		bc.muTail.Lock()
		bc.tails[parentTailIndex] = tailBlock
		bc.muTail.Unlock()
		bc.muForkLengths.Lock()
		bc.forkLengths[parentTailIndex] = newForkLength
		bc.muForkLengths.Unlock()
	}

	if block.IsGenesis() && block.ParentHash() != nil {
		fmt.Println("Resetting forklengths...") //Required when revolting
		bc.muForkLengths.Lock()
		for i := 0; i < len(bc.forkLengths); i++ {
			bc.forkLengths[i] = 0
		}
		bc.muForkLengths.Unlock()
	}

	return nil
}

// GetBlockByHash retrieves a block on the blockchain by block hash.
func (bc *Blockchain) GetBlockByHash(blockHash []byte) (*Block, error) {
	if bc.IsEmpty() {
		return nil, errors.New("BlockChain Not Initilized")
	}
	blockBytes, err := bc.db.Get(storage.BlockBucket, blockHash)
	if err != nil {
		return nil, err
	}
	block, err := NewBlockFromBytes(blockBytes)
	if err != nil {
		return nil, err
	}
	return block, nil
}

//GetBlockByHeight returns the block in the main fork for the given height
func (bc *Blockchain) GetBlockByHeight(height int64) (*Block, error) {
	var err error
	bc.muMainforMap.RLock()
	hash, ok := bc.mainForkMap[height]
	bc.muMainforMap.RUnlock()

	if !ok {
		return nil, errors.New(fmt.Sprintf("Block for height %v does not exist in main fork map", height))
	}

	block, err := bc.GetBlockByHash(hash)

	if err != nil {
		return nil, err
	}

	return block, nil

}

//BlockByHeight returns the parent of the main fork
func (bc *Blockchain) BlockHashByHeight(height int64) ([]byte, error) {
	bc.muMainforMap.RLock()
	defer bc.muMainforMap.RUnlock()

	if _, ok := bc.mainForkMap[height]; !ok {
		return []byte{}, errors.New("Block of height doesn't exist in Main Fork")
	}
	return bc.mainForkMap[height], nil
}

//BinaryDealSearch is a binary search lookup to find the deal in a block
func (bc *Blockchain) BinaryDealSearch(ts int64) (*pb.Deal, int64, error) {
	low := 0
	lastMid := -1
	// fmt.Println("bc.mainForkMap: ", bc.mainForkMap) //This is the height of the genesis block
	bc.muMainforMap.RLock()
	high := len(bc.mainForkMap) - 1 //This is the max height of the main tail
	bc.muMainforMap.RUnlock()

	for low <= high {
		median := low + (high-low)/2
		block, err := bc.GetBlockByHeight(int64(median))
		if err != nil {
			return nil, 0, err
		}

		bTs := block.GetTimestamp()

		targetDeal, err := CheckDealFromBlock(block, ts)

		if targetDeal != nil { //Case where deal was found in the block
			log.Info().Msgf("Ref Deal found!!!! %d", block.Height())
			return targetDeal, block.Height(), nil

		} else if bTs > ts { //When deal was not found and the timestamp of the block is smaller
			high = median - 1
		} else {
			low = median + 1 //Otherwise
		}

		if lastMid == median {
			break
		}
		lastMid = median //In case it runs into infinite loop
	}

	return nil, 0, nil

}

//LinearDealSearch search for the deal in blockchain linearly
func (bc *Blockchain) LinearDealSearch(ts int64) (*pb.Deal, int64, error) {

	tail := bc.GetMainTail()
	for tail != nil && tail.IsGenesis() != true && tail.GetTimestamp() >= ts {
		targetDeal, _ := CheckDealFromBlock(tail, ts)
		if targetDeal != nil {
			return targetDeal, tail.Height(), nil
		}
		if tail != nil {
			tail, _ = bc.GetBlockByHash(tail.ParentHash())
		} else {
			return nil, 0, nil
		}
	}

	return nil, 0, nil
}

//CheckDealFromBlock checks the deal which matches with the timestamp
func CheckDealFromBlock(b *Block, dts int64) (*pb.Deal, error) {
	if b != nil {
		deals := b.GetDeals()

		for _, deal := range deals {
			if deal.GetTimestamp() == dts {
				return deal, nil
			}
		}
	}
	return nil, errors.New("Deal not found in the block")
}

//FindLastVerification finds the similar last verification and returns the timestamp of the verification
func (bc *Blockchain) FindLastVerification(dealHeight int64, dealHash []byte) (int64, error) {

	tail := bc.GetMainTail()
	// Tail height -1 is because blocks are first added to the blockchain and then states are updated
	for i := tail.Height() - 1; i >= dealHeight; i-- { //Iterator shouldn't look beyond the deal height since verification happens after the deal
		block, err := bc.GetBlockByHeight(i)
		if err != nil {
			return 0, err
		}
		chPrPair := block.GetChPrPair()

		for _, proof := range chPrPair {
			res := bytes.Compare(proof.GetDealhash(), dealHash) //looking for the verification proof corrosponding to the deal
			if res == 0 {
				fmt.Println("Congrats. Last verification found!!!!!!!!!!!!!!") //Its a match
				return proof.Timestamp, nil
			}
		}
	}
	return 0, nil
}

// Contains returns true if the blockchain contains the block with hash blockHash.
func (bc *Blockchain) Contains(block *Block) bool {
	if block == nil || bc.db == nil {
		return false
	}
	_, err := bc.db.Get(storage.BlockBucket, block.Hash())
	return err == nil
}

// Genesis returns the blockchain's genesis block.
func (bc *Blockchain) Genesis() *Block {
	return bc.genesis
}

//ChainIDFromGenesis returns the chain-id from genesis
func (bc *Blockchain) ChainIDFromGenesis() (string, error) {

	g := bc.Genesis()
	if g != nil {
		return "", errors.New("Genesis doesn't exist")
	}
	return g.GetHeader().ChainId, nil

}

// GetMainTail returns the blockchain's main tail block.
func (bc *Blockchain) GetMainTail() *Block {

	if bc.mainTailIndex == -1 {
		log.Error().Msgf("Main Tail is not set")
		return nil
	}
	bc.muTail.RLock()
	defer bc.muTail.RUnlock()

	return bc.tails[bc.mainTailIndex]
}

// Length returns the length of the blockchain, i.e., the length of the blockchain's longest fork.
func (bc *Blockchain) Length() int {
	if bc.IsEmpty() {
		return 0
	}
	bc.muForkLengths.RLock()
	defer bc.muForkLengths.RUnlock()
	return bc.forkLengths[bc.mainTailIndex]
}

// NumForks returns the number of forks that the blockchain has.
func (bc *Blockchain) NumForks() int {
	bc.muTail.RLock()
	defer bc.muTail.RUnlock()
	return len(bc.tails)
}

func (bc *Blockchain) String() string {
	// TODO: use string builder instead.
	if bc != nil {
		s := fmt.Sprintf("Genesis: %s\n", bc.genesis)
		s += fmt.Sprintf("Tails:\n")
		for i, tail := range bc.tails {
			s += "\t"
			bc.muForkLengths.RLock()
			forkLength := bc.forkLengths[i]
			bc.muForkLengths.RUnlock()
			if i == bc.mainTailIndex {
				s += "MAIN "
			} else {
				s += "     " // for alignment (monospaced font only)
			}
			s += fmt.Sprintf("%d %s\n", forkLength, tail)
		}
		return s
	}

	s := "Block Chain not initilized!!"
	return s
}

// indexInTails returns the index of block in tails or -1 if block is not a tail block.
func (bc *Blockchain) indexInTails(block *Block) int {
	if block == nil {
		return -1
	}

	bc.muTail.RLock()
	t := bc.tails
	bc.muTail.RUnlock()

	for i, tail := range t {
		if block.IsSameAs(tail) {
			return i
		}
	}
	return -1
}

func (bc *Blockchain) findCommonAncestor(block *Block) (*Block, error) {
	var err error

	// bc.mu.RLock()
	mainTail := bc.GetMainTail()
	// bc.mu.RUnlock()

	//   				/->block
	// ->->->/->->->->->tail
	for block.Height() > mainTail.Height() { // Bring the two to the same height or the newtail is smaller
		block, err = bc.GetBlockByHash(block.ParentHash())
		if err != nil {
			return nil, err
		}
	}

	// Traversing back to see if the parents of two are the same, if same return
	for {
		mainChainBlock, err := bc.GetBlockByHeight(block.Height())
		if err != nil {
			log.Info().Msgf("Could not find the parent block on main chain")
			return nil, err
		}

		if bytes.Equal(mainChainBlock.Hash(), block.Hash()) {
			break
		}
		block, err = bc.GetBlockByHash(block.ParentHash())
		if err != nil {
			log.Info().Msgf("Could not find the parent block on the chain")
			return nil, err
		}
	}
	return block, nil

}

func (bc *Blockchain) Forked(newTail *Block, newTailIndex int) ([]*Block, []*Block, error) {
	var err error
	mailTail := bc.GetMainTail()

	commonParent, err := bc.findCommonAncestor(newTail) //Find common ancestor b/w newTail and oldMainTail
	if err != nil {
		log.Info().Msgf("Unable to find blocks to revert")
		return nil, nil, err
	}

	// These are in decreasing order which is correct in form to roll back
	// Block#17, Block#16 ...
	blocksToRollBack, err := bc.loadBlocksFromTo(mailTail, commonParent)
	if err != nil {
		return nil, nil, err
	}

	// These are in decreasing order, should be made ascending
	newerMailTailBlocks, err := bc.loadBlocksFromTo(newTail, commonParent)
	if err != nil {
		log.Info().Msgf("Unable to find blocks for new tail")
		return nil, nil, err
	}
	// Chain has moved to new main tail
	bc.setMainTailIndex(newTailIndex)

	log.Info().Msgf("Fork was successful")
	return blocksToRollBack, newerMailTailBlocks, nil
}

//FindLongestForkTail returns the tail of the longest fork
func (bc *Blockchain) FindLongestForkTail() *Block {

	longestTail := bc.GetMainTail()
	bc.muTail.RLock()
	t := bc.tails
	bc.muTail.RUnlock()

	for _, tail := range t {
		if tail.Height() > longestTail.Height() {
			longestTail = tail
		}
	}
	return longestTail
}

// updateMainTailIndex finds the longest fork and change the main tail if necessary.
func (bc *Blockchain) UpdateMainTailIndex() ([]*Block, []*Block, bool, error) {
	var err error
	var blocksToRollBack []*Block
	var blocksToProcess []*Block
	forkOccured := false
	bc.muTail.RLock()
	t := bc.tails
	bc.muTail.RUnlock()

	mainForkLength := bc.Length()
	for i, tail := range t {
		// fmt.Println("i: ", i, "tail: ", tail)
		bc.muForkLengths.RLock()
		forkLength := bc.forkLengths[i]
		bc.muForkLengths.RUnlock()

		if forkLength > mainForkLength {
			blocksToRollBack, blocksToProcess, err = bc.Forked(tail, i) // i is the new main tail to be
			if err != nil {
				log.Info().Msgf("Fork switch was not successful")
				return nil, nil, false, err
			}
			forkOccured = true
			mainForkLength = forkLength

		}
	}

	bc.updateMainForkMap(bc.GetMainTail())

	for _, tail := range t {
		if tail.Height() < bc.IrreversibleBlock {
			bc.DeleteFork(tail)
		}
	}

	return blocksToRollBack, blocksToProcess, forkOccured, nil
}

// putBlock puts a block in the blockchain database WITHOUT checking if block is valid.
func (bc *Blockchain) putBlock(block *Block) error {
	blockBytes, err := block.ToBytes()
	if err != nil {
		return err
	}
	return bc.db.Put(storage.BlockBucket, block.Hash(), blockBytes)
}

//ForceStoreBlock stores block in the db without any checks
func (bc *Blockchain) ForceStoreBlock(b *Block) error {
	return bc.putBlock(b)
}

// batchPutBlocks puts multiple blocks in the blockchain database WITHOUT checking if block is valid
func (bc *Blockchain) batchPutBlocks(blocks []*Block) error {
	blockHashList := make([][]byte, len(blocks))
	blockBytesList := make([][]byte, len(blocks))
	for i, block := range blocks {
		blockBytes, err := block.ToBytes()
		if err != nil {
			return err
		}

		blockHashList[i] = block.Hash()
		blockBytesList[i] = blockBytes
	}
	return bc.db.BatchPut(storage.BlockBucket, blockHashList, blockBytesList)
}

//GetMainForkHeight returns the height of the mainForkMap
func (bc *Blockchain) GetMainForkHeight() int {
	tail := bc.GetMainTail()
	if tail == nil {
		return 0 //This should be -1 coz genesis has 1 height
	}

	return tail.HeightInt() + 1 //+1 because genesis has a height of 0
}

// loadBlocksFromTo return list of blocks including the from(block)
func (bc *Blockchain) loadBlocksFromTo(from, to *Block) ([]*Block, error) {
	var err error
	var blocks []*Block

	if bytes.Equal(from.Hash(), to.Hash()) {
		return nil, nil
	}
	for !bytes.Equal(from.Hash(), to.Hash()) {
		blocks = append(blocks, from)
		from, err = bc.GetBlockByHash(from.ParentHash())
		if err != nil {
			log.Info().Msgf("Could not find parent block")
			return nil, err
		}
	}
	return blocks, nil
}

func (bc *Blockchain) PrintFromTail(tail *Block) {
	var err error
	log.Info().Msgf("================================")
	for {
		log.Info().Msgf("Height: %d  |  Hash %s", tail.Height(), hex.EncodeToString(tail.Hash()))
		tail, err = bc.GetBlockByHash(tail.ParentHash())
		if err != nil {
			break
		}
	}
	log.Info().Msgf("================================")

}

func (bc *Blockchain) PrintMainFork() {
	log.Info().Msg("================================")
	for k, v := range bc.mainForkMap {
		log.Info().Msgf("Height[%d] Hash[%s]", k, hex.EncodeToString(v))
	}
	log.Info().Msgf("================================")
}

func (bc *Blockchain) GetTails() []*Block {

	bc.muTail.RLock()
	t := bc.tails
	bc.muTail.RUnlock()

	blocks := make([]*Block, 0, len(t))
	for _, tail := range t {
		blocks = append(blocks, tail)
	}
	return blocks
}

func (bc *Blockchain) IsInMainFork(block *Block) bool {
	bc.muMainforMap.RLock()
	defer bc.muMainforMap.RUnlock()
	hash, ok := bc.mainForkMap[block.Height()]
	isHashSame := bytes.Equal(hash, block.Hash())
	return ok && isHashSame
}

func (bc *Blockchain) RemoveBlock(block *Block) error {

	// This should be uncommented. Reason being, if there is
	// a block added to the chain, the transactions should be validated first.
	// This function is to be only called when removing forks.
	if bc.IsInMainFork(block) {
		log.Error().Msgf("Cannot remove block on main tail")
		return errors.New("Cannot remove block on main tail")
	}

	parentBlock, err := bc.GetBlockByHash(block.ParentHash())
	if err != nil {
		return err
	}

	tIndex := bc.indexInTails(block)
	newForkLength := parentBlock.HeightInt()
	if tIndex == -1 {
		log.Error().Msgf("Cannot delete block that is not a tail")
		return errors.New("Cannot delete block that is not a tail")
	} else {

		if bc.IsInMainFork(parentBlock) { // Delete the other tail
			bc.muTail.Lock()
			bc.tails = removeNthTail(bc.tails, tIndex)
			bc.muTail.Unlock()
			bc.muForkLengths.Lock()
			bc.forkLengths = removeNthTailLength(bc.forkLengths, tIndex)
			bc.muForkLengths.Unlock()
			m := 0
			mi := 0
			for i, e := range bc.tails {
				if i == 0 || e.HeightInt() > m {
					m = e.HeightInt()
					mi = i
				}
			}
			bc.setMainTailIndex(mi)
		} else {
			bc.muTail.Lock()
			bc.tails[tIndex] = parentBlock
			bc.muTail.Unlock()
			bc.muForkLengths.Lock()
			bc.forkLengths[tIndex] = newForkLength
			bc.muForkLengths.Unlock()
		}

	}
	err = bc.db.Delete(storage.BlockBucket, block.Hash())
	if err != nil {
		return err
	}
	return nil
}

func (bc *Blockchain) DeleteFork(tail *Block) error {
	_, err := bc.findCommonAncestor(tail)
	if err != nil {
		log.Error().Msgf("No common ancestor found")
		return err
	}

	walk := tail
	for !bc.IsInMainFork(walk) {
		parentHash := walk.ParentHash()
		bc.RemoveBlock(walk)
		walk, err = bc.GetBlockByHash(parentHash)
		if err != nil {
			log.Error().Msgf("Cannot find parent")
			return err
		}
	}

	return nil
}

// Utility Function
func removeNthTail(t []*Block, i int) []*Block {

	return append(t[:i], t[i+1:]...)
}

func removeNthTailLength(s []int, i int) []int {

	return append(s[:i], s[i+1:]...)
}

func (bc *Blockchain) GetChainByTail(tail *Block) ([]*Block, error) {

	var err error
	block := bc.GetMainTail()
	chain := []*Block{tail}

	for {
		if block.IsGenesis() {
			break
		}
		block, err = bc.GetBlockByHash(block.ParentHash())
		if err != nil {
			return nil, err
		}
		chain = append(chain, block)
	}

	// Reverse the order of chain slice.
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}

//BinaryRoundSearch seaerch the blocks by round and return the first block found by height
func (bc *Blockchain) BinaryRoundSearch(targetRound int64) (int64, error) {
	low := 0
	lastMid := -1

	high := len(bc.mainForkMap) //This is the max height of the main tail
	for low <= high {
		median := low + (high-low)/2
		block, err := bc.GetBlockByHeight(int64(median))
		if err != nil {
			return 0, err
		}

		bRound := block.GetRoundNum()

		if bRound == targetRound { //Case where deal was found in the block
			log.Info().Msgf("Target block found!!!! %d", block.Height())
			return block.Height(), nil

		} else if bRound > targetRound { //When deal was not found and the timestamp of the block is smaller
			high = median - 1
		} else {
			low = median + 1 //Otherwise
		}

		if lastMid == median {
			break
		}
		lastMid = median //In case it runs into infinite loop
	}

	return 0, nil
}

//GetLastMiner returns the last miner by height of the block
func (bc *Blockchain) GetLastMiner(refH, refRound int64, refPk string) (*Block, string, error) {

	for i := refH - 1; i >= 0; i-- {
		// fmt.Println("I is", i)
		b, err := bc.GetBlockByHeight(i)
		if err != nil {
			return nil, "", err
		}
		bMiner := b.MinerPublicKey().String()
		if bMiner != refPk || refRound != b.GetRoundNum() {
			return b, bMiner, nil
		}
	}
	return nil, "", nil //Against the first miner
}

//GetFirstBlockOfMiner returns the first block of the miner
func (bc *Blockchain) GetFirstBlockOfMiner(refB *Block, refH int64, refPk string) (*Block, error) {

	for i := refH - 1; i >= 0; i-- {
		b, err := bc.GetBlockByHeight(i)
		if err != nil {
			return nil, err
		}
		bMiner := b.MinerPublicKey().String()
		if bMiner != refPk || refB.GetRoundNum() != b.GetRoundNum() {
			if i == refH-1 {
				return refB, nil
			}
			reqBlock, err := bc.GetBlockByHeight(i + 1)
			if err != nil {
				return nil, err
			}
			return reqBlock, nil
		}
	}
	return nil, errors.New("Not found")
}

//CheckLastMinerBlocks checks if last miner has produced all the blocks
func (bc *Blockchain) CheckLastMinerBlocks(curMiner string, maxBlockPerMiner int64) error {
	var err error
	tail := bc.GetMainTail()
	if tail == nil || tail.GetRoundNum() == 1 { //Exempt the first round
		return nil
	}
	for tail.MinerPublicKey().String() != curMiner {
		parent := tail.ParentHash()
		tail, err = bc.GetBlockByHash(parent)
		if err != nil {
			return err
		}
	}
	//Got the block with the minerPK != currentMiner
	lastMBheight := tail.Height() - maxBlockPerMiner - 1
	firstMinerB, err := bc.GetBlockByHeight(lastMBheight)
	if err != nil {
		return err
	}
	if tail.MinerPublicKey().String() == firstMinerB.MinerPublicKey().String() {
		return nil
	}
	return errors.New("Yet to receive blocks of last miner")
}

func (bc *Blockchain) GetLastNBlocks(n int) []*pb.Block {

	nBlocks := make([]*pb.Block, 0)
	tailHeight := bc.GetMainTail().Height()

	for i := n; i > 0; i-- {
		val, ok := bc.mainForkMap[tailHeight]
		if !ok {
			log.Error().Msgf("Value for height does not exist in main fork map")
			break
		} else {
			b, err := bc.GetBlockByHash(val)
			if err != nil {
				log.Error().Err(err)
			}
			nBlocks = append(nBlocks, b.PbBlock)

		}

		tailHeight--
	}

	return nBlocks
}

func (bc *Blockchain) UpdateIrreversibleBlockNumber(minerAdd string, ComitteeSize uint32, NumberOfToMineBlockPerSlot int, port uint16) {

	size2by3 := int((2 * ComitteeSize) / 3)
	log.Info().Msgf("Miner - %v : bc.ComitteeSize ComitteeSize %v size2by3 %v", port, ComitteeSize, size2by3)

	if bc.Last2by3MinedCommitteeMembers.Len() == 0 || bc.Last2by3MinedCommitteeMembers.Len() < size2by3*NumberOfToMineBlockPerSlot {
		bc.Last2by3MinedCommitteeMembers.PushBack(minerAdd)
		return
	}

	bc.Last2by3MinedCommitteeMembers.Remove(bc.Last2by3MinedCommitteeMembers.Front())
	bc.Last2by3MinedCommitteeMembers.PushBack(minerAdd)

	keys := make(map[string]bool)
	list := []string{}
	for e := bc.Last2by3MinedCommitteeMembers.Front(); e != nil; e = e.Next() {
		entry := e.Value.(string)
		// log.Info().Msgf("Miner - %v : bc.IrreversibleBlock entry: %v", port, entry)
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}

	if len(list) >= size2by3 {
		go func(bc *Blockchain, blockNumber int64) {
			block, err := bc.GetBlockByHeight(blockNumber)
			if err != nil {
				panic(fmt.Sprintf("Miner - %v UpdateIrreversibleBlockNumber err %v", port, err))
			}
			for _, cProof := range block.GetChPrPair() {

				curTs := time.Unix(cProof.Timestamp, 0)
				dealExpTs := time.Unix(cProof.DealExpiryTime, 0)
				if curTs.After(dealExpTs) {
					log.Info().Msgf("UpdateIrreversibleBlockNumber deal delete from DB %v", hex.EncodeToString(cProof.Dealhash))
					bc.db.Delete(storage.DealBucket, cProof.Dealhash)
					bc.db.Delete(storage.LastChVerificationTimeBucket, cProof.Dealhash)
				}
			}

			for _, deal := range block.GetDeals() {
				for _, infopr := range deal.List {
					hashbytes, _ := proto.Marshal(infopr.Info)
					hash := crypto.Sha256(hashbytes)
					bc.db.Delete(storage.PostStoredBucket, hash)
				}
			}

		}(bc, bc.IrreversibleBlock)
		bc.IrreversibleBlock++
	}

	log.Info().Msgf("Miner - %v : bc.IrreversibleBlock UPdated: %v size2by3 %v bc.GetMainForkHeight() %v diff %v", port, bc.IrreversibleBlock, size2by3, bc.GetMainForkHeight(), bc.GetMainForkHeight()-int(bc.IrreversibleBlock))
}

func (bc *Blockchain) UpdateIrreversibleBlockNumberWhileReverting(minerAdd string, port uint16) {

	lastMinerAddress := bc.Last2by3MinedCommitteeMembers.Back()
	if lastMinerAddress != nil {
		bc.Last2by3MinedCommitteeMembers.Remove(lastMinerAddress)
		if lastMinerAddress.Value.(string) != minerAdd {
			log.Error().Msgf("miner - %v: UpdateIrreversibleBlockNumberWhileReverting Invalid Miner %v got deleted in Bc.Last2by3MinedCommitteeMembers insted of %v ", port, lastMinerAddress.Value.(string), minerAdd)
		}
	} else {
		log.Warn().Msgf("bc.IrreversibleBlock Warning Network Parittion !!!! ")
		bc.IrreversibleBlock -= 1
	}

}

func (bc *Blockchain) GetLastMinedCommitteeMemberLength() int {
	return bc.Last2by3MinedCommitteeMembers.Len()
}

func (bc *Blockchain) AddMinerToIrreversibleBlockNumberCacheWhileReverting(minerAdd string, ComitteeSize uint32, NumberOfToMineBlockPerSlot int) {

	size2by3 := int((2 * ComitteeSize) / 3)
	if !(bc.Last2by3MinedCommitteeMembers.Len() == 0 || bc.Last2by3MinedCommitteeMembers.Len() < size2by3*NumberOfToMineBlockPerSlot) {
		bc.Last2by3MinedCommitteeMembers.Remove(bc.Last2by3MinedCommitteeMembers.Back())

	}

	bc.Last2by3MinedCommitteeMembers.PushFront(minerAdd)
	keys := make(map[string]bool)
	list := []string{}
	for e := bc.Last2by3MinedCommitteeMembers.Front(); e != nil; e = e.Next() {
		entry := e.Value.(string)
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	if len(list) >= size2by3 {
		bc.IrreversibleBlock--
	}
	log.Info().Msgf("AddMinerToIrreversibleBlockNumberCacheWhileReverting bc.IrreversibleBlock revert UPdated: %v", bc.IrreversibleBlock)
}
