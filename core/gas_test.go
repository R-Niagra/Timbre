package core

// import (
// 	"reflect"
// 	"testing"
// 	"time"

// 	"github.com/golang/protobuf/proto"
// 	pb "github.com/guyu96/go-timbre/protobuf"
// 	// kad "github.com/guyu96/noise/skademlia"
// )

// func TestResourcesRequired(t *testing.T) {
// 	var keys net.Keypair
// 	keys.PublicKey, keys.PrivateKey, err = noise.GenerateKeys(nil)
// 	// keys := kad.RandomKeys()
// 	voterAdd := "TestVoter"

// 	vote1 := "+20|127.0.0.1:9000" //Creating a test votes transaction
// 	var votes []string
// 	votes = append(votes, vote1)
// 	voteTrans, _ := NewVote(voterAdd, map[string]int64{}, keys.PrivateKey, votes)
// 	pbVote := voteTrans.GetProtoVote()
// 	voteBytes, _ := voteTrans.ToBytes()
// 	netBytes := len(voteBytes)

// 	//Creating a test retrieval transaction
// 	testTrans, _ := MakeTestTransaction([]byte("TestNode"), []byte("127.0.0.1:9000"), uint64(100), 111, keys.PrivateKey)
// 	pbTrans := testTrans.GetProtoTransaction()
// 	transBytes, _ := testTrans.ToBytes()
// 	transNetBytes := len(transBytes)

// 	//Creating a test deal
// 	deal := &pb.Deal{
// 		Spid:       []byte("Test"),
// 		KadID:      []byte("Test"),
// 		SpSig:      []byte("Test"),
// 		List:       nil,
// 		PublicKey:  nil,
// 		Name:       []byte("Test"),
// 		PosParam:   "Test",
// 		Timestamp:  time.Now().UnixNano(),
// 		ExpiryTime: time.Now().UnixNano() + 1000000,
// 	}

// 	dealBytes, _ := proto.Marshal(deal)
// 	dealNetUsage := uint64(len(dealBytes))

// 	// Creating a sample ChPrpair
// 	chPrPair := &pb.ChPrPair{
// 		Challenge:      nil,
// 		Proof:          nil,
// 		Sig:            nil,
// 		Spid:           []byte("Test"),
// 		Dealhash:       []byte("Test"),
// 		DealExpiryTime: time.Now().UnixNano(),
// 		Timestamp:      time.Now().UnixNano(),
// 		DealStartTime:  time.Now().UnixNano() + 1000000,
// 	}

// 	proofBytes, _ := proto.Marshal(chPrPair)
// 	proofNetUsage := uint64(len(proofBytes))

// 	type args struct {
// 		tx interface{}
// 	}
// 	tests := []struct {
// 		name string
// 		args args
// 		want *ResourceUsage
// 	}{
// 		// TODO: Add furthur test cases if required.
// 		{
// 			name: "Vote Trans",
// 			args: args{
// 				tx: pbVote,
// 			},
// 			want: &ResourceUsage{
// 				cpuUsage: 10,
// 				netUsage: uint64(netBytes),
// 			},
// 		},
// 		{
// 			name: "Retrieval transaction",
// 			args: args{
// 				tx: pbTrans,
// 			},
// 			want: &ResourceUsage{
// 				cpuUsage: 10,
// 				netUsage: uint64(transNetBytes),
// 			},
// 		},
// 		{
// 			name: "Deal Transaction",
// 			args: args{
// 				tx: deal,
// 			},
// 			want: &ResourceUsage{
// 				cpuUsage: 15,
// 				netUsage: dealNetUsage,
// 			},
// 		},
// 		{
// 			name: "Challenge proof transaction",
// 			args: args{
// 				tx: chPrPair,
// 			},
// 			want: &ResourceUsage{
// 				cpuUsage: 12,
// 				netUsage: proofNetUsage,
// 			},
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			if got := ResourcesRequired(tt.args.tx); !reflect.DeepEqual(got, tt.want) {
// 				t.Errorf("ResourcesReq() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }
