package roles

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/guyu96/go-timbre/core"
	"github.com/guyu96/go-timbre/dpos"
	"github.com/guyu96/go-timbre/log"
	"github.com/guyu96/go-timbre/net"
	"github.com/mbilal92/noise"
)

const (
	syncerChanSize                    = 64 // default syncer channel buffer size
	noOfNodesToQuery                  = 3  // Number of nodes to ask for the blocks
	maxResponseTime                   = 5  // Time limit for a response in seconds
	RequestCurrentTailMaxResponseTime = 5  // Time limit for a response in seconds
)

var (
	testNo       = 10
	errDposIsOff = errors.New("Cannot validate blocks without running DPos. RUN DPOS plz")

	GenesisBlocktoSetWhileSync *core.Block
	ChangeGenesisWhileSyncing  bool
	errFailedValidation        = errors.New("Failed block validation")
	BootstrapFromEpoch         = true //BootstrapFromEpoch is a flag to allow syncs from epoch
)

// Syncer is responsible for synchronizing the blockchain.
type Syncer struct {
	node             *net.Node
	incoming         chan net.Message
	download         chan net.Message
	targetBlock      *core.Block
	curBlock         *core.Block //[]byte
	mu               sync.RWMutex
	isSyncing        bool
	chainSwitch      bool
	Running          bool
	InvalidBlockpool *core.Blockpool
	reSync           bool
}

// NewSyncer creates a new syncer.
func NewSyncer(node *net.Node) *Syncer {
	bp, _ := core.NewBlockpool(core.DefaultBlockpoolCapacity)
	syncer := &Syncer{
		node:             node,
		incoming:         make(chan net.Message, syncerChanSize),
		mu:               sync.RWMutex{},
		isSyncing:        false,
		chainSwitch:      false,
		Running:          false,
		InvalidBlockpool: bp,
		reSync:           false,
	}
	node.AddOutgoingChan(syncer.incoming)
	return syncer
}

//GetIsSyncing returns the syncing bool state
func (syncer *Syncer) GetIsSyncing() bool {
	return syncer.isSyncing
}

//SetIsSyncing sets the isSyncing bool
func (syncer *Syncer) SetIsSyncing(flag bool) {
	syncer.isSyncing = flag
}

// Process starts processing incoming messages for syncer.
func (syncer *Syncer) Process() {
	syncer.Running = true
	for msg := range syncer.incoming {
		switch msg.Code {
		case net.MsgCodeBlockNew:
			block, err := getBlockFromMsg(msg)
			if err == nil {
				go syncer.handleBlockNew(block)
			} else {
				log.Error().Msgf("Could not read block from message")
			}
		case net.TestMsg:
			log.Info().Msgf("resceived in the testmsg in syncer %v", msg.Data) //
		case net.MsgCodeBlockAsk:
			go syncer.handleBlockAsk(msg)
		case net.MsgCodeBlockAskByHeight:
			go syncer.handleBlockByAskByHeight(msg)
		case net.MsgCodeBlockSyncByHeight:
			go syncer.handleBlockSyncByHeight(msg)
		case net.MsgCodeGetCurrentTail:
			go syncer.handleCurrentTailReq(msg)
		case net.MsgCodeCurrentTail:
			go syncer.handleWriteToDownload(msg)
		case net.MsgCodeBlockChainLongSyncResquest:
			go syncer.handlBlockChainLongSyncRequest(msg)
		case net.MsgCodeBlockChainLongSyncResponse:
			syncer.handlBlockChainLongSyncResponse(msg)
		case net.MsgCodeAskBlockByHash:
			go syncer.handleAskBlockByHash(msg)

		}

	}
	syncer.Running = false
}

func getBlockFromMsg(msg net.Message) (*core.Block, error) {
	return core.NewBlockFromBytes(msg.Data)
}

//handleBlockNew handles the newly broadcasted vote
func (syncer *Syncer) handleBlockNew(block *core.Block) {

	syncer.mu.Lock()
	defer syncer.mu.Unlock()

	isValid := block.IsValid()
	if isValid != true {
		log.Error().Msgf("Block is invalid")
		return
	}

	log.Info().Msgf("Received %s ", block)

	minerPk := block.MinerPublicKey()
	minerAdd := minerPk.String()

	//handling the bug in which block is added to the chain twice
	if minerAdd == syncer.node.Dpos.GetCurrentMinerAdd() && minerAdd == syncer.node.PublicKey().String() { //Current miner shouldn't receive its own block in syncer
		log.Info().Msgf("Received my own block! Not adding")
		return
	}

	node := syncer.node

	if node.Bc == nil {
		//Bc will not be nil because it is initialized in NewNode
		panic("Bc is nil")

	} else {

		if !node.Bc.Contains(block) && !node.Bp.Contains(block) {

			node.Bp.Add(block) // Add to blockpool and then check blockpool for tails that can be added

			if syncer.isSyncing && syncer.targetBlock != nil && block.Height() > syncer.targetBlock.Height() {
				fmt.Println("Got New block while syncing")
				return
			}
			tails := syncer.node.Bp.GetTails()

			// TODO: Optimize blockpool so that 1) it dynamically maintains a tail slice  and 2) When a new block is added, mark its chain as modified to avoid going through every possible chain.
			for _, tail := range tails {
				chain := syncer.node.Bp.GetChainByTail(tail)
				if chain[0].Height() < syncer.node.Bc.IrreversibleBlock { // remove chain
					fmt.Println("removing Chain...", syncer.node.Bc.IrreversibleBlock)
					for _, b := range chain {
						syncer.node.Bp.Remove(b.Hash())
					}
				} else {
					log.Info().Msgf("Applying iterate check and add chain...")
					syncer.IterateCheckAndAddChain(chain)
				}
			}

			InvalidTails := syncer.InvalidBlockpool.GetTails()
			for _, tail := range InvalidTails {
				chain := syncer.InvalidBlockpool.GetChainByTail(tail)
				fmt.Println("Invalid Chain to Remove %v for tail ", chain, tail)
				if chain[0].Height() < syncer.node.Bc.IrreversibleBlock { // remove chain
					fmt.Println("removing Chain...", syncer.node.Bc.IrreversibleBlock)
					for _, b := range chain {
						syncer.InvalidBlockpool.Remove(b.Hash())
					}
				}
			}

			log.Error().Msgf("Bp Size %d", syncer.node.Bp.Size())

			//Conditioning is not right. Flags are not set. And syning cannot be started without shutting off the DPOS.
			size2by3 := int((2 * syncer.node.Dpos.GetComitteeSize()) / 3)
			if syncer.node.Bp.Size() > size2by3 && !syncer.isSyncing {
				log.Error().Msgf("Asking for tail+1 block because Bp has more than 5 blocks. Tail: %v", syncer.node.Bc.GetMainTail().String())

				go syncer.SyncBlockchain()

			} else {
				if syncer.isSyncing == false {
					syncer.node.Bp.PrintBlockPool()
				}
			}

		} else {

			log.Info().Msgf("Block Already exists in chain or in BlockPool. Block:%v", block)
		}
	}
}

