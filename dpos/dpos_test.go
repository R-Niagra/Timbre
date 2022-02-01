package dpos

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	dposstate "github.com/guyu96/go-timbre/dpos/state"
	"github.com/guyu96/go-timbre/wallet"
	"github.com/stretchr/testify/assert"
)

func TestDpos_getCandidatesFromFile(t *testing.T) {

	type fields struct {
		committeeSize int
		HasStarted    bool
		committee     []*dposstate.Candidate
		quitCh        chan int
		roundNumber   int64
		State         *dposstate.State
	}
	type args struct {
		filePath string
	}
	// testCom:=make(map[string]*dposstate.Candidate)
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "Testing to check if parsing from txt file is correctly happening",
			fields: fields{
				committeeSize: 3,
				HasStarted:    true,
				roundNumber:   0,
			},
			args: args{
				filePath: "bootstrap_committee.txt",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dpos{
				committeeSize: uint32(tt.fields.committeeSize),
				HasStarted:    tt.fields.HasStarted,
				committee:     tt.fields.committee,
				roundNumber:   tt.fields.roundNumber,
				State:         tt.fields.State,
			}
			if err := d.getCandidatesFromFile(tt.args.filePath); (err != nil) != tt.wantErr {
				t.Errorf("Dpos.getCandidatesFromFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			for key, value := range d.committee {
				fmt.Println("Key is:", key, "Value:", value)
			}

		})
	}
}

