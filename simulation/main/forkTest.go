package main

import (
	"fmt"
	"math"
	"os"
	"path"
	"sync"
	"time"

	"github.com/guyu96/go-timbre/log"
	"github.com/guyu96/go-timbre/net"
	"github.com/guyu96/go-timbre/roles"
	"github.com/guyu96/go-timbre/rpc"
	"github.com/guyu96/go-timbre/services"
)

const (
	host           = "127.0.0.1"     // localhost
	bootstrapPort  = uint16(7000)    // bootstrap port starts at 10000
	bootstrapFile  = "bootstrap.txt" // bootstrap peers file
	dbDir          = "../db/"        // database directory
	peersDir       = "../peers/"     // peers file directory
	blocksToAskFor = 5               // value for z
	bootstrapPath  = peersDir + bootstrapFile
	rpcPortFlag    = "7211"
	httpPortFlag   = "7200"
)

func main() {
	// The file will basically create a network to test forks
	// 1:- By periodically creating invalid blocks
	// 2:- It will test  network partition
	//

	// reader := bufio.NewReader(os.Stdin)
	// fmt.Println("Plz enter the bootstrap starting port: ")
	// input, err := reader.ReadString('\n')
	// if err != nil {
	// 	panic(err)
	// }
	// bootPort, err := strconv.ParseInt(strings.TrimSpace(input), 10, 64)
	// if err != nil {
	// 	panic(err)
	// }

	//Creating bootstrap nodes
	// CreateNetBootstrappers(num, port )
	log.SetNewOutput("terminal")
	bootstrappers := createNetBootstrappers(1, 7000)
	RunNetBootstrappers(bootstrappers)

	RunForkSwitchTest(bootstrappers)

	// RunMiners(miners)

	// RunSPs(sps)
	// RunPosters(posters)

	// SendStorageOffers(sps, 5)         //Sending storage offers
	// CreateRandPosts(0.5, posters, 10) //Num of posts per sec
	// time.Sleep(30 * time.Second)

	//Designating last miner as the corrupt miner
	// CreateCorruptBlocksOnTurn(miners[2:3])
	// SendStorageOffers(sps, 5) //Sending storage offers

	// go ValidateDposRoundStartTime(miners)
	// go ValidateDposRoundNum(miners)
	// go ValidateDposMinersATM(miners)
	// select {}
}

//RunForkSwitchTest runs the forkswitch test. One node produces the invalid block -> creates fork
//-> fork eventually gets resolved by looking at the longer chain becase invalid one is rejected by the others
func RunForkSwitchTest(bootstrappers []*net.Node) {
	//Creating the miners
	miners := CreateMinersNet(3, 3000)
	RunMiners(miners)
	time.Sleep(30 * time.Second)

	CreateCorruptBlocksOnTurn(miners[2:3]) //Will create corrupt blocks on it turn

	go ValidateDposRoundStartTime(miners)
	go ValidateDposRoundNum(miners)
	go ValidateDposMinersATM(miners)

	select {}

}

//RunNetworkpartitionTest runs a network partition test
func RunNetworkpartitionTest(bootstrappers []*net.Node) {

	//Creating the miners
	miners := CreateMinersNet(3, 3000)
	// sps := CreateSPNet(2, 4000)
	// posters := CreatePosterNet(3, 5000)
	fmt.Println("Partitioning the network...")

	CreateNetworkPartition(miners, bootstrappers)

	time.Sleep(2 * time.Second)

	checkBroadcastToPeers(miners)

	RunMiners(miners)

	time.Sleep(40 * time.Second)
	fmt.Println("Printing miner of: ", 0)
	miners[0].Node().Dpos.PrintMiners()

	fmt.Println("Printing miner of: ", 3)
	miners[2].Node().Dpos.PrintMiners()

	time.Sleep(100 * time.Second)
	reconnectDividedNet(miners)

	time.Sleep(1 * time.Second)

	checkBroadcastToPeers(miners)

	select {}
}