//SendTestMsg sends the test transaction
func (syncer *Syncer) SendTestMsg() {

	bs := []byte(strconv.Itoa(testNo))
	testNo++
	log.Info().Msgf("Came in testMsg")
	msg := net.NewMessage(net.TestMsg, bs)
	syncer.node.Broadcast(msg)
	if testNo == 15 {
		testNo = 10
	}
	log.Info().Msgf("Test msg is sent")
}

func (syncer *Syncer) clearDownloadVars() {
	syncer.targetBlock = nil
	syncer.curBlock = nil
}

func (syncer *Syncer) handleWriteToDownload(msg net.Message) {
	syncer.download <- msg
}

//Setup update the bc by local Syncing
func (syncer *Syncer) Setup() {
	if syncer.node.Bc.IsEmpty() != true { //Basically if blockchain is not nil
		log.Info().Msgf("Applying local blocks already in the chain")

		length := int64(syncer.node.Bc.Length())
		fmt.Println("bc length: ", length, " ", syncer.node.Bc.GetMainTail().Height())

		for i := int64(0); i <= length; i++ {
			block, _ := syncer.node.Bc.GetBlockByHeight(i)
			fmt.Println("bround: bHeight: ", block.GetRoundNum(), block.Height())
			err := syncer.node.Services.UpdateDpos(block, syncer.node.Dpos)
			if err != nil {
				log.Info().Err(err)
			}
			if block.IsGenesis() {
				syncer.node.Wm.ApplyGenesis(block)
			} else {
				syncer.node.Wm.CheckAndUpdateWalletStates(block, block.MinerPublicKey().String())
			}

		}

	} else {
		log.Info().Msgf("Blockchain has not been initiated yet.")
	}
}

// RequestCurrentTail locally broadcasts and waits for response to update targetBlock
func (syncer *Syncer) RequestCurrentTail() error {

	msg := net.NewMessage(net.MsgCodeGetCurrentTail, nil)
	syncer.download = make(chan net.Message, 32)

	for _, item := range syncer.chooseNPeers(noOfNodesToQuery) {
		syncer.node.Relay(item, msg)
	}

	shouldRespondIn := RequestCurrentTailMaxResponseTime * time.Second // Maximum time limit to respond
	ctx, cancel := context.WithTimeout(context.Background(), shouldRespondIn)
	defer cancel()

	func() {
		for i := 0; i < noOfNodesToQuery; i++ {
			select {
			case <-ctx.Done():
				return
			case msg := <-syncer.download:
				err := syncer.handleCurrentTailRes(msg)
				if err != nil {
					log.Error().Err(err).Msgf("Error finding the tail")
				}
			}
		}
	}()
	if syncer.targetBlock == nil {
		return errors.New("Could not find tail block")
	}
	return nil
}

func (syncer *Syncer) handleCurrentTailRes(msg net.Message) error {

	block, err := core.NewBlockFromBytes(msg.Data)
	if err != nil || len(msg.Data) == 0 {
		return errors.New(fmt.Sprintf("Could not read sent tail Error %v || msg.Data length %v", err, len(msg.Data)))
	}

	log.Info().Msgf("handleCurrentTailRes request from %v, block %v", msg.From.String(), block)

	if syncer.targetBlock == nil {
		log.Info().Msgf("Target block set to %s", block.String())
		syncer.targetBlock = block
	} else {
		if block.Height() > syncer.targetBlock.Height() {
			log.Info().Msgf("Target block set to %s", block.String())
			syncer.targetBlock = block
		} else {
			log.Info().Msgf("Target block is already set.")
		}
	}
	return nil
}

// handleCurrentTailReq sends current tail block if exists otherwise nil
func (syncer *Syncer) handleCurrentTailReq(msg net.Message) {

	var blockBytes []byte
	var err error
	if syncer.node.Bc.Genesis() != nil {
		blockBytes, err = syncer.node.Bc.GetMainTail().ToBytes()
		if err != nil {
			log.Error().Msgf("Could not read main tail")
			return
		}
	}
	res := net.NewMessage(net.MsgCodeCurrentTail, blockBytes)
	syncer.node.Relay(msg.From, res)
}

func (syncer *Syncer) AskByHeight(fromBlockHeight int) {

	msg := net.NewMessage(net.MsgCodeBlockAskByHeight, []byte(strconv.Itoa(fromBlockHeight)))
	for _, item := range syncer.chooseNPeers(noOfNodesToQuery) {
		syncer.node.Relay(item, msg)
	}
}

//AskBlockByHash send an ask of block by hash
func (syncer *Syncer) AskBlockByHash(bHash []byte) {
	msg := net.NewMessage(net.MsgCodeAskBlockByHash, bHash)
	for _, rNode := range syncer.chooseNPeers(noOfNodesToQuery) {
		syncer.node.Relay(rNode, msg)
	}
}

func (syncer *Syncer) createAskMsg(blockHash []byte, n int) net.Message {
	var hashAndZ []byte
	hashAndZ = append(append(append(hashAndZ, blockHash...), []byte(" , ")...), []byte(strconv.Itoa(n))...)
	msg := net.NewMessage(net.MsgCodeBlockAsk, hashAndZ)
	return msg
}

// ChooseNPeers returns n randomly selected peers from the peers list
func (syncer *Syncer) chooseNPeers(n int) []noise.ID {
	rand.Seed(time.Now().UnixNano())
	rl := syncer.node.GetPeerAddress()
	rand.Shuffle(len(rl), func(i, j int) { rl[i], rl[j] = rl[j], rl[i] })

	if n > syncer.node.NumPeers() {
		return rl
	}
	return rl[:n]
}

