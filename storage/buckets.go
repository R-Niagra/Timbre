package storage

var (
	NodeBucket                   []byte
	FullNodeStateBucket          []byte // Full node state bucket
	DealBucket                   []byte // Full node state bucket
	LastChVerificationTimeBucket []byte
	PostStoredBucket             []byte
	PostBucket                   []byte
	DealTagBucket                []byte
	LocalPostBucket              []byte
	UserBucket                   []byte
	BlockStateBucket             []byte
	BlockBucket                  []byte
)

func InitVariables(blockchainName string) {
	blockchainName = blockchainName + "-"
	NodeBucket = []byte(blockchainName + "node-bucket")
	FullNodeStateBucket = []byte(blockchainName + "full-state")
	DealBucket = []byte(blockchainName + "deal-bucket")
	LastChVerificationTimeBucket = []byte(blockchainName + "LastChVerificationTime-bucket")
	PostStoredBucket = []byte(blockchainName + "postStored-bucket")
	PostBucket = []byte(blockchainName + "post-bucket")
	DealTagBucket = []byte(blockchainName + "deal-tag-bucket")
	LocalPostBucket = []byte(blockchainName + "local-post-bucket")
	UserBucket = []byte(blockchainName + "user-bucket")
	BlockStateBucket = []byte(blockchainName + "Block_state")
	BlockBucket = []byte(blockchainName + "block-bucket")
}
