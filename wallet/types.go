package wallet

import (
	"github.com/Nik-U/pbc"
	"github.com/golang/protobuf/proto"
	pb "github.com/guyu96/go-timbre/protobuf"
)

//WalletAccess gives the read access to certain variables
type WalletAccess interface {
	GetCandVotePower(candID string) (uint64, int64, error)
	GetRegisteredCandidates() []string
}

// to access functions related to the deal
type blockChain interface {
	BinaryDealSearch(ts int64) (*pb.Deal, int64, error)
	FindLastVerification(dealHeight int64, dealHash []byte) (int64, error)
	LinearDealSearch(ts int64) (*pb.Deal, int64, error)
}

//TxExecutor for executing transaction and reflect changes in the DPOS
type TxExecutor interface {
	ExecuteQuitCandidate(tx *pb.Transaction) error
	ExecuteRegisterCandidate(tx *pb.Transaction) error
	FindCandByAdd(add string) bool
}

type Node interface {
	GetDealbyHash(dealHash string) *pb.Deal
	GetPosPairing() *pbc.Pairing
	AddLastChVerificationsToDb(lastVrf []int64, dealHash []byte)
	GetLastChVerificationsFromDb(dealHash []byte) []int64
	DeleteLastChVerificationsFromDb(dealHash []byte)
}

//WalletsSnapshot is the snap shot of the wallet manager state- Not using for now
type WalletsSnapshot struct {
	wallets []*Wallet
}

//PbWalletsSnapShot is the snapshot of the protobuf wallet structs
type PbWalletsSnapShot struct {
	PbWallets pb.WalletStates
}

//ToBytes convert it to bytes
func (pbw *PbWalletsSnapShot) ToBytes() ([]byte, error) {
	snapBytes, err := proto.Marshal(&pbw.PbWallets)
	if err != nil {
		return []byte{}, err
	}
	return snapBytes, nil
}

//FromBytes convert back to the struct
func FromBytes(snapshotBytes []byte) (*PbWalletsSnapShot, error) {
	var pbSnapshot = new(PbWalletsSnapShot)
	if err := proto.Unmarshal(snapshotBytes, &pbSnapshot.PbWallets); err != nil {
		return nil, err
	}
	return pbSnapshot, nil
}
