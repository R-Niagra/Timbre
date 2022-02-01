package blockstate

import (
	"github.com/golang/protobuf/proto"
	"github.com/guyu96/go-timbre/dpos"
	pb "github.com/guyu96/go-timbre/protobuf"
	"github.com/guyu96/go-timbre/wallet"
)

//BlockState is the state until the block
type BlockState struct {
	State *pb.FullBlockState
}

//NewBlockState creates the new blockState
func NewBlockState(blockHash []byte, h int64, wm *wallet.WalletManager, d *dpos.Dpos) *BlockState {

	bState := &BlockState{
		State: &pb.FullBlockState{
			Hash:      blockHash,
			Height:    h,
			Wallets:   &wm.SnapShotWalletsUsingPb().PbWallets,
			DposState: &d.DposSnapshotUsingPb().SnapShot,
		},
	}

	return bState
}

//ToBytes convert state to byte
func (bs *BlockState) ToBytes() ([]byte, error) {
	sBytes, err := proto.Marshal(bs.State)
	if err != nil {
		return []byte{}, err
	}
	return sBytes, nil
}

//StateFromBytes creates the state from bytes
func StateFromBytes(stateBytes []byte) (*BlockState, error) {
	bState := new(BlockState)
	bState.State = new(pb.FullBlockState)

	if err := proto.Unmarshal(stateBytes, bState.State); err != nil {
		return nil, err
	}
	return bState, nil
}

//GetDposFromState returns dpos from the block state
func (bs *BlockState) GetDposFromState(address string, wm *wallet.WalletManager) (*dpos.Dpos, error) {

	dpos, err := dpos.NewDposUsingSnapShot(bs.State.DposState, address, wm)
	if err != nil {
		return nil, err
	}
	return dpos, nil
}

//GetWmFromState returns the wallets from the block state
func (bs *BlockState) GetWmFromState(address string) *wallet.WalletManager {

	wm := wallet.NewWalletManagerFromPb(bs.State.Wallets, address)

	return wm
}
