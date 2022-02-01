package net

import (
	"github.com/mbilal92/noise"
	"github.com/mbilal92/noise/broadcast"
	"github.com/mbilal92/noise/relay"
)

// The exported constants are codes for different types of messages.
const (
	MsgCodeBlockNew           = byte(0)
	MsgCodeBlockAsk           = byte(1)
	MsgCodeBlockSync          = byte(2)
	MsgCandidateRegisteration = byte(3) //For testing purpose only
	// MsgVoteTx                 = byte(4)
	MsgTx                    = byte(5)
	TestMsg                  = byte(6)
	MsgMinersAsk             = byte(7) //For asking the set of miners in the current round
	MsgReplyToMinersAsk      = byte(8)
	MsgCodeBlockAskByHeight  = byte(9)
	MsgCodeBlockSyncByHeight = byte(10)

	MsgCodeStorageOfferNew                   = byte(11)
	MsgCodeStorageProviderRecievePostRequest = byte(12)
	MsgCodeStorageProviderPostStored         = byte(13)
	MsgCodeStorageProviderPostNotStored      = byte(14) // TODO: incase of no space avaiable  Penalized sp

	MsgCodePostNew                   = byte(21)
	MsgCodePostStored                = byte(22)
	MsgCodeGetPost                   = byte(23)
	MsgCodePostContent               = byte(24)
	MsgCodePostThreadExpired         = byte(25)
	MsgCodePostNotStored             = byte(26)
	MsgCodeInsufficientBalanceToPost = byte(27)
	MsgCodeUpdateLikes               = byte(28)

	MsgCodeUserPostResponse = byte(31)

	MsgCodeReceiveVarificationTags = byte(41)

	MsgCodeIssueChallenge   = byte(51)
	MsgCodeSpChallengeCall  = byte(52)
	MsgCodeSpChallengeProf  = byte(53)
	MsgCodeSpChallengeProf2 = byte(54)

	MsgCodeGetModList = byte(61)
	MsgCodeModPosts   = byte(62)

	MsgCodeGetCurrentTail = byte(71)
	MsgCodeCurrentTail    = byte(72)

	MsgCodeRelayTest     = byte(81)
	MsgCodeBroadcastTest = byte(82)

	MsgCodeBlockChainLongSyncResquest = byte(91)
	MsgCodeBlockChainLongSyncResponse = byte(92)

	MsgCodeAskBlockByHash   = byte(93)
	MsgCodeReplyBlockByHash = byte(94)
)

// Message is the message struct being transmitted over the p2p network.
type Message struct {
	Code byte     // Code indicates message type so that Data can be correctly deserialized.
	Data []byte   // Data is the serialized bytes of message payload.
	From noise.ID // Incase of incoming msg, need to know the kadID to reply
}

// NewMessage creates a new message.
func NewMessage(code byte, data []byte) Message {
	return Message{Code: code, Data: data}
}

// NewMessageFromRelay creates a new message from a received relay message.
func NewMessageFromRelay(msg relay.Message) Message {
	return Message{Code: msg.Code, Data: msg.Data, From: msg.From}
}

// NewMessageFromBroadcast creates a new message from a received broadcast message.
func NewMessageFromBroadcast(msg broadcast.Message) Message {
	return Message{Code: msg.Code, Data: msg.Data, From: msg.From}
}
