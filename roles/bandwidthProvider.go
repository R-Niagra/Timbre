package roles

import (
	"github.com/guyu96/go-timbre/net"
	"github.com/guyu96/go-timbre/storage"
)

const (
	bpChanSize    = 32  // default BandwidthProvider channel buffer size
	postCacheSize = 128 // default post cache size
)

// All post retrievals go here
// BandwidthProvider
type BandwidthProvider struct {
	node     *net.Node
	incoming chan net.Message

	postCache  *storage.LRU      // Post Cache
	postToSp   map[string]string // posthash -> sp_kad_id
	threadBase map[string]([]string)
}