//CreateNetworkPartition creates the network partition- default set to 2
func CreateNetworkPartition(miners []*roles.Miner, bootstrappers []*net.Node) {
	//Dividing into 2 by default
	// cutInd := (len(miners) / 2)
	// for i := 0; i < cutInd; i++ { //Disconnecting the first half with the later half

	// 	for k := 0; k < len(bootstrappers); k++ { //Disconnecting with the bootstrappers
	// 		miners[i].Node().DisconnectWithPeers([]noise.ID{noise.ID{ID: bootstrappers[k].GetNodeID().ID, Address: bootstrappers[k].GetNodeID().Address}})
	// 	}

	// 	for j := cutInd; j < len(miners); j++ { //Disconnecting with the other nodes
	// 		miners[i].Node().DisconnectWithPeers([]noise.ID{noise.ID{ID: miners[j].Node().GetNodeID().ID, Address: miners[j].Node().GetNodeID().Address}})
	// 	}
	// }

	// for i := cutInd; i < len(miners); i++ { //Disconnecting the first half with the later half

	// 	for k := 0; k < len(bootstrappers); k++ { //Disconnecting with the bootstrappers
	// 		miners[i].Node().DisconnectWithPeers([]noise.ID{noise.ID{ID: bootstrappers[k].GetNodeID().ID, Address: bootstrappers[k].GetNodeID().Address}})
	// 	}
	// 	for j := 0; j < cutInd; j++ { //Disconnecting with the other nodes
	// 		miners[i].Node().DisconnectWithPeers([]noise.ID{noise.ID{ID: miners[j].Node().GetNodeID().ID, Address: miners[j].Node().GetNodeID().Address}})
	// 	}
	// }

	// // for j := cutInd + 1; j < len(miners); j++ {
	// // 	miners[j].Node().DisconnectWithPeers([]noise.ID{noise.ID{ID: miners[cutInd].Node().GetNodeID().ID, Address: miners[cutInd].Node().GetNodeID().Address}, noise.ID{ID: miners[cutInd-1].Node().GetNodeID().ID, Address: miners[cutInd-1].Node().GetNodeID().Address}, noise.ID{ID: bootstrappers[0].GetNodeID().ID, Address: bootstrappers[0].GetNodeID().Address}})
	// // }

	fmt.Println("Network has been divided into 2!!!")
}

func tRelaytoPeer(node *net.Node) {
	peers := node.GetPeerAddress()
	if len(peers) == 0 {
		fmt.Println("No peers to relay to")
		return
	}
	for _, peer := range peers {
		bs := []byte("This is a test msg" + time.Now().String())
		log.Info().Msgf("Relaying test msg total:%v , %v", len(peers), peer)
		msg := net.NewMessage(net.TestMsg, bs)
		node.Relay(peer, msg)
	}
}

func reconnectDividedNet(miners []*roles.Miner) {
	for _, miner := range miners {
		miner.Node().BootstrapFromFile(bootstrapPath)
	}

	for i := 0; i < len(miners); i++ {
		for j := 0; j < len(miners); j++ {
			if i == j {
				continue
			}
			fmt.Println(miners[i].Node().GetNodeID().Address, " trying ", miners[j].Node().GetNodeID().Address)
			miners[i].Node().GetNet().BootstrapDefault([]string{miners[j].Node().GetNodeID().Address})
			time.Sleep(100 * time.Millisecond)
		}
	}

}

//Checks the broadcast testmsg against every node
func checkBroadcastToPeers(miners []*roles.Miner) {
	fmt.Println("checking the broadcast now")

	for _, miner := range miners {
		// tRelaytoPeer(miner.Node())
		tBroadcasttoPeer(miner.Node())
		// time.Sleep(1 * time.Second)
	}
}

func tBroadcasttoPeer(node *net.Node) {
	// peers := node.GetPeerAddress()
	// if len(peers) == 0 {
	// 	fmt.Println("No peers to relay to")
	// 	return
	// }
	// for _, peer := range peers {
	bs := []byte("This is a test msg" + time.Now().String())
	// log.Info().Msgf("Relaying test msg total:%v , %v", len(peers), peer)
	msg := net.NewMessage(net.TestMsg, bs)
	node.Broadcast(msg)
	// }
}