func TestDpos_calcMinerIndUsingRoundTs(t *testing.T) {
	type fields struct {
		committeeSize      uint32
		HasStarted         bool
		committee          []*dposstate.Candidate
		quitCh             chan int
		roundNumber        int64
		State              *dposstate.State
		startRound         chan uint32
		roundEnded         chan struct{}
		MiningSignal       chan struct{}
		ListeningSignal    chan struct{}
		CreateBlock        chan int64
		curMinerAdd        string
		nodeAddress        string
		TimeOffset         time.Duration
		roundTimeOffset    float64
		firstBlockProd     bool
		MinersByRound      map[int64][]string
		SlotsPassedByRound map[int64]int64
		wallets            wallet.WalletAccess
		bcHeight           int64
		roundStartTime     int64
	}
	type args struct {
		ts int64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   int
	}{
		// TODO: Add test cases.
		{
			name: "Test-1",
			fields: fields{
				roundStartTime: 11,
				committeeSize:  3,
			},
			args: args{
				ts: 20,
			},
			want: 0,
		},
		{
			name: "Test-2",
			fields: fields{
				roundStartTime: 10,
				committeeSize:  3,
			},
			args: args{
				ts: 20,
			},
			want: 1,
		},
		{
			name: "Test-3",
			fields: fields{
				roundStartTime: 5,
				committeeSize:  3,
			},
			args: args{
				ts: 15,
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dpos{
				committeeSize:   tt.fields.committeeSize,
				HasStarted:      tt.fields.HasStarted,
				committee:       tt.fields.committee,
				roundNumber:     tt.fields.roundNumber,
				State:           tt.fields.State,
				startRound:      tt.fields.startRound,
				roundEnded:      tt.fields.roundEnded,
				MiningSignal:    tt.fields.MiningSignal,
				ListeningSignal: tt.fields.ListeningSignal,
				CreateBlock:     tt.fields.CreateBlock,
				curMinerAdd:     tt.fields.curMinerAdd,
				nodeAddress:     tt.fields.nodeAddress,
				TimeOffset:      tt.fields.TimeOffset,
				roundTimeOffset: tt.fields.roundTimeOffset,
				firstBlockProd:  tt.fields.firstBlockProd,
				// MinersByRound:      tt.fields.MinersByRound,
				// SlotsPassedByRound: tt.fields.SlotsPassedByRound,
				wallets:        tt.fields.wallets,
				bcHeight:       tt.fields.bcHeight,
				roundStartTime: tt.fields.roundStartTime,
			}
			if got := d.calcMinerIndUsingRoundTs(tt.args.ts); got != tt.want {
				t.Errorf("Dpos.calcMinerIndUsingRoundTs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDpos_AddMinerEntry(t *testing.T) {
	type fields struct {
		committeeSize              uint32
		HasStarted                 bool
		committee                  []*dposstate.Candidate
		nextRoundCommittee         []*dposstate.Candidate
		quitChT                    chan int
		quitChL                    chan int
		roundNumber                int64
		State                      *dposstate.State
		startRound                 chan uint32
		roundEnded                 chan struct{}
		MiningSignal               chan struct{}
		ListeningSignal            chan struct{}
		PrepBlockSig               chan struct{}
		CreateBlock                chan int64
		curMinerAdd                string
		nodeAddress                string
		TimeOffset                 time.Duration
		roundTimeOffset            float64
		firstBlockProd             bool
		MinersByRound              map[int64][]string
		SlotsPassedByRound         map[int64]int64
		MinerCache                 []*MinersInRound
		wallets                    wallet.WalletAccess
		bcHeight                   int64
		roundStartTime             int64
		NumberOfToMineBlockPerSlot int
		bc                         BlockchainAccess
	}
	type args struct {
		miners []string
		round  int64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
		{
			name: "With Emty Miner Cache",
			fields: fields{
				MinerCache: []*MinersInRound{},
			},
			args: args{
				miners: []string{"a"},
				round:  1,
			},
		},

		{
			name: "With one entry in the cache",
			fields: fields{
				MinerCache: []*MinersInRound{
					{
						Miners: []string{
							"a",
						},
						Round:       1,
						SlotsPassed: 1,
					},
				},
			},
			args: args{
				miners: []string{"b"},
				round:  2,
			},
		},

		{
			name: "With two entries in the cache",
			fields: fields{
				MinerCache: []*MinersInRound{
					{
						Miners: []string{
							"a",
						},
						Round:       1,
						SlotsPassed: 1,
					},
					{
						Miners: []string{
							"b",
							"c",
						},
						Round:       2,
						SlotsPassed: 3,
					},
				},
			},
			args: args{
				miners: []string{"d"},
				round:  3,
			},
		},
		{
			name: "With multiple entries in the cache",
			fields: fields{
				MinerCache: []*MinersInRound{
					{
						Miners: []string{
							"a",
						},
						Round:       1,
						SlotsPassed: 1,
					},
					{
						Miners: []string{
							"b",
							"c",
						},
						Round:       2,
						SlotsPassed: 3,
					},
					{
						Miners: []string{
							"d",
							"e",
						},
						Round:       4,
						SlotsPassed: 7,
					},
				},
			},
			args: args{
				miners: []string{"f"},
				round:  5,
			},
		},
		{
			name: "Empty miners in args with multiple entries in the cache",
			fields: fields{
				MinerCache: []*MinersInRound{
					{
						Miners: []string{
							"a",
						},
						Round:       1,
						SlotsPassed: 1,
					},
					{
						Miners: []string{
							"b",
							"c",
						},
						Round:       2,
						SlotsPassed: 3,
					},
					{
						Miners: []string{
							"d",
							"e",
						},
						Round:       4,
						SlotsPassed: 7,
					},
				},
			},
			args: args{
				miners: []string{},
				round:  5,
			},
		},
		{
			name: "Same args miners with multiple entries in the cache",
			fields: fields{
				MinerCache: []*MinersInRound{
					{
						Miners: []string{
							"a",
						},
						Round:       1,
						SlotsPassed: 1,
					},
					{
						Miners: []string{
							"b",
							"c",
						},
						Round:       2,
						SlotsPassed: 3,
					},
					{
						Miners: []string{
							"d",
							"e",
						},
						Round:       3,
						SlotsPassed: 5,
					},
				},
			},
			args: args{
				miners: []string{"d", "e"},
				round:  4,
			},
		},
		{
			name: "One diff miner in args plus multiple entries in the cache",
			fields: fields{
				MinerCache: []*MinersInRound{
					{
						Miners: []string{
							"a",
						},
						Round:       1,
						SlotsPassed: 1,
					},
					{
						Miners: []string{
							"b",
							"c",
						},
						Round:       2,
						SlotsPassed: 3,
					},
					{
						Miners: []string{
							"d",
							"e",
						},
						Round:       4,
						SlotsPassed: 7,
					},
				},
			},
			args: args{
				miners: []string{"d", "e", "f"},
				round:  5,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dpos{
				committeeSize:      tt.fields.committeeSize,
				HasStarted:         tt.fields.HasStarted,
				committee:          tt.fields.committee,
				nextRoundCommittee: tt.fields.nextRoundCommittee,
				quitChT:            tt.fields.quitChT,
				quitChL:            tt.fields.quitChL,
				roundNumber:        tt.fields.roundNumber,
				State:              tt.fields.State,
				startRound:         tt.fields.startRound,
				roundEnded:         tt.fields.roundEnded,
				MiningSignal:       tt.fields.MiningSignal,
				ListeningSignal:    tt.fields.ListeningSignal,
				PrepBlockSig:       tt.fields.PrepBlockSig,
				CreateBlock:        tt.fields.CreateBlock,
				curMinerAdd:        tt.fields.curMinerAdd,
				nodeAddress:        tt.fields.nodeAddress,
				TimeOffset:         tt.fields.TimeOffset,
				roundTimeOffset:    tt.fields.roundTimeOffset,
				firstBlockProd:     tt.fields.firstBlockProd,
				// MinersByRound:              tt.fields.MinersByRound,
				// SlotsPassedByRound:         tt.fields.SlotsPassedByRound,
				MinerCache:                 tt.fields.MinerCache,
				wallets:                    tt.fields.wallets,
				bcHeight:                   tt.fields.bcHeight,
				roundStartTime:             tt.fields.roundStartTime,
				NumberOfToMineBlockPerSlot: tt.fields.NumberOfToMineBlockPerSlot,
				bc:                         tt.fields.bc,
			}
			var prevSlotsPassed int64 = 0
			if len(tt.fields.MinerCache) > 0 {
				prevSlotsPassed = tt.fields.MinerCache[len(tt.fields.MinerCache)-1].SlotsPassed
			}
			d.AddMinerEntry(tt.args.miners, tt.args.round)

			assert.Equal(t, d.MinerCache[len(d.MinerCache)-1].Round, tt.args.round)
			if len(tt.fields.MinerCache) > 0 {
				// fmt.Println("passed: ", tt.fields.MinerCache[len(tt.fields.MinerCache)-1].SlotsPassed, int64(len(tt.args.miners)), d.MinerCache[len(d.MinerCache)-1].SlotsPassed)
				assert.Equal(t, d.MinerCache[len(d.MinerCache)-1].SlotsPassed, (prevSlotsPassed + int64(len(tt.args.miners))))
				lastRoundMiners := tt.fields.MinerCache[len(tt.fields.MinerCache)-1].Miners
				allSame := false
				argsMiners := tt.args.miners
				if len(lastRoundMiners) == len(argsMiners) {
					for i, m := range argsMiners {
						if m != lastRoundMiners[i] {
							allSame = false
							break
						}
						allSame = true
					}
				}
				if allSame { //num of entries should increment
					assert.Equal(t, len(d.MinerCache), len(tt.fields.MinerCache))
				} else { //entries shouldn't increment
					assert.Equal(t, len(d.MinerCache), len(tt.fields.MinerCache)+1)
				}

			} else {
				assert.Equal(t, d.MinerCache[len(d.MinerCache)-1].SlotsPassed, int64(len(tt.args.miners)))

			}

		})

	}
}

func TestDpos_DeleteEntry(t *testing.T) {
	type fields struct {
		committeeSize              uint32
		HasStarted                 bool
		committee                  []*dposstate.Candidate
		nextRoundCommittee         []*dposstate.Candidate
		quitChT                    chan int
		quitChL                    chan int
		roundNumber                int64
		State                      *dposstate.State
		startRound                 chan uint32
		roundEnded                 chan struct{}
		MiningSignal               chan struct{}
		ListeningSignal            chan struct{}
		PrepBlockSig               chan struct{}
		CreateBlock                chan int64
		curMinerAdd                string
		nodeAddress                string
		TimeOffset                 time.Duration
		roundTimeOffset            float64
		firstBlockProd             bool
		MinersByRound              map[int64][]string
		SlotsPassedByRound         map[int64]int64
		MinerCache                 []*MinersInRound
		wallets                    wallet.WalletAccess
		bcHeight                   int64
		roundStartTime             int64
		NumberOfToMineBlockPerSlot int
		bc                         BlockchainAccess
	}
	type args struct {
		round int64
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.

		{
			name: "Emty Miner Cache",
			fields: fields{
				MinerCache: []*MinersInRound{},
			},
			args: args{
				round: 1,
			},
			wantErr: true,
		},
		{
			name: "Round greater",
			fields: fields{
				MinerCache: []*MinersInRound{
					{
						Miners: []string{
							"a",
						},
						Round:       1,
						SlotsPassed: 1,
					},
				},
			},
			args: args{
				round: 2,
			},
			wantErr: true,
		},
		{
			name: "Valid delete req with one entry",
			fields: fields{
				MinerCache: []*MinersInRound{
					{
						Miners: []string{
							"a",
						},
						Round:       4,
						SlotsPassed: 4,
					},
				},
			},
			args: args{
				round: 4,
			},
			wantErr: false,
		},
		{
			name: "Valid delete req with two entries",
			fields: fields{
				MinerCache: []*MinersInRound{
					{
						Miners: []string{
							"a",
						},
						Round:       4,
						SlotsPassed: 4,
					},
					{
						Miners: []string{
							"a",
							"b",
						},
						Round:       5,
						SlotsPassed: 6,
					},
				},
			},
			args: args{
				round: 4,
			},
			wantErr: false,
		},
		{
			name: "Valid delete req with two entries and mul miners",
			fields: fields{
				MinerCache: []*MinersInRound{
					{
						Miners: []string{
							"a",
						},
						Round:       4,
						SlotsPassed: 4,
					},
					{
						Miners: []string{
							"a",
							"b",
							"c",
						},
						Round:       5,
						SlotsPassed: 7,
					},
				},
			},
			args: args{
				round: 2,
			},
			wantErr: false,
		},
		{
			name: "Valid delete req multiple incremental entries (mul del)",
			fields: fields{
				MinerCache: []*MinersInRound{
					{
						Miners: []string{
							"a",
						},
						Round:       1,
						SlotsPassed: 1,
					},
					{
						Miners: []string{
							"b",
						},
						Round:       2,
						SlotsPassed: 2,
					},
					{
						Miners: []string{
							"c",
						},
						Round:       3,
						SlotsPassed: 3,
					},
					{
						Miners: []string{
							"d",
						},
						Round:       4,
						SlotsPassed: 4,
					},
					{
						Miners: []string{
							"e",
						},
						Round:       5,
						SlotsPassed: 5,
					},
				},
			},
			args: args{
				round: 2,
			},
			wantErr: false,
		},
		{
			name: "Valid delete req multiple entries having greater round gaps (mul del-req)",
			fields: fields{
				MinerCache: []*MinersInRound{
					{
						Miners: []string{
							"a",
						},
						Round:       1,
						SlotsPassed: 1,
					},
					{
						Miners: []string{
							"b",
						},
						Round:       2,
						SlotsPassed: 2,
					},
					{
						Miners: []string{
							"c",
						},
						Round:       5,
						SlotsPassed: 5,
					},
					{
						Miners: []string{
							"d",
						},
						Round:       8,
						SlotsPassed: 8,
					},
					{
						Miners: []string{
							"e",
						},
						Round:       10,
						SlotsPassed: 10,
					},
				},
			},
			args: args{
				round: 4,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dpos{
				committeeSize:      tt.fields.committeeSize,
				HasStarted:         tt.fields.HasStarted,
				committee:          tt.fields.committee,
				nextRoundCommittee: tt.fields.nextRoundCommittee,
				quitChT:            tt.fields.quitChT,
				quitChL:            tt.fields.quitChL,
				roundNumber:        tt.fields.roundNumber,
				State:              tt.fields.State,
				startRound:         tt.fields.startRound,
				roundEnded:         tt.fields.roundEnded,
				MiningSignal:       tt.fields.MiningSignal,
				ListeningSignal:    tt.fields.ListeningSignal,
				PrepBlockSig:       tt.fields.PrepBlockSig,
				CreateBlock:        tt.fields.CreateBlock,
				curMinerAdd:        tt.fields.curMinerAdd,
				nodeAddress:        tt.fields.nodeAddress,
				TimeOffset:         tt.fields.TimeOffset,
				roundTimeOffset:    tt.fields.roundTimeOffset,
				firstBlockProd:     tt.fields.firstBlockProd,
				// MinersByRound:              tt.fields.MinersByRound,
				// SlotsPassedByRound:         tt.fields.SlotsPassedByRound,
				MinerCache:                 tt.fields.MinerCache,
				wallets:                    tt.fields.wallets,
				bcHeight:                   tt.fields.bcHeight,
				roundStartTime:             tt.fields.roundStartTime,
				NumberOfToMineBlockPerSlot: tt.fields.NumberOfToMineBlockPerSlot,
				bc:                         tt.fields.bc,
			}
			var prevSlotsPassed int64 = 0
			if len(tt.fields.MinerCache) > 0 {
				prevSlotsPassed = tt.fields.MinerCache[len(tt.fields.MinerCache)-1].SlotsPassed
			}
			toRetain := 0
			for _, v := range tt.fields.MinerCache {
				toRetain++
				if v.Round >= (tt.args.round - 1) {
					break
				}
			}

			if err := d.DeleteEntry(tt.args.round); (err != nil) != tt.wantErr {
				t.Errorf("Dpos.DeleteEntry() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				assert.Equal(t, d.MinerCache[len(d.MinerCache)-1].Round, tt.args.round-1)
				assert.Less(t, d.MinerCache[len(d.MinerCache)-1].SlotsPassed, prevSlotsPassed)
				fmt.Println("slots passed in tests: ", d.MinerCache[len(d.MinerCache)-1].SlotsPassed, toRetain)
				assert.Equal(t, len(d.MinerCache), toRetain)
			}

		})
	}
}

func TestDpos_MinersInRound(t *testing.T) {
	type fields struct {
		committeeSize              uint32
		HasStarted                 bool
		committee                  []*dposstate.Candidate
		nextRoundCommittee         []*dposstate.Candidate
		quitChT                    chan int
		quitChL                    chan int
		roundNumber                int64
		State                      *dposstate.State
		startRound                 chan uint32
		roundEnded                 chan struct{}
		MiningSignal               chan struct{}
		ListeningSignal            chan struct{}
		PrepBlockSig               chan struct{}
		CreateBlock                chan int64
		curMinerAdd                string
		nodeAddress                string
		TimeOffset                 time.Duration
		roundTimeOffset            float64
		firstBlockProd             bool
		MinerCache                 []*MinersInRound
		wallets                    wallet.WalletAccess
		bcHeight                   int64
		roundStartTime             int64
		NumberOfToMineBlockPerSlot int
		bc                         BlockchainAccess
	}
	type args struct {
		round int64
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dpos{
				committeeSize:              tt.fields.committeeSize,
				HasStarted:                 tt.fields.HasStarted,
				committee:                  tt.fields.committee,
				nextRoundCommittee:         tt.fields.nextRoundCommittee,
				quitChT:                    tt.fields.quitChT,
				quitChL:                    tt.fields.quitChL,
				roundNumber:                tt.fields.roundNumber,
				State:                      tt.fields.State,
				startRound:                 tt.fields.startRound,
				roundEnded:                 tt.fields.roundEnded,
				MiningSignal:               tt.fields.MiningSignal,
				ListeningSignal:            tt.fields.ListeningSignal,
				PrepBlockSig:               tt.fields.PrepBlockSig,
				CreateBlock:                tt.fields.CreateBlock,
				curMinerAdd:                tt.fields.curMinerAdd,
				nodeAddress:                tt.fields.nodeAddress,
				TimeOffset:                 tt.fields.TimeOffset,
				roundTimeOffset:            tt.fields.roundTimeOffset,
				firstBlockProd:             tt.fields.firstBlockProd,
				MinerCache:                 tt.fields.MinerCache,
				wallets:                    tt.fields.wallets,
				bcHeight:                   tt.fields.bcHeight,
				roundStartTime:             tt.fields.roundStartTime,
				NumberOfToMineBlockPerSlot: tt.fields.NumberOfToMineBlockPerSlot,
				bc:                         tt.fields.bc,
			}
			got, err := d.MinersInRound(tt.args.round)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dpos.MinersInRound() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Dpos.MinersInRound() = %v, want %v", got, tt.want)
			}
		})
	}
}
