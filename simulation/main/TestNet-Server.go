package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"math/rand"
	myNet "net"
	"os"
	"path"
	"sync"
	"time"

	"github.com/guyu96/go-timbre/log"
	"github.com/guyu96/go-timbre/net"
	pb "github.com/guyu96/go-timbre/protobuf"
	"github.com/guyu96/go-timbre/roles"
	"github.com/guyu96/go-timbre/rpc"
	"github.com/guyu96/go-timbre/services"
)

const (
	host              = "127.0.0.1"     // localhost
	bootstrapPort     = uint16(7001)    // bootstrap port starts at 10000
	numBootstrapPeers = 1               // make 6 bootstrap peers
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

func GetLocalIP() string {
	addrs, err := myNet.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*myNet.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

func main() {

	rand.Seed(5)
	myIp := GetLocalIP()

	blockchainNameFlag := flag.String("bn", "default", "BlockChain Name")
	bootstrapFlag := flag.Bool("b", false, "bootstraping")
	minerportFlag := flag.Uint("mp", uint(7005), "port for miner")
	minerportFlag2 := flag.Uint("mp2", uint(7006), "port for miner")
	minerportFlag3 := flag.Uint("mp3", uint(7007), "port for miner")
	spPortFlag := flag.Uint("sp", uint(7008), "port for sp")
	spPortFlag2 := flag.Uint("sp2", uint(7009), "port for sp")
	posterportFlag := flag.Uint("pp", uint(7010), "port for poster")
	// explorerPortFlag := flag.String("ep", "22100", "port for Explorer")
	// explorerPortFlag2 := flag.String("ep2", "22200", "port for Explorer of miner 2")
	loogFileFlag := flag.String("lf", "terminal", "file Name and Path for log file")
	httpPortFlag := flag.String("http", "7200", "port for api http server")
	rpcPortFlag := flag.String("rpc", "7211", "port for api grc server")
	// indexerPortNodeFlag := flag.Uint("ip", uint(7008), "port for indexer Node")
	// indexerPortFlag := flag.Uint("ipp", uint(7009), "port for indexer listner")

	// latejoiningportFlag := flag.Uint("lp", uint(7008), "port for late joining node")

	flag.Parse()

	MinerPort := uint16(*minerportFlag)
	MinerPort2 := uint16(*minerportFlag2)
	MinerPort3 := uint16(*minerportFlag3)
	PosterPort := uint16(*posterportFlag)
	SpPort := uint16(*spPortFlag)
	SpPort2 := uint16(*spPortFlag2)
	// IndexerNodePort := uint16(*indexerPortNodeFlag)
	// IndexerPort := uint16(*indexerPortFlag)

	fmt.Println("Miner Port", MinerPort)
	// latejoiningNodePort := uint16(*latejoiningportFlag)

	log.SetNewOutput(*loogFileFlag)

	port := bootstrapPort
	bootstrapPath := path.Join(peersDir, bootstrapFile)
	if *bootstrapFlag {
		voteReq := false
		// Two execution modes depending on port.
		bootStrapingNodes := make([]*net.Node, numBootstrapPeers)
		for i := 0; i < numBootstrapPeers; i++ {
			dbPath := path.Join(dbDir, fmt.Sprintf("%d-%s.db", port+uint16(i), *blockchainNameFlag))
			os.Remove(dbPath)
			node, err := net.NewNode("../", myIp, port+uint16(i), 0, voteReq, *blockchainNameFlag)
			if err != nil {
				panic(err)
			}
			node.SetBootstrapper()
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
			_, err := f.WriteString(fmt.Sprintf("%s:%d\n", myIp, bootstrapPort+uint16(i)))
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
			syncer := roles.NewSyncer(node)
			node.SetService(bootstrapService)
			node.SetSyncer(syncer)
			go node.Listen()
			go node.Distribute()
			// go syncer.Process()
			go bootstrapService.Process()
			// go syncer.CheckBpForPending()
			// node.Services.StartDpos(2)
		}
	}

	fmt.Println("Starting other roles")
	minerNode := TestNewNode(MinerPort, bootstrapPath, true, *blockchainNameFlag)
	miner := roles.NewMiner(minerNode)
	minerNode.SetAndStartMiner(miner)
	time.Sleep(60 * time.Second) // let 1st miner start so it does not face the initial Dpos error
	var minerNode2, minerNode3, spNode, spNode2, posterNode *net.Node
	go func(node *net.Node) {
		node = TestNewNode(MinerPort2, bootstrapPath, true, *blockchainNameFlag)
		node.SetAndStartMiner(roles.NewMiner(node))
	}(minerNode2)

	go func(node *net.Node) {
		node = TestNewNode(MinerPort3, bootstrapPath, true, *blockchainNameFlag)
		node.SetAndStartMiner(roles.NewMiner(node))
	}(minerNode3)
	time.Sleep(60 * time.Second) // let 1st miner start so it does not face the initial Dpos error
	go func(node *net.Node) {
		node = TestNewNode(SpPort, bootstrapPath, false, *blockchainNameFlag)
		node.SetAndStartBandwidthProvider(roles.NewBandwidthProvider(node))
		node.SetAndStartStorageProvider(roles.NewStorageProvider(node))
	}(spNode)
	go func(node *net.Node) {
		node = TestNewNode(SpPort2, bootstrapPath, false, *blockchainNameFlag)
		node.SetAndStartBandwidthProvider(roles.NewBandwidthProvider(node))
		node.SetAndStartStorageProvider(roles.NewStorageProvider(node))
	}(spNode2)
	time.Sleep(60 * time.Second) // let 1st miner start so it does not face the initial Dpos error

	posterNode = TestNewNode(PosterPort, bootstrapPath, false, *blockchainNameFlag)
	posterNode.SetAndStartPoster(roles.NewUser(posterNode))

	time.Sleep(60 * time.Second) // let 1st miner start so it does not face the initial Dpos error
	rpc := rpc.NewRPCServer(*rpcPortFlag, *httpPortFlag)
	rpc.Run(posterNode)

	spNode.StorageProvider.SendStorageOffer(100, 3000, 100000) // storage offer values
	log.Info().Msgf("Storage offer sent")
	spNode2.StorageProvider.SendStorageOffer(100, 3000, 4294967295) // storage offer values
	log.Info().Msgf("Storage offer sent")

	time.Sleep(time.Second * 3)

	// indexer := roles.NewIndexer(IndexerNode)
	// go indexer.Process()
	// log.Info().Msgf("Indexer process has started on port %v", IndexerPort)
	// go indexer.StartServer(IndexerNodeSyncer, IndexerPort)

	var ThreadHashes []string
	replyThreadHashes := make(map[string]string)
	mainThreadExpiryTime := make(map[string]time.Time)

	log.Info().Msgf("Sending Post")
	lastThreadHash := posterNode.User.SendPostRequest([]byte("Timbre is running in the bg"), nil, nil, 100, 300, nil)
	ThreadHashes = append(ThreadHashes, lastThreadHash)
	replyThreadHashes[lastThreadHash] = lastThreadHash
	time.Sleep(time.Second * 3)
	mainThreadExpiryTime[lastThreadHash] = time.Now().Add(time.Duration(300) * time.Second)

	log.Info().Msgf("Sending Post reply")
	parentPostHash, _ := hex.DecodeString(lastThreadHash)
	lastReplyHash := posterNode.User.SendPostRequest([]byte("This is a another post"), parentPostHash, parentPostHash, 100, 300, nil)
	time.Sleep(time.Second * 3)
	replyThreadHashes[lastThreadHash] = lastReplyHash

	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// defer cancel()
	// posterNodeService.DoVoteTrans(ctx, string(MinerPort), miner)

	time.Sleep(posterNode.Dpos.TimeLeftToMine(time.Now().Unix()) + 2*time.Second)
	log.Info().Msgf("User Requesting posts content")
	// posterNode.User.GetThreads()
	// time.Sleep(time.Second * 15)

	postFound := 0
	posts := posterNode.User.GetTs()

	if len(posts) != 2 {
		log.Error().Msgf("Number of posts is %v, should be %v ", len(posts), 2)
	}

	for _, post := range posts {
		if string(post) == "Timbre is running in the bg" {
			postFound += 1
		}

		if string(post) == "This is a another post" {
			postFound += 1
		}
		log.Info().Msgf("Post %v", string(post))
	}

	if postFound != 2 {
		log.Error().Msgf("posts not found in blockchain")
	}

	time.Sleep(posterNode.Dpos.TimeLeftToMine(time.Now().Unix()) + 2*time.Second)

	if posterNode.Wm.MyWallet.GetBalance() != 9999 {
		log.Error().Msgf("posterNode wallet balance is %v it should be %v", minerNode.Wm.MyWallet.GetBalance(), 9999)
	}

	if spNode.Wm.MyWallet.GetBalance() != 1001 {
		log.Error().Msgf("spNode wallet balance is %v it should be %v", minerNode.Wm.MyWallet.GetBalance(), 1001)
	}

	randoms := []string{"Lorem ipsum dolor sit amet, consectetur adipiscing elit.",
		"Vivamus et sem ut mi gravida porta ut sit amet nisl.",
		"Aliquam convallis augue et augue tincidunt mollis.",
		"Sed placerat nisl a neque fermentum finibus.",
		"In sodales quam placerat metus viverra ultricies.",
		"Morbi eu mauris faucibus, dignissim metus at, feugiat enim.",
		"Maecenas non quam a justo vestibulum tempor at porttitor eros.",
		"Nulla ac ligula commodo libero feugiat ultricies vitae sit amet quam.",
		"Duis et risus id quam lacinia luctus id quis est.",
		"Integer in nibh at dui ultricies tempor.",
		"Nam sed justo ac tortor mattis blandit non id quam.",
		"Sed et libero ut metus tristique ultrices vitae imperdiet orci.",
		"Aenean sodales libero et bibendum imperdiet."}

	rand.Seed(123456789)
	for i := 0; i < 10; i++ {
		text_to_send := randoms[rand.Intn(len(randoms))]
		text_to_send = text_to_send + " " + time.Now().String() //" " + string(letterRunes[rand.Intn(len(letterRunes))]) + " " + string(letterRunes[rand.Intn(len(letterRunes))]) + " " + string(letterRunes[rand.Intn(len(letterRunes))]) + " " + string(letterRunes[rand.Intn(len(letterRunes))])
		// text_to_send := RandStringRunes(rand.Intn(250))
		// log.Info().Msgf("Main_Thread_text_to_send %v", text_to_send)
		// time_to_expire := uint32(rand.Intn(40)) + 50
		posterNode.User.SendPostRequest([]byte(text_to_send), nil, nil, 100, 500, nil)
		log.Info().Msgf("text_to_send: %v", text_to_send)
		time.Sleep(2 * time.Second)
	}

	var done chan struct{}
	go CreateCorruptBlockRandmly(miner, done)
	go func() {
		time.Sleep(time.Second * 60)
		done <- struct{}{}
	}()

	select {}

}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandStringRunes(n int) string {
	// rand.Seed(time.Now().UnixNano())
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func TestNewNode(port uint16, bootstrapPath string, isMiner bool, blockchainName string) *net.Node {
	dbPath := path.Join(dbDir, fmt.Sprintf("%d-%s.db", port, blockchainName))
	os.Remove(dbPath)
	node, err := net.NewNode("../", GetLocalIP(), port, 2, false, blockchainName)
	if err != nil {
		panic(err)
	}

	nodeService := services.NewServices(node) //Starts the service instance of the node
	syncer := roles.NewSyncer(node)
	node.Setup(nodeService, syncer, bootstrapPath, isMiner)
	// node.BootstrapFromFile(bootstrapPath)
	// node.SetService(nodeService)
	// doneSync := make(chan struct{})

	// go node.Listen()
	// go node.Distribute()
	// go syncer.Process()
	// go nodeService.Process()
	// go syncer.SyncBlockchain(blocksToAskFor, doneSync)
	// go syncer.CheckBpForPending()
	// <-doneSync
	return node
}

func remove(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

//SignedPostInfoTemplate is the template for the post of the poster
func SignedPostInfoTemplate(content, parentPostHash, threadheadPostHash []byte, maxCost, minDuration uint32) (*pb.SignedPostInfo, error) {
	metadata := &pb.PostMetadata{
		Pid: []byte("Test"),
		// KadID:              []byte("Test"),
		ContentHash: []byte("Test"),
		ContentSize: uint32(len(content)),
		// Timestamp:          time.Now().UnixNano(),
		ParentPostHash:     parentPostHash,
		ThreadHeadPostHash: threadheadPostHash,
	}

	param := &pb.PostParameter{
		MaxCost: maxCost,
		// MinDuration: minDuration,
	}
	info := &pb.PostInfo{
		Metadata: metadata,
		Param:    param,
	}

	testPostInfo := &pb.SignedPostInfo{
		Info: info,
		Sig:  []byte("Add my valid sign here. Otherwise block validation might fail"),
	}
	return testPostInfo, nil
}

//GetPostFee returns the deal fee
// func GetPostFee(post *pb.SignedPostInfo) uint64 {
// 	return core.GetTxFee(post)
// }

func CreateCorruptBlockRandmly(miner *roles.Miner, done chan struct{}) {
	for {
		select {
		case <-done:
			break
		default:
			if miner.Node().Syncer.GetIsSyncing() == true || miner.Node().Dpos.HasStarted == false {
				continue //Not creating invalid block when nodes are syncing
			}

			if miner.Node().Dpos.HasStarted == true {
				log.Warn().Msgf("Creating invalid BLOCK!!!&&&&&&............")
				miner.GenerateBlock(nil, nil, nil, nil, nil, miner.Node().Dpos.GetRoundNum(), time.Now().UnixNano(), true)
			}
			time.Sleep(3 * miner.Node().Dpos.GetMinerSlotTime()) //sleeps for 3 slots
		}
	}
}
