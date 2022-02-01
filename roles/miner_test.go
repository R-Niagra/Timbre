package roles

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/guyu96/go-timbre/crypto/pos"
	"github.com/guyu96/go-timbre/net"
	"github.com/guyu96/go-timbre/protobuf"

	"github.com/guyu96/go-timbre/storage"
)

//Deep equal doesn't seems equal. Values has been manually verified.
func TestMiner_FillBlock(t *testing.T) {
	type fields struct {
		node                 *net.Node
		incoming             chan net.Message
		startedMining        bool
		soCache              *storage.LRU
		prCache              *storage.LRU
		spKadIdCache         *storage.LRU
		postToPrimarySpCache *storage.LRU
		localspReqListCache  map[string]map[string]*protobuf.ThreadPostRequestList
		msgMap               map[string][]byte
		storageLeft          map[string]uint32
		challengeCache       map[string]*pos.Chall
		challengeIssuedDeal  map[string]*protobuf.Deal
		Activedeals          map[string]*protobuf.Activedeals
		threadPostingTime    map[string]int64
		ChPrPair             []*protobuf.ChPrPair
		deals                []*protobuf.Deal
		transactions         *storage.TransactionPool
		votes                *storage.TransactionPool
		staleSoCache         *storage.LRU
		ExitBlockLoop        chan struct{}
	}

	// fmt.Println()
	aDeal := &protobuf.Deal{Timestamp: 1}
	aVote := &protobuf.Vote{TimeStamp: 2}
	aProof := &protobuf.ChPrPair{Timestamp: 3}
	aTrans := &protobuf.Transaction{TimeStamp: 4}
	// bytes, _ := proto.Marshal(deal)

	fmt.Println("deal size: ", aDeal.XXX_Size(), " a vote: ", aVote.XXX_Size(), "a proof: ", aProof.XXX_Size(), "a trans: ", aTrans.XXX_Size())

	tTransactions := storage.NewTransactionPool(prCacheSize)
	tTransactions.Push(&protobuf.Transaction{TimeStamp: 1})
	testVotes := storage.NewTransactionPool(prCacheSize)
	testVotes.Push(&protobuf.Vote{TimeStamp: 7})

	testFields := fields{
		deals: []*protobuf.Deal{
			&protobuf.Deal{
				Timestamp: 1,
			},
			&protobuf.Deal{
				Timestamp: 2,
			},
			&protobuf.Deal{
				Timestamp: 9,
			},
		},
		ChPrPair: []*protobuf.ChPrPair{
			&protobuf.ChPrPair{
				Timestamp: 3,
			},
			&protobuf.ChPrPair{
				Timestamp: 4,
			},
			&protobuf.ChPrPair{
				Timestamp: 5,
			},
		},
		votes:        testVotes,
		transactions: tTransactions,
	}

	type args struct {
		limit int64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []*protobuf.Deal
		want1  []*protobuf.ChPrPair
		want2  []*protobuf.Transaction
		want3  []*protobuf.Vote
	}{
		// TODO: Add test cases.
		{
			name:   "Ascending order sorting",
			fields: testFields,
			args: args{
				limit: 6,
			},
			want: []*protobuf.Deal{
				&protobuf.Deal{
					Timestamp: 1,
				},
				&protobuf.Deal{
					Timestamp: 2,
				},
			},
			want1: []*protobuf.ChPrPair{},
			want2: []*protobuf.Transaction{
				&protobuf.Transaction{
					TimeStamp: 1},
			},
			// want3: []*protobuf.Vote{},
		},
		{
			name:   "Less block objects",
			fields: testFields,
			args: args{
				limit: 4,
			},
			want: []*protobuf.Deal{
				&protobuf.Deal{
					Timestamp: 1,
				},
				&protobuf.Deal{
					Timestamp: 2,
				},
			},
			want1: []*protobuf.ChPrPair{},
			want2: []*protobuf.Transaction{},
			// want3: []*protobuf.Vote{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			miner := &Miner{
				node:                 tt.fields.node,
				incoming:             tt.fields.incoming,
				startedMining:        tt.fields.startedMining,
				soCache:              tt.fields.soCache,
				prCache:              tt.fields.prCache,
				spKadIdCache:         tt.fields.spKadIdCache,
				postToPrimarySpCache: tt.fields.postToPrimarySpCache,
				localspReqListCache:  tt.fields.localspReqListCache,
				msgMap:               tt.fields.msgMap,
				storageLeft:          tt.fields.storageLeft,
				challengeCache:       tt.fields.challengeCache,
				challengeIssuedDeal:  tt.fields.challengeIssuedDeal,
				Activedeals:          tt.fields.Activedeals,
				threadPostingTime:    tt.fields.threadPostingTime,
				ChPrPair:             tt.fields.ChPrPair,
				deals:                tt.fields.deals,
				transactions:         tt.fields.transactions,
				votes:                tt.fields.votes,
				staleSoCache:         tt.fields.staleSoCache,
				ExitBlockLoop:        tt.fields.ExitBlockLoop,
			}
			got, got1, got2, got3 := miner.FillBlock(tt.args.limit)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Miner.FillBlock() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("Miner.FillBlock() got1 = %v, want %v", got1, tt.want1)
			}
			if !reflect.DeepEqual(got2, tt.want2) {
				t.Errorf("Miner.FillBlock() got2 = %v, want %v", got2, tt.want2)
			}
			if !reflect.DeepEqual(got3, tt.want3) {
				t.Errorf("Miner.FillBlock() got3 = %v, want %v", got3, tt.want3)
			}
		})
	}
}
