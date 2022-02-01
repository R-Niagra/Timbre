package dpos

import (
	dposstate "github.com/guyu96/go-timbre/dpos/state"
	pb "github.com/guyu96/go-timbre/protobuf"
)

//BlockchainAccess to access blockchain related funcction in dpos
type BlockchainAccess interface {
	Length() int //To get the length of the blockchain
}

//StateSnapshot is the snapshot of the state of dpos
type StateSnapshot struct {
	roundNumber   int64
	nextRoundCom  []*dposstate.Candidate
	MinersByRound [][]string
	CandState     *dposstate.State
}

//AddNextRoundCom deep copies the next round committee to the snapshot struct
func (sn *StateSnapshot) AddNextRoundCom(nextCom []*dposstate.Candidate) {
	sn.nextRoundCom = []*dposstate.Candidate{}

	for _, cand := range nextCom {
		candidate := *cand
		sn.nextRoundCom = append(sn.nextRoundCom, &candidate)
	}
}

//PbStateSnapshot is the state snapshot of the Dpos
type PbStateSnapshot struct {
	SnapShot pb.DposSnapShot
}

//AddNextRoundComPb deep copies the next round committee to the snapshot struct
func (psn *PbStateSnapshot) AddNextRoundComPb(nextCom []*dposstate.Candidate) {
	psn.SnapShot.NextRoundCom = []*pb.Candidate{}

	for _, cand := range nextCom {
		psn.SnapShot.NextRoundCom = append(psn.SnapShot.NextRoundCom, cand.ToProto())
	}
}
