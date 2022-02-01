package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	myNet "net"
	_ "net/http/pprof"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/guyu96/go-timbre/core"
	"github.com/guyu96/go-timbre/crypto/pos"
	"github.com/guyu96/go-timbre/faucet"
	"github.com/guyu96/go-timbre/log"
	"github.com/guyu96/go-timbre/net"
	pb "github.com/guyu96/go-timbre/protobuf"
	"github.com/guyu96/go-timbre/roles"
	"github.com/guyu96/go-timbre/rpc"
	"github.com/guyu96/go-timbre/services"
	"github.com/mbilal92/noise"
)

const (
	// host = "10.230.9.216" // localhost
	host                 = "127.0.0.1"  // localhost
	bootstrapPort        = uint16(7000) // bootstrap port starts at 10000
	indexerBootstrapPort = uint16(8080)
	numBootstrapPeers    = 1               // make 6 bootstrap peers
	bootstrapFile        = "bootstrap.txt" // bootstrap peers file
	Dir                  = "../"           // database directory
	peersDir             = "../peers/"     // peers file directory
	blocksToAskFor       = 10              // value for z
)

// var cpuprofile = flag.String("cpuprofile", "cpu.prof", "write cpu profile to `file`") //For profiling
// var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

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
	var user *roles.User
	var provider *roles.StorageProvider
	var miner *roles.Miner
	var bandwidthProvider *roles.BandwidthProvider
	var moderator *roles.Moderator
	var indexer *roles.Indexer

	blockchainNameFlag := flag.String("bn", "default", "BlockChain Name")
	portFlag := flag.Uint("p", uint(bootstrapPort), "")
	minerFlag := flag.Bool("m", false, "miner ")
	userFlag := flag.Bool("u", false, "user ")
	moderatorFlag := flag.Bool("mo", false, "moderator ")
	storageProviderFlag := flag.Bool("s", false, "storage provider ")
	faucetFlag := flag.Bool("f", false, "faucet ")
	storageProviderTimeOfferFlag := flag.Uint("spt", 300000000, "storage provider Max time")
	storageProviderSpaceOfferFlag := flag.Uint64("sps", 4294967295, "storage provider Max Space ")
	storageProviderPriceOfferFlag := flag.Uint("spp", 100, "storage provider Price ")

	bandwidthProviderFlag := flag.Bool("b", false, "bandwidth provider ")

	indexerFlag := flag.Bool("i", false, "indexer ")
	indexerPortFlag := flag.Uint("pi", uint(indexerBootstrapPort), "indexer port ")
	loogFileFlag := flag.String("lf", "terminal", "file Name and Path for log file")

	httpPortFlag := flag.String("http", "7200", "port for api http server")
	rpcPortFlag := flag.String("rpc", "7211", "port for api grc server")

	resetDB := flag.Bool("r", false, "Reset DB)")

	flag.Parse()

	//For profiling
	// if *cpuprofile != "" {
	// 	f, err := os.Create(*cpuprofile)
	// 	if err != nil {
	// 		// log.Fatal("could not create CPU profile: ", err)
	// 	}
	// 	defer f.Close() // error handling omitted for example
	// 	if err := pprof.StartCPUProfile(f); err != nil {
	// 		// log.Fatal("could not start CPU profile: ", err)
	// 	}
	// 	defer pprof.StopCPUProfile()
	// }
	// if *memprofile != "" {
	// 	fmt.Println("creating file")
	// 	f, err := os.Create(*memprofile)
	// 	if err != nil {
	// 		// log.Fatal("could not create memory profile: ", err)
	// 	}
	// 	defer f.Close() // error handling omitted for example
	// 	runtime.GC()    // get up-to-date statistics
	// 	if err := pprof.WriteHeapProfile(f); err != nil {
	// 		// log.Fatal("could not write memory profile: ", err)
	// 	}
	// }

	/////////////////////
	myIp := GetLocalIP()
	// myIp := "127.0.0.1"
	//Http servver for profiling the programme

	// go func() {
	// 	profPort := myIp + ":" + fmt.Sprint(*portFlag+100)
	// 	fmt.Println("Profport: ", profPort)
	// 	fmt.Println(http.ListenAndServe(profPort, nil))
	// }()

	/////////////////////f
	*blockchainNameFlag = core.DefaultConfig.ChainID
	fmt.Println("blockchain name: ", *blockchainNameFlag)

	log.SetNewOutput(*loogFileFlag)

	log.Info().Msgf("My Ip Address %v", myIp)
	port := uint16(*portFlag)
	indexerPort := uint16(*indexerPortFlag)
	bootstrapPath := path.Join(peersDir, bootstrapFile)
	// voteReq := !(*voteLess)
	// Two execution modes depending on port.
	if port == bootstrapPort {
		// Create nodes.
		nodes := make([]*net.Node, numBootstrapPeers)
		for i := 0; i < numBootstrapPeers; i++ {
			if *resetDB {
				dbPath := path.Join(Dir+"db/", fmt.Sprintf("%d-%s.db", port+uint16(i), *blockchainNameFlag))
				os.Remove(dbPath)
			}
			node, err := net.NewNode(Dir, myIp, port+uint16(i), 0, *blockchainNameFlag)
			// node.SetBootstrapper()
			if err != nil {
				panic(err)
			}
			node.SetBootstrapper()
			nodes[i] = node
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
		for _, node := range nodes {
			go func(node *net.Node) {
				wg.Add(1)
				node.BootstrapFromFile(bootstrapPath)
				wg.Done()
			}(node)
			time.Sleep(time.Millisecond * 50)
		}
		wg.Wait()
		log.Info().Msg("Bootstrapping Done.")
		// Set up nodes for running.
		for _, node := range nodes {
			bootstrapService := services.NewServices(node) //Starts the service instance of the node
			syncer := roles.NewSyncer(node)
			syncer.Setup()
			node.SetSyncer(syncer)
			node.SetService(bootstrapService)

			go node.Listen()
			go node.Distribute()
			go bootstrapService.Process()
		}
		log.Info().Msg("Nodes are up and running.")
		select {}
	} else {

		// Create, bootstrap, and run node.

		if *resetDB {
			fmt.Println("Reset DB...")
			dbPath := path.Join(Dir+"db/", fmt.Sprintf("%d-%s.db", port, *blockchainNameFlag))
			os.Remove(dbPath)
		}
		comSize := 2
		fmt.Println("Creating new node...")
		node, err := net.NewNode(Dir, myIp, port, comSize, *blockchainNameFlag)
		if err != nil {
			panic(err)
		}

		log.Info().Msg("Node created with initial WALLET balance of 10,000.")
		node.BootstrapFromFile(bootstrapPath)
		log.Info().Msgf("Bootstrap done, connected to %d peers: %v", node.NumPeers(), node.GetPeerAddress())
		nodeService := services.NewServices(node) //Starts the service instance of the node
		node.SetService(nodeService)
		syncer := roles.NewSyncer(node)
		node.SetSyncer(syncer)
		syncer.Setup()

		go node.Listen()
		go node.Distribute()

		go syncer.Process()
		go nodeService.Process()

		syncer.SyncBlockchain()

		if (*moderatorFlag) != false {
			moderator = roles.NewModerator(node)
			moderator.Process()
		}

		// Bandwidth provider initiates individually or when it's a storage provider
		if (*bandwidthProviderFlag) != false || (*storageProviderFlag) != false {
			bandwidthProvider = roles.NewBandwidthProvider(node)
			node.SetBandwidthProvider(bandwidthProvider)
			go node.BandwidthProvider.Setup()
			go node.BandwidthProvider.Process()
			go node.BandwidthProvider.GetThreads()
			log.Info().Msgf("Bandwidth Provider process has started")
		}

		if (*storageProviderFlag) != false {
			provider = roles.NewStorageProvider(node)
			node.SetStorageProvider(provider)
			go provider.Process()
			log.Info().Msgf("Storage provider process has started")
			provider.SendStorageOffer(uint32(*storageProviderPriceOfferFlag), uint32(*storageProviderTimeOfferFlag), uint32(*storageProviderSpaceOfferFlag)) // storage offer values
		}

		if (*minerFlag) != false {
			miner = roles.NewMiner(node)
			node.SetMiner(miner)
			node.Services.AnnounceAndRegister() //Will make an miner announcement and do registration for genesis round

			if (*indexerFlag) != false {
				indexer = roles.NewIndexer(node)
				go indexer.Process()
				log.Info().Msgf("Indexer process has started")
				indexer.StartServer(syncer, indexerPort)
			}

			miner.StartMiner()
		}

		if (*userFlag) != false {
			user = roles.NewUser(node)
			log.Info().Msgf("User process has started")
			// fmt.Println(userf)
			node.SetUser(user)
			go node.User.Setup()
			go node.User.Process()
			rpc := rpc.NewRPCServer(*rpcPortFlag, *httpPortFlag) // These all should go in the indexer including the getThreads
			rpc.Run(node)
		}

		if *faucetFlag {
			go func() {
				fmt.Println("Running the faucet on this node...")
				faucet.RunFaucet(node)
			}()
		}

		fmt.Println("NODE TYPE: ", node.NodeType)
		// log.Info().Err(errors.New("hello world"))
		var lastHash string
		for {

			fmt.Println("ENTER KEY: \n cb -> create empty block \n at -> Create block with transaction \n tc -> start consensus and register candidate(m flag required)  \n vt -> vote tx ")
			fmt.Println(" pb -> Print blockchain \n sso -> send storage offer(requires provider flag) \n sp -> send post request(requires poster flag) \n pw -> print wallet balances \n sy : Sync Blockchain ")
			fmt.Println(" pv -> Print votes \n qd -> Quit delegate \n pc -> Print Candidates \n dr -> delegate registration \n pm -> print miners \n mpk -> print my publicKey \n con : Print the connections")
			fmt.Println(" df -> Double forge block \n cs -> Clear state caches \n ar -> Add revolt settings")
			fmt.Println(" sd -> Start/Stop Dpos(if running) \n pmb -> Print miners by round\n psb -> Print SLots by round\n pmc -> Prints dpos miner cache\n ts -> Cureent TS in Unix\n pst -> prints the programme stats \n ppt -> Print post stat \n exit -> Stops programme and quits")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				panic(err)
			}

			if strings.TrimSpace(input) == "at" {
				log.Info().Msgf("Creating block with transaction!!!")
				log.Info().Msgf("Plz enter the publicKey of the transaction reciever...")
				pk, err := reader.ReadString('\n')
				if err != nil {
					panic(err)
				}

				log.Info().Msgf("Plz enter the Amount...")
				amount, err2 := reader.ReadString('\n')
				if err2 != nil {
					panic(err2)
				}

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				amountInt, err3 := strconv.ParseInt(strings.TrimSpace(amount), 10, 64)
				if err3 != nil {
					panic(err3)
				}

				err = nodeService.DoAmountTrans(ctx, strings.TrimSpace(pk), uint64(amountInt))
				if err != nil {
					log.Error().Err(err)
				}
			} else if strings.TrimSpace(input) == "cb" {
				log.Info().Msgf("Creating empty block!!!!")
				fmt.Println("Peer addresses are: ", node.GetPeerAddress())

				if node.Dpos.HasStarted == true {
					miner.GenerateBlock(nil, nil, nil, nil, nil, node.Dpos.GetRoundNum(), time.Now().UnixNano(), true)
				} else {
					miner.GenerateBlock(nil, nil, nil, nil, nil, 0, 0, true)

				}

			} else if strings.TrimSpace(input) == "df" {
				log.Info().Msgf("Double forging block... ")
				tail := node.Bc.GetMainTail()
				tailParent, _ := node.Bc.GetBlockByHash(tail.ParentHash())
				forgingHeight := 0
				if tail != nil {
					forgingHeight = tail.HeightInt()
				}
				for i := 0; i < 1; i++ {
					if node.Dpos.HasStarted == true {
						miner.GenerateTestBlock(tailParent, nil, nil, nil, nil, nil, node.Dpos.GetRoundNum(), time.Now().UnixNano(), int64(forgingHeight), true)
					} else {
						miner.GenerateTestBlock(nil, nil, nil, nil, nil, nil, 0, 0, int64(forgingHeight), true)
					}
					time.Sleep(1 * time.Second)
				}

			} else if strings.TrimSpace(input) == "gp" && user != nil {
				node.User.GetThreads()

			} else if strings.TrimSpace(input) == "pb" {
				syncer.PrintBlockChain()
			} else if strings.TrimSpace(input) == "pw" {
				node.Wm.PrintWalletBalances()
			} else if strings.TrimSpace(input) == "sb" {
				msg := net.NewMessage(net.MsgCodeBroadcastTest, []byte("BroadCast Msg"))
				node.Broadcast(msg)
			} else if strings.TrimSpace(input) == "smp" {
				log.Info().Msgf("Peers %v", node.GetPeerAddress())
				log.Info().Msgf("Enter Peer Public key Address")
				hexPbKey, err := reader.ReadString('\n')
				if err != nil {
					panic(err)
				}
				log.Info().Msgf("Enter Peer Address")
				txtTosend, err := reader.ReadString('\n')
				if err != nil {
					panic(err)
				}
				decoded, _ := hex.DecodeString(hexPbKey)
				var publicKey noise.PublicKey
				copy(publicKey[:], decoded)

				msg := net.NewMessage(net.MsgCodeRelayTest, []byte(txtTosend))
				node.RelayToPB(publicKey, msg)

			} else if strings.TrimSpace(input) == "sp" && user != nil {
				log.Info().Msgf("Text to send:")
				txtTosend, err := reader.ReadString('\n')
				if err != nil {
					panic(err)
				}

				txtTosend = strings.TrimSpace(txtTosend)
				lastHash = user.SendPostRequest([]byte(txtTosend), nil, nil, 100, 100, nil)
			} else if strings.TrimSpace(input) == "spr" && user != nil {

				parentPostHash, _ := hex.DecodeString(lastHash)
				log.Info().Msgf("Text to send:")
				txtTosend, err := reader.ReadString('\n')
				if err != nil {
					panic(err)
				}
				txtTosend = strings.TrimSpace(txtTosend)
				user.SendPostRequest([]byte(txtTosend), nil, parentPostHash, 100, 300, nil)
			} else if strings.TrimSpace(input) == "sso" {
				log.Info().Msgf("Sending storage offer")

				node.StorageProvider.SendStorageOffer(100, 300000000, 4294967295)
			} else if strings.TrimSpace(input) == "sy" {
				if node.Dpos.HasStarted == true {
					node.Dpos.Stop()
				}

				syncer.SyncBlockchain()
				node.Services.StartDpos()
			} else if strings.TrimSpace(input) == "t" {
				syncer.SendTestMsg()
			} else if strings.TrimSpace(input) == "tc" && (*minerFlag) != false {
				log.Info().Msgf("starting to test the consensus protocol")

				err := nodeService.StartDpos()
				if err != nil {
					log.Error().Err(err)
					log.Info().Err(err)
				}

			} else if strings.TrimSpace(input) == "vt" {
				log.Info().Msgf("Creating vote transaction")
				log.Info().Msgf("Plz enter the publicKey of the vote reciever...")
				pk, err := reader.ReadString('\n')
				if err != nil {
					panic(err)
				}
				log.Info().Msgf("Plz enter the percentage weight to give to reciever...")
				percent, err := reader.ReadString('\n')
				if err != nil {
					panic(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				err = nodeService.DoPercentVote(ctx, strings.TrimSpace(pk), strings.TrimSpace(percent))
				if err != nil {
					log.Error().Err(err)
					log.Info().Err(err)
				}
			} else if strings.TrimSpace(input) == "uvt" {
				log.Info().Msgf("Creating test vote transaction")
				log.Info().Msgf("Plz enter the publicKey of the unvote cand...")
				pk, err := reader.ReadString('\n')
				if err != nil {
					panic(err)
				}
				log.Info().Msgf("Plz enter the percentage weight to take from reciever...")
				percent, err := reader.ReadString('\n')
				if err != nil {
					panic(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				err = nodeService.DoPercentUnVote(ctx, strings.TrimSpace(pk), strings.TrimSpace(percent))
				if err != nil {
					log.Error().Err(err)
					log.Info().Err(err)
				}
			} else if strings.TrimSpace(input) == "ar" {
				log.Info().Msgf("Plz enter the revolt height...")
				h, err := reader.ReadString('\n')
				height, err := strconv.ParseInt(strings.TrimSpace(h), 10, 64)
				if err != nil {
					panic(err)
				}
				core.DefaultConfig.RevoltHeight = height
				log.Info().Msgf("Plz enter the revoltblock hash...")
				hash, err := reader.ReadString('\n')
				if err != nil {
					panic(err.Error())
				}
				blockHash, err := hex.DecodeString(strings.TrimSpace(hash))
				if err != nil {
					panic(err.Error())
				}
				core.DefaultConfig.RevoltBlockHash = blockHash
				core.DefaultConfig.Revolt = true
				fmt.Println("Revolt setting added")
			} else if strings.TrimSpace(input) == "pv" {
				node.Wm.PrintWalletVotes()
			} else if strings.TrimSpace(input) == "qd" {
				err := nodeService.DoDelegateQuit()
				if err != nil {
					log.Info().Msgf(err.Error())
				}

			} else if strings.TrimSpace(input) == "ac" {
				if miner != nil { //Only register without transaction if it hasn't synced up
					node.Services.AnnounceCandidacy()
				}

			} else if strings.TrimSpace(input) == "pc" {
				node.Dpos.State.PrintCandidates()
			} else if strings.TrimSpace(input) == "mpk" {
				log.Info().Msgf("My PK:- %v", node.PublicKey().String())
			} else if strings.TrimSpace(input) == "pm" {
				node.Dpos.PrintMiners()
			} else if strings.TrimSpace(input) == "dr" {
				_, err = node.Services.DoDelegateRegisteration()
				if err != nil {
					log.Info().Msgf(err.Error())
				}
			} else if strings.TrimSpace(input) == "con" {
				fmt.Println("Peer addresses are: ", node.GetPeerAddress())
			} else if strings.TrimSpace(input) == "sd" {
				if node.Dpos.HasStarted == true {
					node.Dpos.Stop()
				} else {
					node.Services.StartDpos()
				}
			} else if strings.TrimSpace(input) == "pac" {
				log.Info().Msgf("Active Deals: %v", node.Activedeals)
			} else if strings.TrimSpace(input) == "pmb" {
				node.Dpos.PrintMinersByRound()
			} else if strings.TrimSpace(input) == "pmc" {
				node.Dpos.PrintMinersCache()
			} else if strings.TrimSpace(input) == "psb" {
				node.Dpos.PrintSlotsPassedByRound()
			} else if strings.TrimSpace(input) == "ts" {
				fmt.Println("Cur TS:- ", time.Now().Unix(), time.Now().UnixNano())
			} else if strings.TrimSpace(input) == "tpos" {

				cfg := pos.MakeTestConfig()

				pairing, pk, sk, _ := pos.KeyGenTest(cfg)
				_, _ = pk, sk

				randomPrf := pos.GetRandProof(pairing)
				randomPrf2 := pos.GetProofwith0(pairing)
				protoPrf := randomPrf2.ToProto()
				log.Info().Msgf("\tSP --- prf %v  prf. PSI: %v, Y:%v TAG %v", randomPrf, protoPrf.Psi, protoPrf.Y, protoPrf.Tag)

			} else if strings.TrimSpace(input) == "pst" {
				go NewMonitor(2) //logs after every 2 second
			} else if strings.TrimSpace(input) == "cntw" {
				node.Close()
			} else if strings.TrimSpace(input) == "nntw" {
				node.NewNetwork(myIp)
			} else if strings.TrimSpace(input) == "btf" {
				node.BootstrapFromFile(bootstrapPath)
			} else if strings.TrimSpace(input) == "sdt" {
				if node.Miner != nil {
					miner.StoreDataforTesting = !miner.StoreDataforTesting
				}
			} else if strings.TrimSpace(input) == "csdt" {
				if node.Miner != nil {
					miner.ReceivedChPrPairForTesting = make(map[string]*pb.ChPrPair)
				}
			} else if strings.TrimSpace(input) == "ppt" {

				if node.Miner != nil {
					node.Miner.PrintPostStats()
				}
			} else if strings.TrimSpace(input) == "tr" {
				peers := node.GetPeerAddress()
				testRelaytoPeer(node, peers)
			} else if strings.TrimSpace(input) == "cs" {
				fmt.Println("Clearing the state cache")
				node.StateStore.ClearCachedState()
			} else if strings.TrimSpace(input) == "trm" {

				if node.Miner != nil {
					node.Miner.TestRelayToSp()
				}
			} else if strings.TrimSpace(input) == "exit" {
				fmt.Println("Stoping node...")
				node.Dpos.Stop()
				//Check for other flags and quit
				break
			} else if strings.TrimSpace(input) == "scm" {
				if node.Miner != nil {
					go func() { //Cleverly producing blocks during miner's own turn

						for { //For now produing one block infinitely
							if node.Syncer.GetIsSyncing() == true || node.Dpos.HasStarted == false {
								continue //Not creating invalid block when nodes are syncing
							}

							if node.Dpos.GetCurrentMinerAdd() == node.GetHexID() {
								if node.Dpos.HasStarted == true {
									log.Warn().Msgf("Creating invalid BLOCK!!!&&&&&&............")
									miner.GenerateBlock(nil, nil, nil, nil, nil, node.Dpos.GetRoundNum(), time.Now().UnixNano(), true)
								}
								time.Sleep(node.Dpos.GetMinerSlotTime())
							}
							time.Sleep(node.Dpos.GetTickInterval())
						}
					}()
				}
			} else {
				log.Info().Msgf("Wrong input please try again")
			}
		}
	}
}

var gPeers []noise.ID

func testRelaytoPeer(node *net.Node, peers []noise.ID) {
	if len(peers) == 0 && len(gPeers) == 0 {
		fmt.Println("No peers to relay to")
		return
	} else if len(peers) < len(gPeers) {
		peers = gPeers
	}

	//Only sending to the first one
	bs := []byte("This is a test msg" + time.Now().String())
	log.Info().Msgf("Relaying test msg %v", peers[0])
	msg := net.NewMessage(net.TestMsg, bs)
	node.Relay(peers[0], msg)
	gPeers = peers
}

//NewMonitor gives the programe realted stats
func NewMonitor(duration int) {
	var m Monitor
	var rtm runtime.MemStats
	var interval = time.Duration(duration) * time.Second

	f, err := os.OpenFile("moniter.log", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println(err)
	}

	for {
		<-time.After(interval)

		// Read full mem stats
		runtime.ReadMemStats(&rtm)

		// Number of goroutines
		m.NumGoroutine = runtime.NumGoroutine()

		// Misc memory stats
		m.Alloc = bytesToMB(rtm.Alloc)
		m.TotalAlloc = bytesToMB(rtm.TotalAlloc)
		m.Sys = bytesToMB(rtm.Sys)
		m.Mallocs = rtm.Mallocs
		m.Frees = rtm.Frees

		// Live objects = Mallocs - Frees
		m.LiveObjects = m.Mallocs - m.Frees

		// GC Stats
		m.PauseTotalNs = rtm.PauseTotalNs
		m.NumGC = rtm.NumGC

		// Just encode to json and print
		b, _ := json.Marshal(m)
		result := string(b)
		fmt.Println(result)

		if _, err := f.WriteString(result); err != nil {
			fmt.Println(err.Error())
		}
	}
}

//Monitor is the struct to get stats related to the programme
type Monitor struct {
	Alloc,
	TotalAlloc,
	Sys,
	Mallocs,
	Frees,
	LiveObjects,
	PauseTotalNs uint64

	NumGC        uint32
	NumGoroutine int
}

//bytesToMB convert byte to the Mbs
func bytesToMB(b uint64) uint64 {
	return b / 1024 / 1024
}
func remove(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}
