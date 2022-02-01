package services

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/guyu96/go-timbre/log"
	"github.com/guyu96/go-timbre/net"
	"github.com/mbilal92/noise"
)

//RelayMsgToAddresses relays the msg to the given addresses
func (s *Services) RelayMsgToAddresses(minerAdds []string, msg net.Message) error {

	myNodeAdd := s.node.PublicKey().String()
	for _, add := range minerAdds {
		if myNodeAdd == add { //Can't relay msg to myself
			continue
		}

		pk, err := hex.DecodeString(add)
		if err != nil {
			log.Info().Err(errors.New("Error decoding the string. Can't relay msg"))
		}
		var pbk noise.PublicKey
		copy(pbk[:], pk)
		s.node.RelayToPB(pbk, msg)
	}
	log.Info().Msgf("tx has been relayed!!!")
	return nil
}

//RelayMsgToMiners asks about the miners address and relay message to the miners
func (s *Services) RelayMsgToMiners(msg net.Message) error {
	node := s.node
	var minerAdd []string
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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
		log.Info().Msgf("Unable to fetch the miner addresses within the time limit")
		fmt.Println("Broad-casting msg now")
		s.node.Broadcast(msg)
		return nil
	}

	err := s.RelayMsgToAddresses(minerAdd, msg)
	if err != nil {
		return err
	}
	return nil
}
