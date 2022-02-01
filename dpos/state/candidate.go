package dposstate

import (
	"time"

	"github.com/golang/protobuf/proto"
	pb "github.com/guyu96/go-timbre/protobuf"
	"github.com/guyu96/go-timbre/wallet"
)

// Candidate is a struct to create candidate instance
type Candidate struct {
	Address    string
	ID         string
	VotePower  int64
	TimeStamp  int64
	bannedTill int64
}

//NewCandidate returns the new Candidate opject
func NewCandidate(id, address string, votePower int64, timestamp int64, bannedTillHeight int64) *Candidate {
	newCandidate := &Candidate{
		ID:         id,
		Address:    address,
		VotePower:  votePower,
		TimeStamp:  timestamp,
		bannedTill: bannedTillHeight,
	}
	return newCandidate
}

//NewCandidateByWallet uses wallet address to create new candidate
func NewCandidateByWallet(id string, candWallet *wallet.Wallet, votePower int64, timestamp time.Time) *Candidate {
	newCandidate := &Candidate{
		ID:         id,
		Address:    candWallet.GetAddress(),
		VotePower:  votePower,
		TimeStamp:  timestamp.Unix(),
		bannedTill: candWallet.GetBannedTill(),
	}
	return newCandidate
}

//ToProto converts return proto candidate object from Candidate type
func (c *Candidate) ToProto() *pb.Candidate {
	cand := &pb.Candidate{
		Address:    c.Address,
		ID:         c.ID,
		VotePower:  c.VotePower,
		TimeStamp:  c.TimeStamp,
		BannedTill: c.bannedTill,
	}
	return cand
}

//FromProto converts
func FromProto(pbCandidate *pb.Candidate) *Candidate {
	cand := &Candidate{
		ID:         pbCandidate.ID,
		Address:    pbCandidate.Address,
		VotePower:  pbCandidate.VotePower,
		TimeStamp:  pbCandidate.TimeStamp,
		bannedTill: pbCandidate.BannedTill,
	}
	return cand
}

//ToBytes changes the candidte into bytes
func (c *Candidate) ToBytes() ([]byte, error) {
	pbCand := c.ToProto()
	return proto.Marshal(pbCand)
}

//FromBytes converts back to the candidate from bytes
func FromBytes(bytes []byte) (*Candidate, error) {
	pbCandidate := new(pb.Candidate)
	if err := proto.Unmarshal(bytes, pbCandidate); err != nil {
		return nil, err
	}
	return FromProto(pbCandidate), nil
}

//SetVotePower sets the votePower
func (c *Candidate) SetVotePower(VP int64) {
	c.VotePower = VP
}

//SetBannedHeight sets the bannedtill to the given height
func (c *Candidate) SetBannedHeight(h int64) {
	c.bannedTill = h
}