//ValidateForMainTail validates the main-tail block
func (syncer *Syncer) ValidateForMainTail(b *core.Block) error {
	bootstrapFromEpochReq := false //Bootstrap from epoch is required if true

	if syncer.isSyncing == true && syncer.node.Dpos.HasStarted == false { //while node is downloading old blocks already produced
		fmt.Println("Updating Dpos call from syncer...")
		err := syncer.node.Services.UpdateDpos(b, syncer.node.Dpos)
		if err != nil {
			log.Info().Err(err)
			if syncer.node.Bc.CanBootstrapFromEpoch(b) { //If CanBootstrapFromEpoch is true than dpos will be reset any way. Error doesn't matter
				bootstrapFromEpochReq = true //Set to true if there is error
			} else {
				return err
			}
		}
	} else if syncer.isSyncing == false && syncer.node.Dpos.HasStarted == false {
		log.Warn().Msgf("Cannot validate blocks without running DPos. RUN DPOS plz")
		return errDposIsOff
	}

	//Validating for the digest block
	bcHeight := int64(0)
	if syncer.node.Bc.CanBootstrapFromEpoch(b) && syncer.isSyncing {
		bcHeight = b.Height() //If there is no parent then height is equal to this block's height
	} else {
		bcHeight = int64(syncer.node.Bc.GetMainForkHeight())
	}
	if bcHeight%core.TestEpoch.Duration == 0 && bcHeight != 0 { //If state is appended in the block then it is the digest block

		curEpochNum := float64(bcHeight) / float64(core.TestEpoch.Duration)
		// fmt.Println("cur e[och: ", curEpochNum, syncer.node.GetCurEpoch())

		if b.IsDigestBlock() == true {
			fmt.Println("THis is digest block...")

			if syncer.isSyncing && syncer.node.Bc.CanBootstrapFromEpoch(b) {
				//Bootstrap from epoch is only fired if a node is syncing and doesn't have a parent at its height-1
				err := syncer.node.BootstrapFromEpochBlock(b) //Testing bootstrapping from the epoch block
				if err != nil {
					panic(err.Error())
				}
				bootstrapFromEpochReq = false //Bootstrap done sp required bootstrap is set to false
				log.Info().Msgf("Bootstrap done")
			}

			err := syncer.node.Wm.ValidateDigestBlock(b)
			if err != nil {
				return err
			}
			err = syncer.node.Dpos.ValidateDigestMiners(b.PbBlock.GetState().GetMiners(), b.PbBlock.State.NextRoundMiners)
			if err != nil {
				return err
			}
		} else {
			return errors.New("Was expecting Digest block")
		}

		if int32(curEpochNum) != b.GetEpochNum() {
			return errors.New("Epoch numbs in the block doesn't match")
		}

	}

	if bootstrapFromEpochReq { //Bootstrap should have already happened. May
		panic("Bootstrap from epoch was required but wasn't done")
	}

	if syncer.isSyncing && syncer.node.Bc.CanBootstrapFromEpoch(b) {
		log.Info().Msgf("Not validating Miner slot")
	} else {
		errT := syncer.ValidateMinerSlot(b, syncer.node.Dpos)
		if errT != nil {
			log.Info().Err(errT)
			fmt.Println(errT.Error())
			return errT
		}
	}

	err2 := syncer.node.Wm.CheckAndUpdateWalletStates(b, b.MinerPublicKey().String())
	if err2 != nil {
		fmt.Println("Error updating the states: ", err2.Error())
		log.Info().Err(err2)
		return err2
	}

	return nil
}

//IterateCheckAndAddChain ...
func (syncer *Syncer) IterateCheckAndAddChain(chain []*core.Block) bool {
	syncer.node.BcMtx.Lock()
	defer syncer.node.BcMtx.Unlock()
	for _, b := range chain {
		log.Info().Msgf("IterateCheckAndAddChain processing %v", b)

		err, errNo := syncer.node.Bc.Validate(b)
		if err != nil {
			log.Error().Msgf("Block Not Added in BC, Validation Failed (IterateCheckAndAddChain) %s %v", b, err)
			if errNo == 3 {
				syncer.node.Bp.Remove(b.Hash())
			}
			if errNo == 2 {
				syncer.InvalidBlockpool.Add(b)
				syncer.node.Bp.Remove(b.Hash())
			}
			return false
		}

		if syncer.InvalidBlockpool.ContainsBlockByHash(b.ParentHash()) {
			fmt.Println("Invalid block - Added to InvalidBlockpool ", b)
			syncer.InvalidBlockpool.Add(b)
			syncer.node.Bp.Remove(b.Hash())
			return false
		}

		inMainTail := b.IsGenesis() || syncer.node.Bc.CanBootstrapFromEpoch(b) || bytes.Equal(syncer.node.Bc.GetMainTail().Hash(), b.ParentHash())

		if inMainTail {
			err := syncer.ValidateForMainTail(b)
			if err != nil {

				if syncer.isSyncing == true && syncer.node.Dpos.HasStarted == false { //while node is downloading old blocks already produced
					syncer.node.Dpos.RevertDposTill(syncer.node.Bc.GetMainTail().GetRoundNum())
				}

				if err == errDposIsOff { //Don't remove block from pool in that case
					return false
				} else if err.Error() == "Block is from future" {
					//Do something
					return false
				}

				log.Info().Msgf("Err is: %v", err.Error())
				log.Info().Msgf("Invalid block removing from bp")
				syncer.InvalidBlockpool.Add(b)
				syncer.node.Bp.Remove(b.Hash())
				return false
			}
		} else {

			log.Info().Msgf("Initiate snapshot protocol")
			parent, err := syncer.node.Bc.GetBlockByHash(b.ParentHash())
			if err != nil {
				log.Error().Msgf(err.Error())
				return false
			}
			parentState, err := syncer.GetStateForBlockValidation(parent) //Fetching the parent state to validate the block
			if err != nil {
				fmt.Println("error is: ", err.Error())
				return false
			}
			err = syncer.ValidateForNonMainTail(b, parentState.CDpos, parentState.CWallets)
			if err != nil {
				fmt.Println("error in non-main validation: ", err.Error())
				syncer.InvalidBlockpool.Add(b)
				syncer.node.Bp.Remove(b.Hash())
				return false
			}

			fmt.Println("Non-main Validation SUCCESSFULL...")

			parentState.UpdateStateComp(b.Hash(), b.Height())
			syncer.node.AddDeals(b.GetDeals(), false) //will add deals in the db
		}

		// err = syncer.node.Bc.Append([]*core.Block{b})
		err = syncer.node.Bc.Append(b)
		if err != nil { //Append unsuccessful
			if syncer.isSyncing == true && syncer.node.Dpos.HasStarted == false { //while node is downloading old blocks already produced
				syncer.node.Dpos.RevertDposTill(syncer.node.Bc.GetMainTail().GetRoundNum())
			}
			fmt.Println("Invalid block parent hahs ", hex.EncodeToString(b.ParentHash()))
			// TODO : rething about this for now A hack
			if syncer.InvalidBlockpool.ContainsBlockByHash(b.ParentHash()) {
				fmt.Println("Invalid block Added to InvalidBlockpool ", hex.EncodeToString(b.Hash()))
				syncer.InvalidBlockpool.Add(b)
				syncer.node.Bp.Remove(b.Hash())
			}
			log.Info().Err(err)
		} else { // Append successful
			blocksToRevert, blocksToProcess, _, err := syncer.node.Bc.UpdateMainTailIndex()

			if err != nil {
				log.Error().Err(err)
			}

			if inMainTail {
				if len(blocksToProcess) == 0 { //If blocks is from past apply this and restart Dpos to compute committee accordingly
					// blocksToRevert, blocksToProcess = syncer.CheckDposRounds(b)
					syncer.CheckDposRounds(b) // we don't need to stop and start dpos for delayed accross round block
				}
			}

			if len(blocksToProcess) > 0 { // Implying there is a fork
				if len(blocksToRevert) > 0 && len(blocksToProcess) > 1 {
					log.Info().Msgf("A fork occured. Rollback/Process blocks here !!!")
				}

				err = syncer.SwitchFork(blocksToRevert, blocksToProcess)
				if err != nil { //shouldn't give an error coz states are already validated
					return false
				}

			} else { // Same main fork extended or added to a smaller fork
				if syncer.node.Bc.IsInMainFork(b) {
					syncer.UpdateStates(b)
					if syncer.isSyncing && syncer.curBlock.Height() < b.Height() {
						syncer.curBlock = b //.Hash()
					}
					log.Info().Msgf("Block Added in BC (IterateCheckAndAddChain) %s", b)
				} else {
					log.Info().Msgf("Block Added in BC But not in Main Tail (IterateCheckAndAddChain) %s", b)
					go func() {
						syncer.CheckAndReportMisbehaviour(b)
					}()

				}
				log.Info().Msgf("New Blockchain: %s", syncer.node.Bc.String())
			}

			syncer.node.Bp.Remove(b.Hash())
		}
	}

	return true
}

//CheckAndReportMisbehaviour checks if miner has misbehaved and create automatic podf
func (syncer *Syncer) CheckAndReportMisbehaviour(b *core.Block) {

	mainForkBlock, _ := syncer.node.Bc.GetBlockByHeight(b.Height())

	if mainForkBlock != nil { //Same height block
		if bytes.Equal(mainForkBlock.PublickeyBytes(), b.PublickeyBytes()) { //Same public key
			//Creating a podf transaction
			fmt.Println("Found Double forged block. Reporting...")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			syncer.node.Services.CreatePodf(ctx, mainForkBlock.GetHeader(), b.GetHeader())
		}

	}
}

//CheckDposRounds validates if block rounds are the same as the DPos
func (syncer *Syncer) CheckDposRounds(b *core.Block) ([]*core.Block, []*core.Block) {
	//This block is child of the main tail which is in another round

	if b.GetRoundNum() != syncer.node.Dpos.GetRoundNum() {

		if syncer.node.Dpos.GetRoundNum() > b.GetRoundNum() {

			if syncer.node.Dpos.GetRoundNum() == b.GetRoundNum()+1 {
				log.Warn().Msgf("Got block from last round!!!!. Will simply add now. Recomputing committee from last round")
				syncer.node.Dpos.RecomputeLastRoundCommittee() //will recompute the committee
				return []*core.Block{}, []*core.Block{}
			} else {
				panic("Block is older than last round. Check this")
			}
		} else {
			if syncer.node.Dpos.GetRoundNum() == 0 {
				panic("Restart plz. I started in different slot then my fellows :)")
			}
			panic("Block is from future. Find a way to handle this")
		}
	}

	return []*core.Block{}, []*core.Block{}
}

//SwitchFork switches to the fork given in blocks to process. Alos stops and restarts the DPos
func (syncer *Syncer) SwitchFork(blocksToRevert, blocksToProcess []*core.Block) error {
	log.Info().Msgf("Stopping the DPOS... %v", syncer.node.GetNodeID().Address)
	wasRunning := false //For DPos. DPos should only be restarted in case it was running beforehand

	if syncer.node.Dpos.HasStarted == true {
		wasRunning = true
	}

	syncer.node.Dpos.Stop()
	if !syncer.isSyncing { //Check this condition
		syncer.isSyncing = true
		defer func(syncer *Syncer) {
			syncer.isSyncing = false
		}(syncer)
	}

	for _, revertBlock := range blocksToRevert {
		syncer.RevertState(revertBlock)
	}

	lastblockToRevert := blocksToRevert[len(blocksToRevert)-1]
	goBackblock := syncer.node.Bc.GetLastMinedCommitteeMemberLength()
	for i := 0; i < goBackblock; i++ {
		lastblockToRevert, _ = syncer.node.Bc.GetBlockByHash(lastblockToRevert.ParentHash())
		if lastblockToRevert == nil {
			break
		}
	}

	counter := int((2*syncer.node.Dpos.GetComitteeSize())/3)*syncer.node.Dpos.NumberOfToMineBlockPerSlot - goBackblock
	log.Info().Msgf("counter : %v, goBackblock %v", counter, goBackblock)
	for i := 0; i < counter; i++ {
		lastblockToRevert, _ = syncer.node.Bc.GetBlockByHash(lastblockToRevert.ParentHash())
		log.Info().Msgf("lastblockToRevert %v", goBackblock)
		if lastblockToRevert == nil {
			break
		}
		minerAdd := lastblockToRevert.MinerPublicKey().String()
		syncer.node.Bc.AddMinerToIrreversibleBlockNumberCacheWhileReverting(minerAdd, syncer.node.Dpos.GetComitteeSize(), syncer.node.Dpos.NumberOfToMineBlockPerSlot)
	}

	last := len(blocksToProcess) - 1
	fmt.Println("Applying blocks")
	for i := range blocksToProcess { // To run in ascending order
		err := syncer.ValidateForMainTail(blocksToProcess[last-i])
		if err != nil { // TODO: Rever all blocks to previos chain"
			return errFailedValidation
		}
		syncer.UpdateStates(blocksToProcess[last-i])
		syncer.node.Bp.Remove(blocksToProcess[last-i].Hash())
	}
	if wasRunning == true {
		log.Info().Msgf("Blocks reverted and applied")
		log.Info().Msgf("Starting DPOS again")
		err := syncer.node.Services.StartDpos()
		if err != nil {
			if err.Error() == "Stale Main-Tail" {
				syncer.isSyncing = false
				go syncer.SyncBlockchain()
			}
			return err
		}
	}

	return nil
}

//SwitchFork2 switches to the fork given in blocks to process. Alos stops and restarts the DPos
func (syncer *Syncer) SwitchFork2(blocksToRevert, blocksToProcess []*core.Block) error {
	log.Info().Msgf("Stopping the DPOS... %v", syncer.node.GetNodeID().Address)
	wasRunning := false //For DPos. DPos should only be restarted in case it was running beforehand

	if syncer.node.Dpos.HasStarted == true {
		wasRunning = true
	}

	syncer.node.Dpos.Stop()
	if !syncer.isSyncing { //Check this condition
		syncer.isSyncing = true
		defer func(syncer *Syncer) {
			syncer.isSyncing = false
		}(syncer)
	}

	//Checking for the cached state in the blockstate to directly inject changes in dpos and walletManager
	if len(blocksToProcess) > 0 {
		//checking the cached state
		state := syncer.CheckAndGetBlockState(blocksToProcess[0])
		if state != nil { //cached state exists
			// Directly inject component changes to dpos to dpos
			err := syncer.node.Wm.InjectWalletManager(state.CWallets)
			if err != nil { //TODO:- if err then changes need to be reverted
				return err
			}

			err = syncer.node.Dpos.InjectDposChanges(state.CDpos)
			if err != nil { //TODO:- if err then changes need to be reverted
				return err
			}
		} else {
			return errors.New("No state to directly switch. Follow revert protocol")
		}
		fmt.Println("INJECTION Successful")
	}

	if wasRunning == true {
		log.Info().Msgf("Changes directly injected")
		log.Info().Msgf("Starting DPOS again")
		err := syncer.node.Services.StartDpos()
		if err != nil {
			if err.Error() == "Stale Main-Tail" {
				syncer.isSyncing = false
				go syncer.SyncBlockchain()
			}
			return err
		}
	}

	return nil
}

//UpdateStates updates all the state associated with the node
func (syncer *Syncer) UpdateStates(b *core.Block) bool {

	log.Info().Msgf("Updating States for %v\n", b)
	minerPk := b.MinerPublicKey()
	minerAdd := minerPk.String()

	syncer.node.AddDeals(b.GetDeals(), true)

	if b.IsDigestBlock() == true { //If state is appended then it is the digest block
		syncer.node.EmptyRoleCaches()
		//TODO: take snapshot
	}

	// Adding the deals to the actice deals
	if syncer.node.Miner != nil { //Node having miner object shall have updated active deals
		syncer.node.Miner.UpdateCache(b)
		syncer.node.Miner.PrintPostStats()
	}

	syncer.node.RemoveExpiredDeals(b.GetTimestamp(), b.GetChPrPair())

	if syncer.node.StorageProvider != nil && syncer.isSyncing == false {
		syncer.node.StorageProvider.ProcessNewBlock(b)
	}

	if syncer.node.User != nil {
		syncer.node.UpdateThreadBase(b)
	}

	syncer.node.Bc.UpdateIrreversibleBlockNumber(minerAdd, syncer.node.Dpos.GetComitteeSize(), syncer.node.Dpos.NumberOfToMineBlockPerSlot, syncer.node.Port)
	return true
}

//RevertState reverts the state using the block- SHould be passed in descending order
func (syncer *Syncer) RevertState(revertBlock *core.Block) {

	log.Info().Msgf("RevertState for bloc %v", revertBlock)
	minerPk := revertBlock.MinerPublicKey()
	minerAdd := minerPk.String()
	syncer.node.Wm.RevertWalletStates(revertBlock, minerAdd) //Should revert in descending order

	//TODO revert deals in hash, posts in cache,
	parent, _ := syncer.node.Bc.GetBlockByHash(revertBlock.ParentHash())
	syncer.node.Dpos.RevertDposState(revertBlock.GetRoundNum(), parent.GetRoundNum())
	syncer.node.Bc.UpdateIrreversibleBlockNumberWhileReverting(minerAdd, syncer.node.Port)
	syncer.node.RemoveDeals(revertBlock.GetDeals())
	syncer.node.AddExpiredDealsWhileReverting(revertBlock.GetChPrPair())

	if revertBlock.IsDigestBlock() == true { //If state is appended then it is the digest block
	}
}

func (syncer *Syncer) handleBlockByAskByHeight(msg net.Message) {

	heightFrom, err := strconv.Atoi(string(msg.Data))
	if err != nil || err != nil {
		log.Error().Msgf("Could not convert string to integer")
		return
	}

	if syncer.node.Bc.IsEmpty() {
		log.Error().Msgf("Can't send blocks, blockchain is empty")
		return
	}

	block, err := syncer.node.Bc.GetBlockByHeight(int64(heightFrom))
	if err != nil {
		log.Error().Msgf("Can't send block, block of height: %v does not exist", heightFrom)
		return
	}

	blockBytes, err := block.ToBytes()
	if err != nil {
		log.Error().Msgf("Can't conver block to bytes")
		return
	}
	msgToSend := net.NewMessage(net.MsgCodeBlockSyncByHeight, blockBytes)
	syncer.node.Relay(msg.From, msgToSend)

}

// handleAskBlockByHash reply back the block asked by hash
func (syncer *Syncer) handleAskBlockByHash(msg net.Message) {

	blockHash := msg.Data
	b, err := syncer.node.Bc.GetBlockByHash(blockHash)
	if err != nil {
		return
	}
	blockBytes, err := b.ToBytes()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	reply := net.NewMessage(net.MsgCodeReplyBlockByHash, blockBytes)
	syncer.node.Relay(msg.From, reply)
}

