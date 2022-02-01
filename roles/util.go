package roles

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/guyu96/go-timbre/core"
	"github.com/guyu96/go-timbre/core/blockstate"
	"github.com/guyu96/go-timbre/crypto"
	"github.com/guyu96/go-timbre/dpos"
	"github.com/guyu96/go-timbre/log"
	"github.com/guyu96/go-timbre/net"
	"github.com/guyu96/go-timbre/protobuf"
	"github.com/guyu96/go-timbre/storage"
	"github.com/guyu96/go-timbre/wallet"
)

// GenerateTestBlock creats a test block at the given height
func (miner *Miner) GenerateTestBlock(parent *core.Block, transactions []*protobuf.Transaction, votes []*protobuf.Vote, deals []*protobuf.Deal, chPrPair []*protobuf.ChPrPair, podfs []*protobuf.PoDF, roundNum, bTs, height int64, broadcast bool) *core.Block {
	var block *core.Block
	var err error
	node := miner.node
	minerPk := miner.node.PublicKey()
	minerAdd := minerPk.String()
	node.BcMtx.Lock()
	defer node.BcMtx.Unlock()

	if node.Bc.Genesis() == nil { //if no genesis block
		log.Info().Msgf("Creating genesis block")
		trans := miner.GetDelegateRegTx()
		err := miner.CheckCandWithDelRegTx(trans)
		if err != nil {
			fmt.Println(err.Error())
			return nil
		}

		if broadcast == false {
			if core.DefaultConfig.Revolt {
				block, err = core.NewGenesisBlockWithTs(core.DefaultConfig.RevoltBlockHash, minerPk, miner.node.TimeOffset, trans, bTs, core.DefaultConfig.RevoltHeight+1)
			} else {
				block, err = core.NewGenesisBlockWithTs(nil, minerPk, miner.node.TimeOffset, trans, bTs, 0)
			}
		} else {
			if core.DefaultConfig.Revolt {
				block, err = core.NewGenesisBlock(core.DefaultConfig.RevoltBlockHash, minerPk, miner.node.TimeOffset, trans, core.DefaultConfig.RevoltHeight+1)
			} else {
				block, err = core.NewGenesisBlock(nil, minerPk, miner.node.TimeOffset, trans, 0)
			}
		}

		block.Sign(node.PrivateKey())

	} else {

		if broadcast == false {
			block = core.CreateNewBlockWithTs(parent, node.PublicKey(), node.PrivateKey(), transactions, votes, deals, chPrPair, podfs, miner.node.TimeOffset, roundNum, bTs) //Extra transaction variable as comp to MakeTestBlock
		} else {
			var tchPrPair []*protobuf.ChPrPair
			for _, chPr := range miner.ReceivedChPrPairForTesting {

				chPr.Timestamp = time.Now().Unix()
				tchPrPair = append(tchPrPair, chPr)
			}
			block = core.CreateNewBlock(parent, node.PublicKey(), node.PrivateKey(), transactions, votes, deals, tchPrPair, podfs, miner.node.TimeOffset, roundNum) //Extra transaction variable as comp to MakeTestBlock
		}
	}

	isValid := block.IsValid()
	if isValid != true {
		log.Info().Msgf("Block is not valid. Returning")
		return nil
	}

	err = node.Wm.CheckAndUpdateWalletStates(block, minerAdd) // only added validated transaction
	if err != nil {
		log.Error().Msgf("Miner -- Wallet state could not be updated")
		log.Error().Msgf(err.Error())
	}

	miner.node.AddDeals(deals, true)
	for _, deal := range deals {
		for _, infopr := range deal.List {
			hashbytes, _ := proto.Marshal(infopr.Info)
			hash := crypto.Sha256(hashbytes)
			miner.prCache.Remove(string(hash))
			miner.stalePrCache.Remove(string(hash))
			miner.node.Db.Put(storage.PostStoredBucket, hash, []byte("0"))
		}
	}
	miner.node.RemoveExpiredDeals(block.GetTimestamp(), block.GetChPrPair())
	if miner.node.StorageProvider != nil {
		miner.node.StorageProvider.ProcessNewBlock(block)
	}
	if miner.node.User != nil {
		miner.node.UpdateThreadBase(block)
	}

	if err != nil {
		log.Error().Err(err)
		log.Error().Msgf("Miner -- Block Mined but Not added")
		return nil
	}

	blockBytes, err := block.ToBytes()
	if err != nil {
		log.Error().Err(err)
		return nil
	}
	if broadcast == false {
		log.Info().Msgf("Prepared %s", block)

	} else {
		log.Info().Msgf("CREATED %s", block)
	}
	node.Bc.Append(block)
	node.Bc.UpdateMainTailIndex()
	log.Info().Msgf("New Blockchain: %s", node.Bc)
	miner.PrintPostStats()
	miner.node.Wm.ResetWalletCache()
	miner.ChPrPair = nil

	if broadcast == true {
		msg := net.NewMessage(net.MsgCodeBlockNew, blockBytes)
		node.Broadcast(msg)
	}

	return block
}

//CheckBlockState fetches blockstate from the state-store
func (syncer *Syncer) CheckBlockState(b *core.Block) bool {

	//checking if the parent hash exists in the cached componets
	stateExists := syncer.node.StateStore.CheckCachedComponent(b.Hash())
	return stateExists
}