//CreateCorruptBlocksOnTurn creates the corrupt blocks periodically
func CreateCorruptBlocksOnTurn(corruptMiners []*roles.Miner) {
	for _, miner := range corruptMiners {
		log.Warn().Msgf("Designating a corrupt Miner: %v", miner.Node().GetHexID())
		go func() { //Cleverly producing blocks during miner's own turn

			for { //For now produing one block infinitely
				if miner.Node().Syncer.GetIsSyncing() == true || miner.Node().Dpos.HasStarted == false {
					continue //Not creating invalid block when nodes are syncing
				}

				if miner.Node().Dpos.GetCurrentMinerAdd() == miner.Node().GetHexID() {
					if miner.Node().Dpos.HasStarted == true {
						log.Warn().Msgf("Creating invalid BLOCK!!!&&&&&&............")
						miner.GenerateBlock(nil, nil, nil, nil, nil, miner.Node().Dpos.GetRoundNum(), time.Now().UnixNano(), true)
					}
					time.Sleep(miner.Node().Dpos.GetMinerSlotTime())
				}
				time.Sleep(miner.Node().Dpos.GetTickInterval())
			}
		}()

	}
}

//CreateCorruptBlockRandmly creates the invalid block irrespective of the turn
func CreateCorruptBlockRandmly(corruptMiners []*roles.Miner) {
	for _, miner := range corruptMiners {
		log.Warn().Msgf("Designating a corrupt Miner: %v", miner.Node().GetHexID())
		go func() { //Cleverly producing blocks during miner's own turn

			for { //For now produing one block infinitely
				if miner.Node().Syncer.GetIsSyncing() == true || miner.Node().Dpos.HasStarted == false {
					continue //Not creating invalid block when nodes are syncing
				}

				// if miner.Node().Dpos.GetCurrentMinerAdd() == miner.Node().GetHexID() {
				if miner.Node().Dpos.HasStarted == true {
					log.Warn().Msgf("Creating invalid BLOCK!!!&&&&&&............")
					miner.GenerateBlock(nil, nil, nil, nil, nil, miner.Node().Dpos.GetRoundNum(), time.Now().UnixNano(), true)
				}
				time.Sleep(3 * miner.Node().Dpos.GetMinerSlotTime()) //sleeps for 3 slots
				// }
				// time.Sleep(miner.Node().Dpos.GetTickInterval())
			}
		}()

	}
}

//ValidateDposRoundStartTime validates that every miners is giving mining signal correctly
func ValidateDposRoundStartTime(miners []*roles.Miner) {
	time.Sleep(20 * time.Second)

	time.Sleep(miners[0].Node().Dpos.TimeLeftToMine(time.Now().Unix()) + 2*time.Second)
	log.Info().Msgf("Starting to validate Dpos timing...")
	for { //Forever validate the DPOS round start time
		var roundTime int64 = 0
		for i, miner := range miners {
			if miner.Node().Syncer.GetIsSyncing() == true || miner.Node().Dpos.HasStarted == false {
				continue //Round start time will be different when switching between forks so don't check
			}
			fmt.Println("start time left: ", miners[0].Node().Dpos.TimeLeftToMine(time.Now().Unix()))
			mRT := miner.Node().Dpos.GetRoundStartTime()
			log.Debug().Msgf("%v MRT: %v", i, mRT)
			if roundTime == 0 { //setting the round start time of the miner
				roundTime = mRT
			} else {
				if roundTime > mRT+5 || roundTime < mRT-5 { //testing on
					fmt.Println("roundTIme: ", roundTime, "MinerTime: ", mRT, "curTime:", time.Now().Unix())
					panic("Diff round start time")
				}
			}
		}

		timeToSleep := miners[0].Node().Dpos.TimeLeftToMine(time.Now().Unix()) + 5*time.Second
		// for timeToSleep <= 0 {
		// 	time.Sleep(2 * time.Second)
		// 	timeToSleep = miners[0].Node().Dpos.TimeLeftToMine(time.Now().Unix()) + 1*time.Second
		// }
		fmt.Println("time to sleep: ", timeToSleep)
		time.Sleep(timeToSleep)
	}

}