func (syncer *Syncer) handleBlockAsk(msg net.Message) {
	if syncer.isSyncing {
		return
	}
	if syncer.node.Bc.IsEmpty() {
		log.Info().Msgf("Can't send blocks, blockchain is empty")
		return
	}

	HashAndZ := bytes.Split(msg.Data, []byte(" , "))
	blockHash := HashAndZ[0]
	n := HashAndZ[1]

	curEpochNum := float64(syncer.node.Bc.Length()) / float64(core.TestEpoch.Duration)

	z, err := strconv.Atoi(string(n)) // set it epoc blokc number
	if err != nil {
		log.Error().Msgf("Could not convert string to integer")
		return
	}

	var tempb *core.Block

	isStartGenesis := bytes.Equal(blockHash, syncer.node.Bc.Genesis().Hash())
	if isStartGenesis {
		tempb, err = syncer.node.Bc.GetBlockByHeight((int64(curEpochNum) * core.TestEpoch.Duration))
		if err != nil {
			log.Error().Msgf("Do not have starting  EPOCH block of hash: %v", hex.EncodeToString(blockHash))
			return
		}
		log.Info().Msgf("Sending %v blocks to From Epoch block %v %v", z, tempb, tempb.Height())
		blockHash = tempb.Hash()
	} else {
		tempb, err = syncer.node.Bc.GetBlockByHash(blockHash)
		if err != nil {
			log.Error().Msgf("Do not have starting block of hash: %v", hex.EncodeToString(blockHash))
			return
		}
		log.Info().Msgf("Sending %v blocks to From block %v %v", z, tempb, tempb.Height())
	}

	node := syncer.node
	var allBlocks [][]byte
	var toSend []byte
	if node.Bc.IsEmpty() != true { //Bascially if blockchain is not empty

		TargetHeight := tempb.Height() + int64(z)
		// }
		if TargetHeight > int64(node.Bc.Length()) {
			TargetHeight = int64(node.Bc.Length())
		}

		TargetHeightHahs, err2 := node.Bc.BlockHashByHeight(TargetHeight)
		if err2 != nil {
			log.Error().Err(err2)
			return
		}

		walk, err3 := node.Bc.GetBlockByHash(TargetHeightHahs)
		if err3 != nil {
			log.Error().Err(err3)
			return
		}

		log.Info().Msgf("starting walk %v", walk)
		for {
			blockBytes, err := walk.ToBytes()
			if err != nil {
				log.Error().Err(err)
				return
			}
			allBlocks = append([][]byte{blockBytes}, allBlocks...)
			log.Info().Msgf("Sending block %v to %v", walk, msg.From.Address)

			if bytes.Equal(walk.Hash(), blockHash) || bytes.Equal(walk.Hash(), tempb.Hash()) {
				break
			}
			walk, err = node.Bc.GetBlockByHash(walk.ParentHash())
			if err != nil {
				break
			}
		}

		log.Info().Msgf("Sending %v blocks to %v", z, msg.From.Address)
		if z > len(allBlocks) {
			toSend = bytes.Join(allBlocks, []byte(" , "))
		} else {
			toSend = bytes.Join(allBlocks[:z], []byte(" , "))

		}
	}
	if isStartGenesis { //
		log.Info().Msgf("Sending block from epoch")
		tempbytes, err := syncer.node.Bc.Genesis().ToBytes()
		if err != nil {
			return
		}
		tempbytes = append(tempbytes, []byte(" , ")...)
		toSend = append(tempbytes, toSend...)
	}

	leftOver := bytes.Split(toSend, []byte(" , "))
	log.Info().Msgf("handleBlockByAskByHeight %v", len(leftOver))

	msgToSend := net.NewMessage(net.MsgCodeBlockSync, toSend)
	syncer.node.Relay(msg.From, msgToSend)
}

func (syncer *Syncer) handleBlockSyncByHeight(msg net.Message) error {
	block, err := core.NewBlockFromBytes(msg.Data)
	if err != nil {
		log.Error().Msgf("Could not read block from bytes")
		return err
	}
	syncer.handleBlockNew(block)
	return nil
}

func (syncer *Syncer) handlBlockChainLongSyncRequest(msg net.Message) {
	if syncer.isSyncing {
		return
	}

	if syncer.node.Bc.IsEmpty() {
		log.Info().Msgf("Can't send blocks, blockchain is empty")
		return
	}

	BlockHashs := bytes.Split(msg.Data, []byte("timbre"))

	var ByteTosend []byte
	blockBytes, err := syncer.node.Bc.GetMainTail().ToBytes()
	if err != nil {
		log.Info().Msgf("handlBlockChainLongSyncRequest Error GetMainTail().ToBytes() Error %v", err)
		return
	}
	ByteTosend = append(blockBytes, []byte("timbre")...)
	if len(BlockHashs) > 1 {
		var tempb *core.Block
		var err error
		for _, blockHash := range BlockHashs {
			tempb, err = syncer.node.Bc.GetBlockByHash(blockHash)
			if err == nil {
				break
			}
		}
		if tempb == nil {
			log.Info().Msgf(" handlBlockChainLongSyncRequest No hash found in current chain")
			return
		}

		ByteTosend = append(ByteTosend, tempb.Hash()...)
		ByteTosend = append(ByteTosend, []byte("timbre")...)
		ByteTosend = append(ByteTosend, syncer.node.Bc.Genesis().Hash()...)
	} else {
		genesisBlockBytes, _ := syncer.node.Bc.Genesis().ToBytes()
		ByteTosend = append(ByteTosend, genesisBlockBytes...)
	}

	msgToSend := net.NewMessage(net.MsgCodeBlockChainLongSyncResponse, ByteTosend)

	syncer.node.Relay(msg.From, msgToSend)
}

func (syncer *Syncer) handlBlockChainLongSyncResponse(msg net.Message) {

	bytesArray := bytes.Split(msg.Data, []byte("timbre"))
	mainTailBlockByte := bytesArray[0]
	startBlockByte := bytesArray[1]

	block, err := core.NewBlockFromBytes(mainTailBlockByte)
	if err != nil {
		fmt.Printf("Error: Could not read sent tail Error %v || msg.Data length %v", err, len(msg.Data))
		return
	}

	var startBlock *core.Block
	if len(bytesArray) > 2 {

		startBlock, err = syncer.node.Bc.GetBlockByHash(startBlockByte)
		if err != nil {
			fmt.Printf("Error: Could not find startBlockHash Error %v || msg.Data length %v", err)
			return
		}
	} else {
		startBlock, err = core.NewBlockFromBytes(startBlockByte)
		if err != nil {
			fmt.Printf("Error: Could not read sent Start Block Error %v", err)
			return
		}
	}

	if syncer.targetBlock == nil {

		log.Info().Msgf("Target block set to %s", block.String())
		log.Info().Msgf("startingBlock block set to %s", startBlock.String())
		syncer.targetBlock = block
		syncer.curBlock = startBlock

	} else {
		if block.Height() > syncer.targetBlock.Height() {

			log.Info().Msgf("Target block set to %s", block.String())
			syncer.targetBlock = block

			log.Info().Msgf("startingBlock block set to %s", startBlock.String())
			syncer.curBlock = startBlock

		} else {
			log.Info().Msgf("Target block is already set.")
		}
	}
}

func (syncer *Syncer) handleBlockSync(msg net.Message) error {
	log.Info().Msgf("In HandleBlockSync")

	allBlocks := msg.Data
	if len(allBlocks) == 0 {
		return errors.New("Empty block list received")
	}
	leftOver := bytes.Split(allBlocks, []byte(" , "))
	log.Info().Msgf("HandleBlockSync %v", len(leftOver))
	lastsentBlock, err := core.NewBlockFromBytes(leftOver[len(leftOver)-1])

	if err != nil {
		return errors.New("Unable to read latest block")
	}

	log.Info().Msgf("In HandleBlockSync lastsentBlock parent %v lastsentBlock %v", hex.EncodeToString(lastsentBlock.ParentHash()), lastsentBlock)
	if syncer.curBlock != nil && lastsentBlock.Height() < syncer.curBlock.Height() { //IsSameAs(lastsentBlock) {
		return errors.New("Received blocks have already been received")
	}

	for _, blockBytes := range leftOver {
		block, err := core.NewBlockFromBytes(blockBytes)
		if err != nil {
			continue
		}
		if block.HasValidSigWithID(core.DefaultConfig.ChainID) { //Valid sign should have same chain-id as set in config
			//if block belongs to same chain-id
			syncer.handleBlockNew(block)
		} else if core.DefaultConfig.Revolt { //If I am revolting then I can force append old chain blocks
			//Meant for just forming a chain of blocks without having a state update
			syncer.ForceAppendBlock(block)
		}

	}

	return nil
}

