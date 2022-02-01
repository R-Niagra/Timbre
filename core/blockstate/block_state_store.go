package blockstate

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/guyu96/go-timbre/core"
	"github.com/guyu96/go-timbre/log"
	"github.com/guyu96/go-timbre/storage"
)

const (
	cacheSize = 20
)

var (
	blockStateBucket []byte
)

func init() {
	blockStateBucket = []byte("Block_state")
}

//StateStore manages the state of the block
type StateStore struct {
	db          *storage.Database // database connection
	cachedState []*BlockState     //For now not using this cache
	Stored      map[string]int64
	CachedComp  []*StateComponents //map of hash-> Dpos&wallets struct
}

//NewStateStore returns the new state store
func NewStateStore(database *storage.Database) (*StateStore, error) {

	store := &StateStore{
		db:          database,
		cachedState: make([]*BlockState, cacheSize),
		Stored:      make(map[string]int64),
	}

	if !store.db.HasBucket(storage.BlockStateBucket) {
		fmt.Println("creating new bucket ", string(storage.BlockStateBucket))
		err := store.db.NewBucket(storage.BlockStateBucket)
		if err != nil {
			log.Warn().Msgf("Block state bucket failed to initialize")
			return nil, err
		}
	} else {

	}
	return store, nil
}

//PersistBlockState persists the blockstate to the Db
func (ss *StateStore) PersistBlockState(blockState *BlockState, port uint16) error {
	stateBytes, err := blockState.ToBytes()
	if err != nil {
		return err
	}

	err = ss.db.Put(storage.BlockStateBucket, blockState.State.Hash, stateBytes)
	if err != nil {
		return err
	}
	fmt.Println("put successfully: state hash: ", blockState.State.Hash, port)
	ss.Stored[hex.EncodeToString(blockState.State.Hash)] = blockState.State.Height

	return nil
}

//CheckState checks if the state of the block exists
func (ss *StateStore) CheckState(blockHash []byte) bool {
	if _, ok := ss.Stored[hex.EncodeToString(blockHash)]; ok {
		return true
	}
	return false
}

//GetBlockState gets the blockstate from the db
func (ss *StateStore) GetBlockState(blockHash []byte) (*BlockState, error) {
	if ss.CheckState(blockHash) {
		stateBytes, err := ss.db.Get(storage.BlockStateBucket, blockHash)

		if err != nil {
			fmt.Println("db state fetch failed: ", err.Error())
			return nil, err
		}

		blockState, err := StateFromBytes(stateBytes)
		if err != nil {
			return nil, err
		}

		return blockState, nil
	}
	fmt.Println("check state failed")
	return nil, errors.New("Block state doesn't exist in the Db")
}

//RemoveState removes the state from the Db
func (ss *StateStore) RemoveState(blockHash []byte) {
	if ss.CheckState(blockHash) {
		ss.db.Delete(storage.BlockStateBucket, blockHash)
	}
	delete(ss.Stored, hex.EncodeToString(blockHash)) //deleting it from the map function

}

//GenerateComponentsFromBLockState generates dpos and wallets from the state snapshot
func (ss *StateStore) GenerateComponentsFromBLockState(bState *BlockState, myID string) (*StateComponents, error) {

	sc := &StateComponents{
		CWallets:    bState.GetWmFromState(myID),
		BlockHeight: bState.State.Height,
		BlockHash:   bState.State.Hash,
	}
	d, err := bState.GetDposFromState(myID, sc.CWallets)
	if err != nil {
		return nil, err
	}
	sc.CDpos = d

	return sc, nil
}

//GetCachedComponent returns the cached state components
func (ss *StateStore) GetCachedComponent(bHash []byte) *StateComponents {

	for _, comp := range ss.CachedComp {
		if bytes.Equal(comp.BlockHash, bHash) {
			return comp
		}
	}

	return nil
}

//CheckCachedComponent returns the cached state components
func (ss *StateStore) CheckCachedComponent(bHash []byte) bool {

	for _, comp := range ss.CachedComp {
		if bytes.Equal(comp.BlockHash, bHash) {
			return true
		}
	}

	return false
}

//GetClosestState returns the closest state
func (ss *StateStore) GetClosestState(ID string, bc *core.Blockchain, refHeight int64) (*StateComponents, error) {
	fmt.Println("geting the closest state")
	blockHash := []byte{}
	//For now just adding the state
	var closestHeight int64 = -1
	for k, height := range ss.Stored {
		hashBytes, err := hex.DecodeString(k) //hash in bytes of the block
		if err != nil {
			fmt.Println("error decoding: ", err.Error())
			return nil, err
		}
		b, err := bc.GetBlockByHeight(height)
		if err != nil {
			log.Info().Msgf(err.Error())
			return nil, err
		}
		if bytes.Equal(b.Hash(), hashBytes) { //will only consider state present in main cannonical chain
			fmt.Println("State of cannonical-chain block present")
			if height > closestHeight { //SHould refHeight be used? Or just use the latest blockstate
				closestHeight = height
				blockHash = hashBytes
			}
		}
	}

	if closestHeight > -1 {
		fmt.Println("closest state height is: ", closestHeight)
		bState, err := ss.GetBlockState(blockHash)
		if err != nil {
			fmt.Println("err in get state: ", err.Error())
			return nil, err
		}

		stateComp, err := ss.GenerateComponentsFromBLockState(bState, ID)
		if err != nil {
			return nil, err
		}

		return stateComp, nil
	}

	return nil, errors.New("No closest state found")
}

//CacheStateComp caches the state component
func (ss *StateStore) CacheStateComp(stateComp *StateComponents) {

	if ss.CheckCachedComponent(stateComp.BlockHash) {
		log.Warn().Msgf("Cached state already exists. Anyway appending!")
	}
	if len(ss.CachedComp) > 10 {
		fmt.Println("Cached comp exceeds 20. Popping first")
		ss.CachedComp = ss.CachedComp[1:]
	}
	ss.CachedComp = append(ss.CachedComp, stateComp)
}

//ClearCachedState clears the cached state
func (ss *StateStore) ClearCachedState() {

	ss.cachedState = []*BlockState{}
	fmt.Println("Cached state has been cleared")

}