//ValidateDposMinersATM validates that every node running the dpos should have same miner mining
func ValidateDposMinersATM(miners []*roles.Miner) {
	time.Sleep(20 * time.Second)

	time.Sleep(miners[0].Node().Dpos.TimeLeftToMine(time.Now().Unix()) + 2*time.Second)
	log.Info().Msgf("Starting to validate Miners ATM...")
	for { //Forever validate the miners
		var curMiner string = ""
		for i, miner := range miners {
			if miner.Node().Syncer.GetIsSyncing() == true || miner.Node().Dpos.HasStarted == false {
				continue //Round start time will be different when switching between forks so don't check
			}
			m := miner.Node().Dpos.GetCurrentMinerAdd()
			log.Debug().Msgf("%v m: %v", i, m)
			if curMiner == "" { //setting the round start time of the miner
				curMiner = m
			} else {
				if curMiner != m { //testing on
					fmt.Println("curMinerGlobal: ", curMiner, "curMiner: ", m, "-> ", miner.Node().GetHexID(), " CurTime", time.Now().Unix())

					panic("Diff miners ATM")
				}
			}
		}
		timeToSleep := miners[0].Node().Dpos.TimeLeftToMine(time.Now().Unix()) + 5*time.Second
		// for timeToSleep <= 0 {
		// 	time.Sleep(2 * time.Second)
		// 	timeToSleep = miners[0].Node().Dpos.TimeLeftToMine(time.Now().Unix()) + 1*time.Second
		// }
		fmt.Println("time to sleep: ", timeToSleep)
		time.Sleep(timeToSleep)
		// time.Sleep(miners[0].Node().Dpos.GetMinerSlotTime())
	}

}

//ValidateDposRoundNum validates the DPOS round number with a sleep of one miner slot time
func ValidateDposRoundNum(miners []*roles.Miner) {
	time.Sleep(20 * time.Second)

	time.Sleep(miners[0].Node().Dpos.TimeLeftToMine(time.Now().Unix()) + 2*time.Second)
	log.Info().Msgf("Starting to validate Dpos round number...")
	for { //Forever validate the DPOS round start time
		var roundNum int64 = 0
		for i, miner := range miners {
			// syncer.isSyncing == true && syncer.node.Dpos.HasStarted == false
			if miner.Node().Syncer.GetIsSyncing() == true || miner.Node().Dpos.HasStarted == false {
				continue //Round number will be different when switching between forks so don't check
			}

			mRN := miner.Node().Dpos.GetRoundNum()
			log.Debug().Msgf("%v MRN: %v", i, mRN)
			if roundNum == 0 { //setting the round start time of the miner
				roundNum = mRN
			} else {
				if roundNum != mRN {
					fmt.Println("roundNum: ", roundNum, "MinerRoundNum: ", mRN, "curTime: ", time.Now().Unix())
					panic("Diff round numbers")
				}
			}
		}

		timeToSleep := miners[0].Node().Dpos.TimeLeftToMine(time.Now().Unix()) + 5*time.Second
		// for timeToSleep <= 0 {
		// 	time.Sleep(2 * time.Second)
		// 	timeToSleep = miners[0].Node().Dpos.TimeLeftToMine(time.Now().Unix()) + 1*time.Second
		// }
		fmt.Println("time to sleep: ", timeToSleep)
		time.Sleep(timeToSleep)
	}
}

//CreateMinersNet creates the network of the miners
func CreateMinersNet(num int, port uint16) []*roles.Miner {
	fmt.Println("Creating miners node")
	miners := make([]*roles.Miner, num)

	for i := 0; i < num; i++ {
		n := CreateNode(port+uint16(i), true)
		m := roles.NewMiner(n)
		n.SetAndStartMiner(m)
		miners[i] = m
	}

	return miners
}

//CreateSPNet create storage provider network
func CreateSPNet(num int, port uint16) []*roles.StorageProvider {
	fmt.Println("Creating Storage provider Network...")
	sps := make([]*roles.StorageProvider, num)

	for i := 0; i < num; i++ {
		n := CreateNode(port+uint16(i), false)
		sp := roles.NewStorageProvider(n)
		n.SetStorageProvider(sp)
		go sp.Process()
		sps[i] = sp
	}
	return sps
}

//SendStorageOffers sends the storage offer
func SendStorageOffers(sps []*roles.StorageProvider, startDelay int) {
	time.Sleep(time.Duration(startDelay) * time.Second)

	for _, sp := range sps {
		sp.SendStorageOffer(100, 300000000, 4294967295) // storage offer values
	}
}

