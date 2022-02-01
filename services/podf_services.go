package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/guyu96/go-timbre/core"
	"github.com/guyu96/go-timbre/log"
	"github.com/guyu96/go-timbre/net"
	pb "github.com/guyu96/go-timbre/protobuf"
)

//CreatePodf creates a podf transaction
func (s *Services) CreatePodf(ctx context.Context, h1 *pb.BlockHeader, h2 *pb.BlockHeader) error {
	fmt.Println("Creating podf...")
	node := s.node
	myPk := node.PublicKey().String() //My public key

	newPodf, err := core.NewPodf(h1, h2, s.node.TransactionNonce, myPk, node.PrivateKey())
	if err != nil {
		return err
	}
	s.node.TransactionNonce++

	protoPodf := newPodf.GetProtoTransaction()
	fmt.Println("validating podf")
	err = node.Wm.ValidatePodf(protoPodf, s.node.Dpos.GetRoundNum())
	if err != nil {
		fmt.Println("err is: ", err.Error())
		return err
	}

	podfBytes, err := newPodf.ToBytes()
	if err != nil {
		return err
	}
	msg := net.NewMessage(net.MsgTx, podfBytes)

	var minerAdd []string

	if node.Miner != nil {
		node.Miner.PushTransaction(protoPodf)
		fmt.Println("Podf added in the miner cache")
		log.Info().Msgf("Podf added in the miner cache")
	} else {
		fmt.Println("Miner is nil.....")
	}

	node.PendingTrans++
	if s.node.PendingTrans > 1 { //If not first transaction
		s.node.QuitNonceUpChan <- true
	}
	go s.node.SyncTransActionNonceAfterTenSec()

	if node.Dpos.HasStarted == true { //if it is running dpos then it already know the addresses
		minerAdd = s.node.Dpos.GetMinersAddresses()
		fmt.Println("relaying msgs to address")
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
