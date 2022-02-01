package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path"
	"sync"
	"time"

	"github.com/guyu96/go-timbre/log"
	"github.com/guyu96/go-timbre/net"
	"github.com/guyu96/go-timbre/roles"
	"github.com/guyu96/go-timbre/services"
)

const (
	host              = "127.0.0.1"     // localhost
	bootstrapPort     = uint16(7001)    // bootstrap port starts at 10000
	numBootstrapPeers = 2               // make 6 bootstrap peers
	bootstrapFile     = "bootstrap.txt" // bootstrap peers file
	dbDir             = "../db/"        // database directory
	peersDir          = "../peers/"     // peers file directory
	blocksToAskFor    = 5               // value for z
)

func cleanUp(path string) {
	dir, _ := os.Open(path)
	files, _ := dir.Readdir(0)
	for _, file := range files {
		fpath := path + file.Name()
		os.Remove(fpath)
	}
}

func main() {
	// MinerPort := uint16(7005)
	// PosterPort := uint16(7006)
	// SpPort := uint16(7007)
	// latejoiningNodePort := uint16(7008)
	// bwPort := uint16(7008)
	rand.Seed(5)

	var poster *roles.Poster
	var provider *roles.StorageProvider
	var miner *roles.Miner
	var bandwidthProvider *roles.BandwidthProvider

	minerportFlag := flag.Uint("mp", uint(7005), "port for miner")
	spPortFlag := flag.Uint("sp", uint(7006), "port for sp")
	posterportFlag := flag.Uint("pp", uint(7007), "port for poster")
	latejoiningportFlag := flag.Uint("lp", uint(7008), "port for late joining node")

	flag.Parse()

	MinerPort := uint16(*minerportFlag)
	PosterPort := uint16(*posterportFlag)
	SpPort := uint16(*spPortFlag)
	latejoiningNodePort := uint16(*latejoiningportFlag)

	port := bootstrapPort
	bootstrapPath := path.Join(peersDir, bootstrapFile)
	voteReq := false
	// Two execution modes depending on port.
	bootStrapingNodes := make([]*net.Node, numBootstrapPeers)
	for i := 0; i < numBootstrapPeers; i++ {
		dbPath := path.Join(dbDir, fmt.Sprintf("%d.db", port+uint16(i)))
		os.Remove(dbPath)
		node, err := net.NewNode(dbPath, host, port+uint16(i), 0, voteReq)
		if err != nil {
			panic(err)
		}
		bootStrapingNodes[i] = node
	}
	log.Info().Msgf("Nodes created.")
	// Create bootstrap peers file.
	os.Remove(bootstrapPath)
	f, err := os.Create(bootstrapPath)
	if err != nil {
		panic(err)
	}
	for i := 0; i < numBootstrapPeers; i++ {
		_, err := f.WriteString(fmt.Sprintf("%s:%d\n", host, bootstrapPort+uint16(i)))
		if err != nil {
			panic(err)
		}
	}
	f.Close()
	log.Info().Msg("Bootstrap peers file created.")
	// Bootstrap nodes with each other
	var wg sync.WaitGroup
	for _, node := range bootStrapingNodes {
		go func(node *net.Node) {
			wg.Add(1)
			node.BootstrapFromFile(bootstrapPath)
			wg.Done()
		}(node)
		time.Sleep(time.Millisecond * 50)
	}
	wg.Wait()
	// Set up nodes for running.
	for _, node := range bootStrapingNodes {
		bootstrapService := services.NewServices(node) //Starts the service instance of the node
		syncer := roles.NewSyncer(node, bootstrapService)

		go node.Listen()
		go node.Distribute()
		go syncer.Process()
		go bootstrapService.Process()
		go syncer.CheckBpForPending()

	}

	minerSyncer, minerNode, minerNodeService := TestNewNode(MinerPort, bootstrapPath)
	spSyncer, spNode, _ := TestNewNode(SpPort, bootstrapPath)
	posterSyncer, posterNode, posterNodeService := TestNewNode(PosterPort, bootstrapPath)

	poster = roles.NewPoster(posterNode)
	log.Info().Msgf("Poster process has started")

	posterNode.StartPoster(poster)
	go posterNode.Poster.Setup()
	go posterNode.Poster.Process()
	go posterNodeService.StartExplorer("22100")

	provider = roles.NewStorageProvider(spNode)
	go provider.Process()

	bandwidthProvider = roles.NewBandwidthProvider(spNode)
	spNode.StartBandwidthProvider(bandwidthProvider)
	go spNode.BandwidthProvider.Setup()
	go spNode.BandwidthProvider.Process()
	go spNode.BandwidthProvider.GetThreads()
	go spNode.BandwidthProvider.Process()

	miner = roles.NewMiner(minerNode)
	time.Sleep(time.Second)
	go miner.CreateBlockLoop()
	// comSize := 2
	err = minerNodeService.StartDpos()
	if err != nil {
		log.Error().Err(err)
	}
	go func() { //Make candidate announcement so that nodes joining in a moment may know other candidates
		for x := 0; x < 6; x++ {
			minerNodeService.AnnounceCandidacy()
			time.Sleep(time.Second)
		}
	}()

	go func() {
		for {
			select {
			case <-minerNode.Dpos.ListeningSignal:
				ctx, _ := context.WithTimeout(context.Background(), (minerNode.Dpos.GetCommitteeTime() - 2*1e9)) //The time should be equivalent to the miner timeslot
				miner.StartListening(ctx)
			case <-minerNode.Dpos.MiningSignal:
				// ctx, _ := context.WithTimeout(context.Background(), (node.Dpos.GetMinerSlotTime() - node.Dpos.GetTickInterval())) //The time should be equivalent to the miner timeslot
				// defer cancel()
				miner.StartMining()
			}
		}
	}()

	log.Info().Msgf("waiting for DPOS to start")
	time.Sleep(time.Second * 25)

	var lastHash string
	provider.SendStorageOffer(1, 300, 500) // storage offer values
	log.Info().Msgf("Storage offer sent")

	time.Sleep(time.Second * 3)

	log.Info().Msgf("Sending Post")
	lastHash = poster.SendPostRequest([]byte("TIMBRE IS GREAT YAHOOO!!!!"), nil, nil, 1, 30)
	time.Sleep(time.Second * 3)

	log.Info().Msgf("Sending Post reply")
	ThreadPostHash, _ := hex.DecodeString(lastHash)
	lastHash = poster.SendPostRequest([]byte("HERE AGAIN, TIMBRE IS GREAT, I am TELLING YOU!!! MAN"), ThreadPostHash, ThreadPostHash, 1, 30)
	log.Info().Msgf("Sending Post reply")
	parentPostHash, _ := hex.DecodeString(lastHash)
	poster.SendPostRequest([]byte("3rd Try Man"), parentPostHash, ThreadPostHash, 1, 30)
	time.Sleep(time.Second * 3)

	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// defer cancel()
	// posterNodeService.DoVoteTrans(ctx, string(MinerPort), miner)

	time.Sleep(time.Second * 7)
	log.Info().Msgf("User Requesting posts content")
	posterNode.Poster.GetThreads()
	time.Sleep(time.Second * 7)

	postFound := 0
	posts := posterNode.Poster.GetTs()
	if len(posts) != 2 {
		log.Error().Msgf("Number of posts is %v, should be %v ", len(posts), 2)
	}

	for post := range posts {
		if string(post) == "TIMBRE IS GREAT YAHOOO!!!!" {
			postFound += 1
		}
		if string(post) == "HERE AGAIN, TIMBRE IS GREAT, I am TELLING YOU!!! MAN" {
			postFound += 1
		}
	}

	if postFound != 2 {
		log.Error().Msgf("posts not found in blockchain")
	}

	posterSyncer.PrintBlockChain()
	minerSyncer.PrintBlockChain()
	spSyncer.PrintBlockChain()

	time.Sleep(time.Second * 3)
	minerNode.Wm.PrintWalletBalances()
	posterNode.Wm.PrintWalletBalances()
	spNode.Wm.PrintWalletBalances()

	if minerNode.Wm.MyWallet.GetBalance() != 10010 {
		log.Error().Msgf("minerNode wallet balance is %v it should be %v", minerNode.Wm.MyWallet.GetBalance(), 10010)
	}

	if posterNode.Wm.MyWallet.GetBalance() != 9999 {
		log.Error().Msgf("posterNode wallet balance is %v it should be %v", minerNode.Wm.MyWallet.GetBalance(), 9999)
	}

	if spNode.Wm.MyWallet.GetBalance() != 1001 {
		log.Error().Msgf("spNode wallet balance is %v it should be %v", minerNode.Wm.MyWallet.GetBalance(), 1001)
	}

	_, latejoiningNode, _ := TestNewNode(latejoiningNodePort, bootstrapPath)
	time.Sleep(time.Second * 5)
	log.Info().Msgf("LatejoiningNode started, waiting for syn")

	if latejoiningNode.Bc.GetMainTail() == minerNode.Bc.GetMainTail() {
		log.Error().Msgf("latejoiningNode not synced up properly ")
	}

	select {}

}

func TestNewNode(port uint16, bootstrapPath string) (*roles.Syncer, *net.Node, *services.Services) {
	dbPath := path.Join(dbDir, fmt.Sprintf("%d.db", port))
	os.Remove(dbPath)
	node, err := net.NewNode(dbPath, host, port, 2, false)
	if err != nil {
		panic(err)
	}

	node.BootstrapFromFile(bootstrapPath)
	nodeService := services.NewServices(node) //Starts the service instance of the node
	syncer := roles.NewSyncer(node, nodeService)
	go node.Listen()
	go node.Distribute()
	go syncer.Process()
	go nodeService.Process()
	syncer.SyncBlockchain()
	return syncer, node, nodeService
}
