package core

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/guyu96/go-timbre/log"
)

func init() {
	DefaultConfig.RevoltBlockHash, _ = hex.DecodeString("e9c1d7ee3a6ac1489f3c83cad87c11d2f9a3629d87d870d52ec42d419f7cced9")

}

const (
	//TxTransfer is corrosponding to the retrieval transaction
	TxTransfer = "Transfer"
	//TxBecomeCandidate is for announcing the intention to become delegate
	TxBecomeCandidate = "Become_Candidate"
	//TxQuitCandidate is for announcing to quit from being candidate
	TxQuitCandidate = "Quit_Transaction"
	//TxVote the vote tx
	TxVote = "Vote"
	//TxPodf is the tx for the proof of double forgery
	TxPodf = "Podf"
)

//Epoch entatils the feature requirement to run an epoch on the node
type Epoch struct {
	Duration           int64 //Duration (int) represents the height of the block after whichh epoch should end
	RollBackPercentage int   //After an epoch ends the rollback percentage is the amount the user will be able to claim back
	Inflation          int   //Percentage inflation after an epoch
	EpochNum           int32
}

//TestEpoch is an epoch config we will be using for testing
var TestEpoch *Epoch = &Epoch{
	Duration:           1000,
	RollBackPercentage: 100,
	Inflation:          0,
}

//NodeConfig contains the configuration param for running the node
type NodeConfig struct {
	ChainID         string
	Delimiter       string
	Revolt          bool
	RevoltHeight    int64  //height from where new chain will start
	RevoltBlockHash []byte //Block hash at the provided height
}

//DefaultConfig is defult configuration used by the node
var DefaultConfig *NodeConfig = &NodeConfig{
	ChainID:      "1",
	Delimiter:    ".",
	Revolt:       false,
	RevoltHeight: 12,
}

//GetParentChainID returns the chain id of the parent e.g 1 is the parent of 1.1 chainID
func (nc *NodeConfig) GetParentChainID() (string, error) {
	lastDelIndex := strings.LastIndex(nc.ChainID, nc.Delimiter) //Finding the last occurance of the delimiter
	if lastDelIndex < 0 {
		log.Error().Msgf("Wrong chain-id. Plz check ur chain-id")
		return "", fmt.Errorf("Wrong chain-id. Plz check ur chain-id")
	}

	parentID := nc.ChainID[:lastDelIndex]
	fmt.Println("Parent id is: ", parentID)
	return parentID, nil
}
