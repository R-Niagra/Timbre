package services

import (
	"context"
	"errors"

	"github.com/guyu96/go-timbre/core"
	"github.com/guyu96/go-timbre/log"
	"github.com/guyu96/go-timbre/net"
	"github.com/guyu96/go-timbre/roles"
)

var (
	voteToSend = 100
)

//DoVote votes the candidate
func (s *Services) DoVote(ctx context.Context, pk string, miner *roles.Miner) error {
	node := s.node
	peerAdd := pk

	myVote := "+20|" + peerAdd //TODO:ChangeToPK

	log.Info().Msgf("Recepient of vote tx: ", peerAdd)
	_, err := node.Dpos.State.GetCandidate(peerAdd) //To check if the address is already a candidate
	if err != nil {
		log.Info().Msgf(err.Error())
	}

	var votes []string
	votes = append(votes, myVote)

	voteTrans, _ := core.NewVote(node.PublicKey().String(), node.PrivateKey(), votes, s.node.TransactionNonce)

	s.node.TransactionNonce++

	protoVote := voteTrans.GetProtoTransaction()
	voteBytes, err := voteTrans.ToBytes()
	if err != nil {
		return err
	}
	msg := net.NewMessage(net.MsgTx, voteBytes)
	var minerAdd []string

	if miner != nil {
		miner.PushTransaction(protoVote)
		log.Info().Msgf("My vote added in miner cache")
	}
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

//DoUnVote unvotes the guy previously voted
func (s *Services) DoUnVote(ctx context.Context, pk string, miner *roles.Miner) error {
	node := s.node
	peerAdd := pk

	myVote := "-20|" + peerAdd //Fixed 20 percent unvote

	log.Info().Msgf("Recepient of vote tx: ", peerAdd)
	_, err := node.Dpos.State.GetCandidate(peerAdd) //To check if the address is already a candidate
	if err != nil {
		log.Info().Msgf(err.Error())
	}

	var votes []string
	votes = append(votes, myVote)

	voteTrans, _ := core.NewVote(node.PublicKey().String(), node.PrivateKey(), votes, s.node.TransactionNonce)
	s.node.TransactionNonce++

	protoVote := voteTrans.GetProtoTransaction()
	voteBytes, err := voteTrans.ToBytes()
	if err != nil {
		return err
	}
	msg := net.NewMessage(net.MsgTx, voteBytes)
	var minerAdd []string

	if miner != nil {
		miner.PushTransaction(protoVote)
		log.Info().Msgf("My vote added in miner cache")
	}
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

//DoPercentVote votes a candidate with the given percentage
func (s *Services) DoPercentVoteDuplicate(ctx context.Context, pk string, percent string) error {
	node := s.node
	peerAdd := pk
	myVote := "+" + percent + "|" + peerAdd

	log.Info().Msgf("Recepient of vote tx: ", peerAdd)
	_, err := node.Dpos.State.GetCandidate(peerAdd) //To check if the address is already a candidate
	if err != nil {
		log.Info().Msgf(err.Error())
	}

	var votes []string
	votes = append(votes, myVote)

	voteTrans, _ := core.NewVote(node.PublicKey().String(), node.PrivateKey(), votes, s.node.TransactionNonce)
	s.node.TransactionNonce++
	voteBytes, err := voteTrans.ToBytes()
	if err != nil {
		return err
	}
	msg := net.NewMessage(net.MsgTx, voteBytes)
	var minerAdd []string

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

//DoPercentVote votes a candidate with the given percentage
func (s *Services) DoPercentVote(ctx context.Context, pk string, percent string) error {
	node := s.node
	peerAdd := pk
	myVote := "+" + percent + "|" + peerAdd

	log.Info().Msgf("Recepient of vote tx: ", peerAdd)
	_, err := node.Dpos.State.GetCandidate(peerAdd) //To check if the address is already a candidate
	if err != nil {
		log.Info().Msgf(err.Error())
	}

	var votes []string
	votes = append(votes, myVote)
	voteTrans, _ := core.NewVote(node.PublicKey().String(), node.PrivateKey(), votes, s.node.TransactionNonce)

	s.node.TransactionNonce++

	protoVote := voteTrans.GetProtoTransaction()
	voteBytes, err := voteTrans.ToBytes()
	if err != nil {
		return err
	}
	msg := net.NewMessage(net.MsgTx, voteBytes)
	var minerAdd []string

	if node.Miner != nil {
		node.Miner.PushTransaction(protoVote)
		log.Info().Msgf("My vote added in miner cache")
	}
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

// DoPercentUnVote unvotes by given percentage
func (s *Services) DoPercentUnVote(ctx context.Context, pk string, percent string) error {
	node := s.node
	peerAdd := pk

	myVote := "-" + percent + "|" + peerAdd //Fixed 20 percent unvote

	log.Info().Msgf("Recepient of vote tx: %v", peerAdd)
	_, err := node.Dpos.State.GetCandidate(peerAdd) //To check if the address is already a candidate
	if err != nil {
		log.Info().Msgf(err.Error())
	}

	var votes []string
	votes = append(votes, myVote)

	voteTrans, _ := core.NewVote(node.PublicKey().String(), node.PrivateKey(), votes, s.node.TransactionNonce)

	s.node.TransactionNonce++

	protoVote := voteTrans.GetProtoTransaction()
	voteBytes, err := voteTrans.ToBytes()
	if err != nil {
		return err
	}
	msg := net.NewMessage(net.MsgTx, voteBytes)
	var minerAdd []string

	if node.Miner != nil {
		node.Miner.PushTransaction(protoVote)
		log.Info().Msgf("My vote added in miner cache")
	}
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