//PrintBlockChain prints the blocks in blockchain
func (syncer *Syncer) PrintBlockChain() {
	log.Info().Msgf("Syncer: BlockChain:\n %s", syncer.node.BlockChainString())
}

//How about the case in which block from the last miner comes in the next miner slot

//ValidateMinerSlot validates the slot of the miner
func (syncer *Syncer) ValidateMinerSlot(block *core.Block, d *dpos.Dpos) error { //TODO check per miner tolerance

	bPk := block.MinerPublicKey().String()
	bRound := block.GetRoundNum()
	if block.IsGenesis() == true {
		return nil
	}

	bHeight := block.Height()
	blocksPerMiner := d.BlocksPerMiner()

	err := syncer.VerifyLastMiner(block, d) //In this case this is the block from last miner which got delayed then just check
	if err != nil {
		return err
	}

	bHeightCheck := bHeight - int64(blocksPerMiner)
	if bHeightCheck >= 0 {
		refBlock, err := syncer.node.Bc.GetBlockByHeight(bHeightCheck)
		if err != nil {
			log.Warn().Msgf("Blocks doesn't exist at %v to check blocks produced by miner in round", bHeightCheck)
			return nil
		}
		refRound := refBlock.GetRoundNum()
		//Checking the miner of the reference block
		refMinerPk := refBlock.MinerPublicKey().String()

		if bPk == refMinerPk && refRound == bRound {
			log.Info().Err(errors.New("MORE BLOCKS PRODUCED THAN ALLOWED"))
			//TODO:- Put punishment protocol here
			return errors.New("More blocks produced than allowed")
		}
	}

	return nil
}

//VerifyLastMiner verfies the last miner in the round turn
func (syncer *Syncer) VerifyLastMiner(block *core.Block, d *dpos.Dpos) error {

	bPk := block.MinerPublicKey().String()
	bRound := block.GetRoundNum()
	bTs := block.GetTimestamp()
	phash := block.ParentHash()
	pBlock, err := syncer.node.Bc.GetBlockByHash(phash)
	if err != nil {
		log.Info().Err(err)
		return err
	}
	if pBlock == nil {
		return nil
	}
	pPk := pBlock.MinerPublicKey().String()
	pRound := pBlock.GetRoundNum()
	pTs := pBlock.GetTimestamp()
	tickInterval := d.GetTickInterval().Nanoseconds()

	timeDif := bTs - pTs
	if timeDif < int64(tickInterval-2e9) || timeDif > int64(tickInterval+2e9) { //maintaining 2s tolerance     tickInterval-2 <=timeDiff<= tickInterval+2
		log.Warn().Err(errors.New("block - parent TS is less then tick interval"))
	}

	miningErr := d.ValidateMiningOrder(pPk, bPk, pRound, bRound, bTs-pTs, bTs, syncer.node.Bc.Genesis().GetTimestamp()) //This validation only happens accors the miners
	if miningErr != nil {
		return miningErr
	}
	return nil
}

//SyncBlockchain asks for the new block in case main tail is stale
func (syncer *Syncer) SyncBlockchain() {
	log.Info().Msgf("Running sync chain protocol new...")

	if core.DefaultConfig.Revolt { //&& core.DefaultConfig.RevoltHeight > blockheight { //If I am revolting then I need to sync till the revoltHeight and then start my own chain
		syncer.SyncBlockchainForRevolt()
		return
	}

	if syncer.isSyncing == true {
		log.Info().Msgf("Already syncing...")
		return
	}

	syncer.isSyncing = true
	defer func(syncer *Syncer) {
		syncer.isSyncing = false
	}(syncer)

	syncer.targetBlock = nil
	syncer.curBlock = nil
	blockheight := int64(0)
	counterToGetMainTail := 0
	if syncer.node.Bc.IsEmpty() != true { //Basically if chain is not nil
		syncer.node.Dpos.Stop()
		syncer.node.Dpos.RevertDposTill(syncer.node.Bc.GetMainTail().GetRoundNum())
		blockheight = syncer.node.Bc.GetMainTail().Height()
	}

L:
	for {
	L1:
		for {
			var lastNblocksData []byte
			log.Info().Msgf("ResyncChain: Restart %v", counterToGetMainTail)
			for i := 0; i < 10 && blockheight > 0; i++ {
				b, _ := syncer.node.Bc.GetBlockByHeight(blockheight)
				lastNblocksData = append(lastNblocksData, b.Hash()...)
				lastNblocksData = append(lastNblocksData, []byte("timbre")...)
				blockheight = blockheight - 1
			}

			msg := net.NewMessage(net.MsgCodeBlockChainLongSyncResquest, lastNblocksData)

			ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
			for _, peer := range syncer.chooseNPeers(noOfNodesToQuery) {
				syncer.node.Relay(peer, msg)
			}

		L2:
			for {
				select {
				case <-time.After(1 * time.Second):
					fmt.Println("ResyncChain: wait till targetBlock is set")
					if syncer.targetBlock != nil {
						cancel()
						break L1
					}
				case <-ctx.Done():
					fmt.Println("ResyncChain: ctx.Done case")
					if syncer.targetBlock != nil {
						cancel()
						break L1
					}
					if syncer.node.Bc.IsEmpty() {
						counterToGetMainTail += 1
						if counterToGetMainTail >= 2 {
							fmt.Println("ResyncChain: Blockchain in nil, could not find any Tail, start own chain, returning counterToGetMainTail", counterToGetMainTail)
							break L
						}
					}
					cancel()
					break L2

				}
			}
		}

		if syncer.node.Bc.IsEmpty() || syncer.targetBlock.Height() > syncer.node.Bc.GetMainTail().Height() {
			syncer.SyncBlockchainCore(10, true)
		} else {
			break L
		}
	}

	log.Info().Msgf("Sync done. Startig DPos now...")
	err := syncer.node.Services.StartDpos()
	if err != nil {
		if err.Error() == "Stale Main-Tail" {
			syncer.isSyncing = false
			go syncer.SyncBlockchain()
		}
		// return err
	}
	fmt.Println("ResyncChain: syncer.isSyncing ", syncer.isSyncing)
}

