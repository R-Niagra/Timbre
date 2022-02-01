package core

import (
	"encoding/hex"
	"errors"

	"github.com/golang/protobuf/proto"
	pb "github.com/guyu96/go-timbre/protobuf"
	"github.com/mbilal92/noise"
)

//Vote struct to wrap protobuf Vote
type Vote struct {
	pbVote *pb.Transaction
}

//NewVote creates new vote instance
func NewVote(id string, voterPrivateKey noise.PrivateKey, votedCand []string, nonce uint64) (*Transaction, error) {

	tx := GetTransactionTemplate(id, nonce)
	payloadBytes, err := VotePayloadToByte(votedCand)
	if err != nil {
		return nil, err
	}
	tx.pbTransaction.Body.Payload = payloadBytes
	tx.pbTransaction.Type = TxVote
	payerSign, err := SignTransaction(tx.pbTransaction.Body, voterPrivateKey)
	if err != nil {
		return nil, err
	}
	tx.pbTransaction.PayerSig = payerSign
	return tx, nil
}

//VotePayloadToByte converts the vote payload to byte
func VotePayloadToByte(votedCand []string) ([]byte, error) {

	votePayload := &pb.VotePayload{
		VoteCand: votedCand,
	}
	return proto.Marshal(votePayload)
}

//VotePayloadFromByte is the payload of the vote
func VotePayloadFromByte(payload []byte) ([]string, error) {

	votePayload := new(pb.VotePayload)
	if err := proto.Unmarshal(payload, votePayload); err != nil {
		return nil, err
	}
	return votePayload.VoteCand, nil
}

//ValidlySignedVote checks if the vote has the valid sign
func ValidlySignedVote(v *pb.Vote) (bool, error) {
	tmpSig := make([]byte, len(v.GetVoterSig()))
	copy(tmpSig, v.GetVoterSig())
	v.VoterSig = nil //Assigning signature nil

	defer func() {
		v.VoterSig = tmpSig
	}()

	pk, err := hex.DecodeString(v.GetId())
	if err != nil {
		return false, err
	}
	publicKey := ByteToPublicKey(pk)

	m, err := proto.Marshal(v)
	if err != nil {
		return false, err
	}

	valid := publicKey.VerifyB(m, tmpSig)
	if !valid {
		return false, errors.New("Failed vote validation")
	}

	return true, nil
}

//GetProtoVote returns the protoVote
func (v *Vote) GetProtoVote() *pb.Transaction {
	return v.pbVote
}

//ToBytes convert the vote to bytes
func (v *Vote) ToBytes() ([]byte, error) {
	return proto.Marshal(v.pbVote)
}

//VoteFromBytes convert the bytes back to vote object
func VoteFromBytes(voteBytes []byte) (*Vote, error) {
	vote := new(Vote)
	vote.pbVote = new(pb.Transaction)
	if err := proto.Unmarshal(voteBytes, vote.pbVote); err != nil {
		return nil, err
	}
	return vote, nil
}

//GetID returns the id of the vote
func (v *Vote) GetID() string {
	return v.pbVote.Body.GetSenderId()
}

//ResourceUtilized returns the resource used object
func (v *Vote) ResourceUtilized() *ResourceUsage {
	voteSize, _ := v.ToBytes()
	gasBurnt := &ResourceUsage{
		cpuUsage: 10,
		netUsage: int64(len(voteSize)),
	}
	return gasBurnt
}

//DeepCopyVotes deep copies vote in another array
func DeepCopyVotes(votes []*pb.Vote) []*pb.Vote {
	copy := make([]*pb.Vote, len(votes))
	for i, vote := range votes {
		var copied pb.Vote = *vote
		copy[i] = &copied
	}
	return copy
}
