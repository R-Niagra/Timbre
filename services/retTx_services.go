package services

import (
	"context"
	"errors"

	"github.com/guyu96/go-timbre/core"
	"github.com/guyu96/go-timbre/log"
	"github.com/guyu96/go-timbre/net"
)

//DoAmountTrans creates a retrieval transaction
func (s *Services) DoAmountTrans(ctx context.Context, pk string, txAmount uint64) error {

	node := s.node
	peerAdd := pk

	log.Info().Msgf("Recepient of amount tx: ", peerAdd)
	testTrans, err := core.MakeTestTransaction(node.PublicKey().String(), peerAdd, int64(txAmount), s.node.TransactionNonce, node.PrivateKey()) //hex encoded strings converted to byte
	if err != nil {
		return err
	}
	s.node.TransactionNonce++

	err = node.Wm.MyWallet.ValidateTransactionAmount(testTrans)
	if err != nil {
		return err
	}

	protoTrans := testTrans.GetProtoTransaction()
	transBytes, err := testTrans.ToBytes()
	if err != nil {
		return err
	}
	msg := net.NewMessage(net.MsgTx, transBytes)
	var minerAdd []string

	if node.Miner != nil {
		node.Miner.PushTransaction(protoTrans)
		log.Info().Msgf("Ret trans added in miner cache")
	}

	node.PendingTrans++
	if s.node.PendingTrans > 1 { //If not first transaction
		s.node.QuitNonceUpChan <- true
	}
	go s.node.SyncTransActionNonceAfterTenSec()

	if node.Dpos.HasStarted == true { //if it is running dpos then it already know the addresses
		minerAdd = s.node.Dpos.GetMinersAddresses()
		err := s.RelayMsgToAddresses(minerAdd, msg)
		return err
	}

	s.AskMinersAdd()

	select {
	case miners := <-s.minersInfo:
		currentMiners, err := s.node.Dpos.MinersFromBytes(miners)
		if err != nil {
			return err
		}
		minerAdd = currentMiners.GetMinerAddresses()
	case <-ctx.Done():
		return errors.New("Unable to fetch the miner addresses within the time limit")
	}

	err = s.RelayMsgToAddresses(minerAdd, msg)
	if err != nil {
		return err
	}

	return nil
}

//DoDelegateRegisteration creates a delagate registration transaction and broadcasts it
func (s *Services) DoDelegateRegisteration() (*core.Transaction, error) {
	node := s.node

	tx, err := core.RegisterCandidateTx(node.PublicKey().String(), node.PrivateKey(), s.node.TransactionNonce)
	if err != nil {
		return nil, err
	}
	s.node.TransactionNonce++
	protoTrans := tx.GetProtoTransaction()
	transBytes, err := tx.ToBytes()
	if err != nil {
		return nil, err
	}
	msg := net.NewMessage(net.MsgTx, transBytes)
	var minerAdd []string

	if node.Miner != nil {
		node.Miner.PushTransaction(protoTrans)
		log.Info().Msgf("Delegate register added in miner cache")
	}
	if node.Dpos.HasStarted == true && node.Dpos.GetRoundNum() > 0 { //if it is running dpos then it already know the addresses
		minerAdd = s.node.Dpos.GetMinersAddresses()
		err := s.RelayMsgToAddresses(minerAdd, msg)
		return tx, err
	}
	if len(minerAdd) == 0 {
		s.node.Broadcast(msg)
		log.Info().Msgf("Don't know miner Add-> Broadcasted")
		return tx, nil
	}
	err = s.RelayMsgToAddresses(minerAdd, msg)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

//DoDelegateQuit create a delegate quit transaction and realy it to the miners
func (s *Services) DoDelegateQuit() error {
	node := s.node

	tx, err := core.QuitCandidateTx(node.PublicKey().String(), node.PrivateKey(), s.node.TransactionNonce)
	if err != nil {
		return err
	}
	s.node.TransactionNonce++
	protoTrans := tx.GetProtoTransaction()
	transBytes, err := tx.ToBytes()
	if err != nil {
		return err
	}
	msg := net.NewMessage(net.MsgTx, transBytes)
	var minerAdd []string

	if node.Miner != nil {
		node.Miner.PushTransaction(protoTrans)
		log.Info().Msgf("Quit delegate Tx added in miner cache")
	}
	if node.Dpos.HasStarted == true { //if it is running dpos then it already know the addresses
		minerAdd = s.node.Dpos.GetMinersAddresses()
		err := s.RelayMsgToAddresses(minerAdd, msg)
		return err
	}
	if len(minerAdd) == 0 {
		s.node.Broadcast(msg)
		log.Info().Msgf("Don't know miner Add-> Broadcasting")
		return nil
	}
	err = s.RelayMsgToAddresses(minerAdd, msg)
	if err != nil {
		return err
	}

	return nil
}
