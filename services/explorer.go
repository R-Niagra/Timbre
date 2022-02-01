package services

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/mux"
	"github.com/guyu96/go-timbre/core"
	"github.com/guyu96/go-timbre/protobuf"
)

// Blockchain Explorer outputs blockchain as json on port 8080
// To start type 'se' on main

type pst struct {
	Hash           string
	Content        string
	ParentHash     string
	RootThreadHash string
	Timestamp      int64
}

// handleThreads
func (s *Services) HandleThreads(w http.ResponseWriter, r *http.Request) {
	s.node.User.GetThreads()
	time.Sleep(1 * time.Second)
	ts := s.node.User.GetTs()
	// var posts []byte
	var postStrings []*pst
	for _, post := range ts {

		p := &protobuf.StoredPost{}
		err := proto.Unmarshal(post, p)
		if err != nil {
			return
		}

		pk := &pst{
			Hash:           hex.EncodeToString(p.Hash),
			Content:        string(p.Content),
			ParentHash:     hex.EncodeToString(p.ParentHash),
			RootThreadHash: hex.EncodeToString(p.ThreadHeadHash),
			Timestamp:      p.GetTimestamp(),
		}

		postStrings = append(postStrings, pk)
	}

	// var threads map[string]([]*pst)
	threads := make(map[string]([]*pst))
	for _, post := range postStrings {
		if post.ParentHash == "" {
			threads[post.Hash] = append(threads[post.Hash], post)
		} else {
			threads[post.RootThreadHash] = append(threads[post.RootThreadHash], post)
		}
	}

	for _, v := range threads {
		sort.Slice(v, func(i, j int) bool {
			return v[i].Timestamp < v[j].Timestamp
		})
	}

	bytes, err := json.Marshal(threads)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Println(postStrings)
	io.WriteString(w, string(bytes))
}

// handleGetBlockchain callback to show full blockchain
func (s *Services) handleGetBlockchain(w http.ResponseWriter, r *http.Request) {
	var err error
	walk := s.node.Bc.GetMainTail()
	var blocks []*core.Block
	for {
		blocks = append(blocks, walk)
		walk, err = s.node.Bc.GetBlockByHash(walk.ParentHash())
		if err != nil {
			break
		}
	}

	bytes, err := json.Marshal(blocks)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	io.WriteString(w, string(bytes))
}

// handleGetBlockchain callback to show full blockchain
func (s *Services) handleShowStats(w http.ResponseWriter, r *http.Request) {
	var err error
	walk := s.node.Bc.GetMainTail()
	height := walk.HeightInt()
	dealCount := 0
	totalDealsSize, totalBlockSize := 0, 0
	totalTransactionsSize, totalTransactions := 0, 0
	var blocks []*core.Block
	for {
		blocks = append(blocks, walk)
		dealCount += len(walk.GetDeals())
		b, _ := walk.ToBytes()
		totalBlockSize += len(b)
		for _, deal := range walk.GetDeals() {

			bb, _ := proto.Marshal(deal)
			totalDealsSize += len(bb)
		}
		totalTransactions += len(walk.GetTransactions())
		for _, transaction := range walk.GetTransactions() {
			tt, _ := proto.Marshal(transaction)
			totalTransactionsSize += len(tt)
		}

		walk, err = s.node.Bc.GetBlockByHash(walk.ParentHash())
		if err != nil {
			break
		}
	}

	// For division by zero error
	if totalTransactions == 0 {
		totalTransactions = -1
	}
	if dealCount == 0 {
		dealCount = -1
	}
	if height == 0 {
		height = -1
	}

	displayObj := struct {
		NodePublicKey         string
		NodeWallet            int64
		TotalBlockChainSize   int
		BlockchainHeight      int
		AvgBlockSize          int
		TotalDealsSize        int
		NumberOfDeals         int
		AvgDealSize           int
		TotalTransactionsSize int
		NumberOfTransactions  int
		TransactionSize       int
	}{ //TODO:ChangeToPK -> review
		NodePublicKey:         s.node.PublicKey().String(),
		NodeWallet:            s.node.Wm.MyWallet.GetBalance(),
		BlockchainHeight:      height,
		TotalBlockChainSize:   totalBlockSize,
		AvgBlockSize:          totalBlockSize / height,
		TotalDealsSize:        totalDealsSize,
		NumberOfDeals:         dealCount,
		AvgDealSize:           totalDealsSize / dealCount,
		TotalTransactionsSize: totalTransactionsSize,
		NumberOfTransactions:  totalTransactions,
		TransactionSize:       totalTransactionsSize / totalTransactions,
	}
	bytes, err := json.Marshal(displayObj)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	io.WriteString(w, string(bytes))
}

// StartExplorer starts blockchain explorer
func (s *Services) StartExplorer(pno string) {
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/threads", s.HandleThreads)
	router.HandleFunc("/", s.handleGetBlockchain)
	router.HandleFunc("/stats", s.handleShowStats)
	log.Fatal(http.ListenAndServe(":"+pno, router))
}
