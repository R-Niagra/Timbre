package services

import (
	"time"

	"github.com/guyu96/go-timbre/log"
	"github.com/guyu96/go-timbre/net"
)

func (s *Services) handleMinersAsk(msg net.Message) {
	if s.node.Dpos.HasStarted == true {
		miners, err := s.node.Dpos.MinersToBytes()
		if err != nil {
			return
		}
		reply := net.NewMessage(net.MsgReplyToMinersAsk, miners)
		s.node.Relay(msg.From, reply)
	}

}

func (s *Services) handleReplyToMinersAsk(msg net.Message) {
	select {
	case s.minersInfo <- msg.Data:
		//Will relay message to the func who asked for miners. That func should be recieving over minersInfo
	default:
		return
	}
}

//AskMinersAdd sends a ask message for the addresses of the miners
func (s *Services) AskMinersAdd() {

	msg := net.NewMessage(net.MsgMinersAsk, []byte("MinerAddressesRequested"))
	s.node.Broadcast(msg)
}

//AnnounceAndRegister make miner announcement if it's genesis round.
func (s *Services) AnnounceAndRegister() {

	isRevoltingTip := s.node.Bc.IsRevoltingTip()

	if s.node.Bc.IsEmpty() || isRevoltingTip { //Only register without transaction if it hasn't synced up
		log.Info().Msgf("Announcing reg-Cand !")
		go func() {
			for i := 0; i < 3; i++ { //Announcing candidacy 3 times
				s.AnnounceCandidacy()

				time.Sleep(1 * time.Second)
			}
			log.Info().Msgf("Announcement done. Sending official dr registration tx")
			s.DoDelegateRegisteration() //Also adding the transaction to establish as a ground truth- making it official

		}()

	} else {
		log.Info().Msg("Sending delegate reg-tx only")
		s.DoDelegateRegisteration()
	}
}