//SyncBlockchainCore fetches block in batch
func (syncer *Syncer) SyncBlockchainCore(z int, iterateOverBlockPool bool) {
	var lastBlockHash []byte
	for {
		b1 := (syncer.node.Bc.IsEmpty() != true && syncer.targetBlock.Height() <= syncer.node.Bc.GetMainTail().Height())
		b2 := syncer.curBlock != nil && syncer.curBlock.IsSameAs(syncer.targetBlock)
		if b1 || b2 {
			log.Info().Msgf("Sync Complete. b1 %v b2 %v", b1, b2)
			log.Info().Msgf("Sync Complete. Target Height: %d ||| Current Block Height:  %d ||| Main Height:  %d", syncer.targetBlock.Height(), syncer.curBlock.Height(), syncer.node.Bc.GetMainTail().Height())
			break
		}

		lastBlockHash = syncer.curBlock.Hash()
		log.Info().Msgf("syncer.curBlock height %v", syncer.curBlock.Height())

		askMsg := syncer.createAskMsg(lastBlockHash, z)
		respChan := make(chan net.Message, 128)
		syncer.node.AddOutgoingChan(respChan)
		for _, item := range syncer.chooseNPeers(noOfNodesToQuery) {
			syncer.node.Relay(item, askMsg)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second) // 3 second timeout
	L:
		for {
			select {
			case <-ctx.Done():
				log.Error().Msgf("Context timed out before blocks received")
				break L
			case msg := <-respChan:
				if msg.Code == net.MsgCodeBlockSync {

					err := syncer.handleBlockSync(msg) // Add error handling
					if err != nil {
						log.Error().Msgf("HandleBlockSync err %v", err)
					}
					break L
				}

			}
		}
		cancel()
		syncer.node.RemoveOutgoingChan(respChan)
	}

	if iterateOverBlockPool {
		log.Info().Msgf("Network Sync Done, Iterate Blockpool")
		tails := syncer.node.Bp.GetTails()
		for _, tail := range tails {
			chain := syncer.node.Bp.GetChainByTail(tail)
			log.Info().Msgf("BP tails: %v", tails)
			syncer.IterateCheckAndAddChain(chain)
		}
	}
}

//ForceAppendBlock will append block forcefully. It will not trigger state update. Useful for adding blocks when revolting
func (syncer *Syncer) ForceAppendBlock(block *core.Block) error {
	node := syncer.node
	log.Info().Msgf("Force appending block...")

	if !node.Bc.Contains(block) && !node.Bp.Contains(block) && block.Height() <= core.DefaultConfig.RevoltHeight {
		node.Bp.Add(block)
		tails := syncer.node.Bp.GetTails()
		for _, tail := range tails {
			chain := syncer.node.Bp.GetChainByTail(tail)
			log.Info().Msgf("Chain to add %v for tail %v", chain, tail)
			if chain[0].Height() < syncer.node.Bc.IrreversibleBlock { // remove chain

				for _, b := range chain {
					syncer.node.Bp.Remove(b.Hash())
				}
			} else {

				for _, b := range chain {

					if !b.IsGenesis() {
						//Validating blocks
						pBlock, err := node.Bc.GetBlockByHash(b.ParentHash())
						if err != nil {
							return err
						}
						if !pBlock.IsValidParentOf(b) {
							return fmt.Errorf("Is not valid child of parent")
						}
					}

					fmt.Println("Appending blocks without updating state...")
					err := syncer.node.Bc.Append(b)

					if err != nil {
						log.Error().Msgf("error: ", err)
						fmt.Println("GOT ERROR: ", err.Error())
						node.Bp.Remove(b.Hash())
						continue
					} else {
						fmt.Println("Added: ", b)
					}
					node.Bp.Remove(b.Hash())

					if syncer.isSyncing && syncer.curBlock.Height() < b.Height() {
						syncer.curBlock = b //.Hash()
					}

				}
			}
		}
	} else {
		node.Bp.Remove(block.Hash())
	}

	return nil
}

//SyncBlockchainForRevolt for syncing blocks from parent chain
func (syncer *Syncer) SyncBlockchainForRevolt() {

	parentID, err := core.DefaultConfig.GetParentChainID()
	if err != nil {
		log.Error().Msgf(err.Error())
		return
	}

	syncer.node.SetNetworkChainID(parentID) //Setting the chain-id to accept the parent chain blocks

	if syncer.isSyncing == true {
		log.Info().Msgf("Already syncing...")
		return
	}

	syncer.isSyncing = true
	defer func(syncer *Syncer) {
		syncer.isSyncing = false
	}(syncer)

	syncer.targetBlock = nil
	syncer.curBlock = nil
	var revoltTargetBlock *core.Block
	revoltTargetBlock = nil
	for i := 0; i < 5; i++ { //Will try asking for the block for 5 times
		syncer.AskBlockByHash(core.DefaultConfig.RevoltBlockHash)
		msg, err := syncer.node.GetResponse(net.MsgCodeReplyBlockByHash)
		if err != nil {
			log.Warn().Msgf(err.Error())
			continue
		} else {
			b, err := core.NewBlockFromBytes(msg.Data)
			if err != nil {
				panic(err.Error())
			}
			revoltTargetBlock = b
			break
		}
	}

	if revoltTargetBlock == nil {
		return
	}
	log.Info().Msgf("revoltTargetBlock: %v", revoltTargetBlock)

L:
	for {
		var lastNblocksData []byte
		msg := net.NewMessage(net.MsgCodeBlockChainLongSyncResquest, lastNblocksData)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, peer := range syncer.chooseNPeers(noOfNodesToQuery) {
			syncer.node.Relay(peer, msg)
		}
		for {
			select {
			case <-time.After(1 * time.Second):
				fmt.Println("ResyncChain: wait till targetBlock is set")

			case <-ctx.Done():
				fmt.Println("ResyncChain: ctx.Done case")
				if syncer.targetBlock != nil {
					cancel()
					break L
				}
				break
			}
		}
	}

	if syncer.targetBlock != nil {
		syncer.targetBlock = revoltTargetBlock
		syncer.SyncBlockchainCore(10, false)

	} else {
		fmt.Println("Failed to fetch target block")
		return
	}

	if !bytes.Equal(syncer.node.Bc.GetMainTail().Hash(), core.DefaultConfig.RevoltBlockHash) {
		log.Error().Msgf("Didn't reach target height")
		//Try re-running the sync protocol
		return
	}

	log.Info().Msgf("We are all-set to Revolt")
	syncer.node.Bp, _ = core.NewBlockpool(core.DefaultBlockpoolCapacity)
	syncer.node.SetNetworkChainID(core.DefaultConfig.ChainID) //Setting back chain-id

	log.Info().Msgf("Sync done. Startig DPos now...")
	err = syncer.node.Services.StartDpos()
	if err != nil {
		if err.Error() == "Stale Main-Tail" {
			syncer.isSyncing = false
			go syncer.SyncBlockchain()
		}
	}

}
