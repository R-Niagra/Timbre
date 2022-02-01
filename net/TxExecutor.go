package net

import (
	"github.com/guyu96/go-timbre/log"
	pb "github.com/guyu96/go-timbre/protobuf"
)

//ExecuteRegisterCandidate executes the regster candidate transaction
func (n *Node) ExecuteRegisterCandidate(tx *pb.Transaction) error {
	//If successful. Insert candidate in the DPOS
	candID := tx.Body.GetSenderId()
	_, err := n.Dpos.State.RegisterCand(candID) //Treating address as ID for testing
	if err != nil {
		return err
	}
	log.Info().Msgf("Candidate has been registered. %s", candID)
	return nil
}

//ExecuteQuitCandidate executes the quit candidate transaction
func (n *Node) ExecuteQuitCandidate(tx *pb.Transaction) error {

	//Removing candidate from all linked ds
	candID := tx.Body.GetSenderId()
	err := n.Dpos.State.QuitCandidate(candID)
	if err != nil {
		return err
	}

	n.Wm.UnSetCandidacy(candID)

	return nil
}
