package services

import (
	"github.com/guyu96/go-timbre/net"
)

const (
	servChanSize = 32
)

//Services will handles services related to node
type Services struct {
	node       *net.Node
	incoming   chan net.Message
	minersInfo chan []byte
}

//NewServices creates the new Services instance
func NewServices(node *net.Node) *Services {
	s := &Services{
		node:       node,
		incoming:   make(chan net.Message, servChanSize),
		minersInfo: make(chan []byte),
	}
	node.AddOutgoingChan(s.incoming)
	return s
}

//Process processes the service events
func (s *Services) Process() {
	for msg := range s.incoming {
		switch msg.Code {
		case net.MsgCandidateRegisteration: //Candidate registeration msg for testing purpose
			s.HandleRegisterCandidate(msg)
		case net.TestMsg:
		case net.MsgMinersAsk:
			s.handleMinersAsk(msg)
		case net.MsgReplyToMinersAsk:
			s.handleReplyToMinersAsk(msg)
			// s.handleWriteToDownload(msg)
		}
	}
}
