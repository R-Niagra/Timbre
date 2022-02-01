package roles

import (
	"sync"

	"github.com/guyu96/go-timbre/net"
	// "github.com/mbilal92/pbc"
)

const (
	spChanSize   = 32         // default storage provider channel buffer size
	defTotalSize = 4294967295 // default storage size
	chunkSize    = 1000
)

// StorageProvider provides storage for posts in a Timbre forum.
type StorageProvider struct {
	node               *net.Node
	incoming           chan net.Message
	totalCapacity      uint32
	committedStorage   uint32
	usedStorage        uint32
	chunks             map[string]([]uint32) // PostID -> List of chunk sizes
	Myactivedeals      map[string]bool       //*protobuf.Deal // Active Deals from blockchain
	ActiveDealMtx      sync.RWMutex          // ActiveDeal mutex
	NewMadeDeals       map[string]bool       // Active Deals from blockchain
	ProcessNewBlockMtx sync.Mutex
}