//RunSPs runs the SPS
func RunSPs(sps []*roles.StorageProvider) {
	for _, sp := range sps {
		sp.Node().Services.StartDpos()
	}
}

//CreatePosterNet creates the network of the users
func CreatePosterNet(num int, port uint16) []*roles.User {
	fmt.Println("Creating users network...")
	users := make([]*roles.User, num)

	for i := 0; i < num; i++ {
		n := CreateNode(port+uint16(i), false)
		u := roles.NewUser(n)
		n.SetUser(u)
		go n.User.Setup()
		rpc := rpc.NewRPCServer(rpcPortFlag, httpPortFlag) // These all should go in the indexer including the getThreads
		rpc.Run(n)
		users[i] = u
	}
	return users
}

//RunPosters runs the posters
func RunPosters(users []*roles.User) {
	for _, u := range users {
		u.Node().Services.StartDpos()
	}
}

//CreateRandPosts creates the random post per second
func CreateRandPosts(rate float64, users []*roles.User, startDelay int) {
	time.Sleep(time.Duration(startDelay) * time.Second)
	randPost := "This is a rand TestPOST by a programmer who is himself stressed out and wants to stress the network as well. Lets all enjoy a stressful life. Wish u best of luck :-)>>> "
	for _, u := range users {
		go func(u *roles.User, post string) {
			for {
				var sleepTime time.Duration = 2 * time.Second
				if rate < 1 {
					sec := int(float64(1) / rate)
					sleepTime = time.Duration(sec) * time.Second
					// fmt.Println("Sleep sec: ", sleepTime)
				}
				time.Sleep(sleepTime)

				for i := 0; i < int(math.Ceil(rate)); i++ {
					fmt.Println("Sending Post...")
					u.SendPostRequest([]byte(post+time.Now().String()+" >>>>>"), nil, nil, 100, 300, nil)
				}
			}

		}(u, randPost)
	}
}

//RunMiners starts the miners mining process. Basically starts the DPOS
func RunMiners(miners []*roles.Miner) {
	for _, miner := range miners {
		go miner.Node().Services.StartDpos()
		go miner.Node().Services.AnnounceAndRegister() //Will make an miner announcement and do registration for genesis round

	}
}

//CreateNode creates a general node.
func CreateNode(port uint16, isMiner bool) *net.Node {
	blockchainName := "default"
	dbPath := path.Join(dbDir, fmt.Sprintf("%d-%s.db", port, blockchainName))
	os.Remove(dbPath)
	node, err := net.NewNode("../", host, port, 2, false, blockchainName)
	if err != nil {
		panic(err)
	}

	nodeService := services.NewServices(node) //Starts the service instance of the node
	syncer := roles.NewSyncer(node)
	node.SetSyncer(syncer)
	node.SetupWithoutSync(nodeService, syncer, bootstrapPath, isMiner)
	return node
}

//createNetBootstrappers creates the instance of the bootstrappers
func createNetBootstrappers(num int, port uint16) []*net.Node {
	log.Info().Msgf("Creating bootstrapping nodes...")
	blockchainName := "default"
	bootStrapingNodes := make([]*net.Node, num)
	bootstrapPath := path.Join(peersDir, bootstrapFile)
	for i := 0; i < num; i++ {
		dbPath := path.Join(dbDir, fmt.Sprintf("%d-%s.db", port, blockchainName))
		os.Remove(dbPath)
		node, err := net.NewNode("../", host, port+uint16(i), 0, false, blockchainName)
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
	for i := 0; i < num; i++ {
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
	return bootStrapingNodes
}

//RunNetBootstrappers runs the network bootstrapper nodes
func RunNetBootstrappers(bootstrappers []*net.Node) {
	log.Info().Msgf("Running bootstrapping node")
	for _, node := range bootstrappers {
		bootstrapService := services.NewServices(node) //Starts the service instance of the node
		syncer := roles.NewSyncer(node)
		node.SetSyncer(syncer)
		node.SetService(bootstrapService)

		go node.Listen()
		go node.Distribute()
		// go syncer.Process()
		go bootstrapService.Process()
	}
}
