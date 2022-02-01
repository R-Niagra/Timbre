package net

import (
	"context"
	"net/http"

	"github.com/guyu96/go-timbre/core"
	"github.com/guyu96/go-timbre/dpos"
	"github.com/guyu96/go-timbre/protobuf"
	rpcpb "github.com/guyu96/go-timbre/rpc/pb"
)

//Services is the interface to access services funcs accross roles
type Services interface {
	Process()
	AnnounceAndRegister()
	HandleThreads(w http.ResponseWriter, r *http.Request)
	StartDpos() error
	StartDpos2(comSize int, minerRole bool) error
	UpdateDpos(*core.Block, *dpos.Dpos) error
	AnnounceCandidacy() error
	StartExplorer(pno string)
	DoDelegateRegisteration() (*core.Transaction, error)
	DoDelegateQuit() error
	TimeLeftInRoundBySlots(b *core.Block) int64
	DoAmountTrans(ctx context.Context, address string, txAmount uint64) error
	DoPercentVote(ctx context.Context, pk string, percent string) error
	DoPercentUnVote(ctx context.Context, pk string, percent string) error
	DoPercentVoteDuplicate(ctx context.Context, pk string, percent string) error
	CreatePodf(ctx context.Context, h1 *protobuf.BlockHeader, h2 *protobuf.BlockHeader) error
}

// Interfaces for roles inorder to give mutual access to roles
// There is a seperate role for each inorder to avoid empty functions in other roles
type BaseRole interface {
	Setup()
	Process()
}

//User is the interface against the user role
type User interface {
	BaseRole
	SendPostRequest(content, parentPostHash, threadheadPostHash []byte, maxCost, minDuration uint32, postAuhtorPk []byte) string
	GetStoredP() []*protobuf.StoredPost
	GetThread(a string)
	GetThreads()
	GetTs() [][]byte
	GetThreadRoots() []string
	GetThreadFromCache(key string) ([]*rpcpb.Post, error)
	UpdateLikes(hash string, votes uint32)
}

//StorageProvider is the interface against the storage-provider role
type StorageProvider interface {
	Process()
	SendStorageOffer(minPrice, maxDuration, size uint32)
	IssueChallenges(seed int64)
	EmptyCaches()
	// GetSpBlockTickChan() chan struct{}
	FilterMyDeals(deals []*protobuf.Deal)
	ProcessNewBlock(b *core.Block)
	GetStorageProviderStats() *rpcpb.StorageProviderStats
	RemoveExpiredDealDataFromCahce(deal *protobuf.Deal)
}

//Syncer is the interface against the syncer role
type Syncer interface {
	Process()
	GetIsSyncing() bool
	SyncBlockchain()
	PrintBlockChain()
	// TimeLeftInRound(block *core.Block) (bool, float64)
	// RoundStartTime(block *core.Block) int64
	SetIsSyncing(flag bool)
}

//BandwidthProvider is the interface against the Bandwidth-Provider role
type BandwidthProvider interface {
	BaseRole
	UpdateThreadBase()
	GetThreads()
}

//Miner is the interface against the miner role
type Miner interface {
	// BaseRole
	Process(context.Context)
	TestRelayToSp()
	PrintPostStats()
	CreateBlockLoop()
	PushTransaction(pbTrans *protobuf.Transaction)
	StartMiner()
	UpdateCache(block *core.Block)
	EmptyCaches()
	RemoveExpiredDealDataFromCahce(deal *protobuf.Deal)
}