//PutBlockState puts the blockstate in the state-store
func (syncer *Syncer) PutBlockState() error {
	log.Info().Msgf("Taking snapshot of the DPOS and wallets")
	err := syncer.node.TakeAndStoreStateSnapShot()
	return err
}

//CheckAndGetBlockState checks and returns the cached state
func (syncer *Syncer) CheckAndGetBlockState(b *core.Block) *blockstate.StateComponents {
	stateComp := new(blockstate.StateComponents)
	if syncer.CheckBlockState(b) {
		stateComp = syncer.node.StateStore.GetCachedComponent(b.Hash()) //Should generate state of the parent
		if stateComp == nil {
			return nil
		}
		return stateComp
	}
	return nil
}

//GetStateForBlockValidation returns the block state from the cache. Note:- TO validate a block parent state should be fetched
func (syncer *Syncer) GetStateForBlockValidation(b *core.Block) (*blockstate.StateComponents, error) {

	stateComp := new(blockstate.StateComponents)

	if syncer.CheckBlockState(b) {
		stateComp = syncer.node.StateStore.GetCachedComponent(b.Hash()) //Should generate state of the parent
		if stateComp == nil {
			return nil, errors.New("State doesn't exists")
		}
		return stateComp, nil
	}

	err := syncer.PutBlockState() //takes and persists the snapshot of the tail-block state
	if err != nil {
		fmt.Println("got error while putting the block", err.Error())
		return nil, err
	}

	err = syncer.node.GenerateStateSnapshot(b.Hash())
	if err != nil {
		fmt.Println("Error in generating state snapshot", err.Error())
	}
	stateComp = syncer.node.StateStore.GetCachedComponent(b.Hash())
	if stateComp == nil {
		fmt.Println("Failed to generate state")
		return nil, errors.New("Failed to generate state")
	}
	fmt.Println("Finally got the generated snapshot")
	return stateComp, nil

}

//ValidateForNonMainTail validates the main-tail block
func (syncer *Syncer) ValidateForNonMainTail(b *core.Block, d *dpos.Dpos, wm *wallet.WalletManager) error {

	fmt.Println("Validating for the non-main tail")
	if d.HasStarted == false { //while node is downloading old blocks already produced
		fmt.Println("Updating no-main Dpos call from syncer...")
		err := syncer.node.Services.UpdateDpos(b, d)
		if err != nil {
			log.Info().Err(err)
		}
	}

	//Validating for the digest block
	parentBlock, err := syncer.node.Bc.GetBlockByHash(b.ParentHash())
	if err != nil {
		return err
	}

	bcHeight := int64(parentBlock.Height())
	if bcHeight%core.TestEpoch.Duration == 0 && bcHeight != 0 { //If state is appended in the block then it is the digest block

		curEpochNum := float64(bcHeight) / float64(core.TestEpoch.Duration)

		if b.IsDigestBlock() == true {
			err := wm.ValidateDigestBlock(b)
			if err != nil {
				return err
			}
			err = d.ValidateDigestMiners(b.PbBlock.GetState().GetMiners(), b.PbBlock.State.NextRoundMiners)
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
	errT := syncer.ValidateMinerSlot(b, d)
	if errT != nil {
		log.Info().Err(errT) //TODO :- Return false in case of error
		fmt.Println(errT.Error())
		return errT // THIS SHOULD CHANGE
	}

	err2 := wm.CheckAndUpdateWalletStates(b, b.MinerPublicKey().String())
	if err2 != nil {
		fmt.Println("Error updating the states: ", err2.Error())
		log.Info().Err(err2)
		return err2
	}

	return nil
}

//CreateAndStartMiner will first create new miner instance start it and attach role to the daemon process
func CreateAndStartMiner(node *net.Node) error {

	if node == nil {
		return fmt.Errorf("Daemon node is not running")
	}
	if node.Miner != nil { //Will return if miner instance is already running
		return fmt.Errorf("Miner instance was already created and is running now")
	}
	miner := NewMiner(node)
	node.SetMiner(miner)
	node.Services.AnnounceAndRegister()
	miner.StartMiner()
	log.Info().Msgf("Miner process has started")
	return nil
}

//CreateAndStartStorageProvider creates new sp(storage provider) and start it
func CreateAndStartStorageProvider(node *net.Node) error {

	if node == nil {
		return fmt.Errorf("Daemon node is not running")
	}
	if node.StorageProvider != nil { //Will return if miner instance is already running
		return fmt.Errorf("Storage provider instance has already been created")
	}

	provider := NewStorageProvider(node)
	node.SetStorageProvider(provider)
	go provider.Process()
	log.Info().Msgf("Storage provider process has started")
	return nil
}

//CreateAndStartUser creates new user instance and start it
func CreateAndStartUser(node *net.Node) error {

	if node == nil {
		return fmt.Errorf("Daemon node is not running")
	}
	if node.User != nil { //Will return if miner instance is already running
		return fmt.Errorf("User instance is already running ")
	}

	user := NewUser(node)
	node.SetUser(user)
	go node.User.Setup()
	go node.User.Process()
	log.Info().Msgf("User process has started")

	return nil
}
