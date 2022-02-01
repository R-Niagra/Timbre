package blockstate

import (
	"github.com/guyu96/go-timbre/dpos"
	pb "github.com/guyu96/go-timbre/protobuf"
	"github.com/guyu96/go-timbre/wallet"
)

//StateComponents is the light weight version of DPos and WalletManager instance
type StateComponents struct {
	CDpos       *dpos.Dpos //caches dpos
	CWallets    *wallet.WalletManager
	BlockHash   []byte
	BlockHeight int64
}

//UpdateStateComp updates the stateComponents
func (sc *StateComponents) UpdateStateComp(bHash []byte, height int64) {
	sc.BlockHash = bHash
	sc.BlockHeight = height
}

//BlockStateFromComponents generate back block-state from the components
func (sc *StateComponents) BlockStateFromComponents() *BlockState {
	bState := &BlockState{
		State: &pb.FullBlockState{
			Hash:      sc.BlockHash,
			Height:    sc.BlockHeight,
			Wallets:   &sc.CWallets.SnapShotWalletsUsingPb().PbWallets,
			DposState: &sc.CDpos.DposSnapshotUsingPb().SnapShot,
		},
	}
	return bState
}
