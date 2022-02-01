package noderunner

import (
	"fmt"
	myNet "net"
	"os"
	"os/signal"
	"path"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/guyu96/go-timbre/core"
	"github.com/guyu96/go-timbre/log"
	"github.com/guyu96/go-timbre/net"
	"github.com/guyu96/go-timbre/roles"
	"github.com/guyu96/go-timbre/rpc"
	"github.com/guyu96/go-timbre/services"
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

//DaemonNode is the daemonNode node instance run by the programme
var daemonNode *net.Node

//GetLocalIP gets the local ip-address
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

//RunAsBootstrapper run node as a bootstrapper
func RunAsBootstrapper(portNumber string, resetDB bool) error {

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	bootstrapPath := path.Join(peersDir, bootstrapFile)
	myIp := GetLocalIP()
	// myIp := "127.0.0.1"

	fmt.Println("myIp: ", myIp)
	port64, err := strconv.ParseUint(portNumber, 10, 64)
	if err != nil {
		return err
	}
	port := uint16(port64)
	// Create nodes.
	nodes := make([]*net.Node, numBootstrapPeers)
	blockchainName := core.DefaultConfig.ChainID
	for i := 0; i < numBootstrapPeers; i++ {
		if resetDB {

			dbPath := path.Join(Dir+"db/", fmt.Sprintf("%d-%s.db", port, blockchainName))
			os.Remove(dbPath)
		}
		fmt.Println("Creating new node")
		node, err := net.NewNode(Dir, myIp, port+uint16(i), 0, blockchainName)
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
	log.Info().Msgf("Bootstrap peers file created.")
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
	// Setting up and running nodes
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
	fmt.Println("Waiting for the closing signal")

	<-sigCh
	log.Info().Msgf("Elegantly shutting down timbre node")

	return nil
}

//RunAndSetNewNode runs new node and set it as globally accessible daemon node// Is miner flag is required in case u are the first one to run the chain
func RunAndSetNewNode(nodePort, rpcPort, httpPort string, resetDB bool, isMiner bool) error {

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	comSize := 2
	fmt.Println("Creating new node...")
	port64, err := strconv.ParseUint(nodePort, 10, 64)
	if err != nil {
		return err
	}
	port := uint16(port64)

	bootstrapPath := path.Join(peersDir, bootstrapFile)
	myIp := GetLocalIP()
	blockchainName := core.DefaultConfig.ChainID
	//resets the db
	if resetDB {
		// dbPath := path.Join(Dir+"db/", fmt.Sprintf("%d.db", port))
		dbPath := path.Join(Dir+"db/", fmt.Sprintf("%d-%s.db", port, blockchainName))
		os.Remove(dbPath)
	}

	node, err := net.NewNode(Dir, myIp, port, comSize, blockchainName)
	if err != nil {
		panic(err)
	}

	daemonNode = node
	fmt.Println("Daemon node is set")
	log.Info().Msg("Node created with initial WALLET balance of 10,000.")
	node.BootstrapFromFile(bootstrapPath)
	log.Info().Msgf("Bootstrap done, connected to %d peers: %v", node.NumPeers(), node.GetPeerAddress())
	nodeService := services.NewServices(node) //Starts the service instance of the node
	node.SetService(nodeService)
	syncer := roles.NewSyncer(node)
	node.SetSyncer(syncer)
	syncer.Setup()

	log.Info().Msgf("Running rpc server...")
	rpc := rpc.NewRPCServer(rpcPort, httpPort) //Running rpc server
	rpc.Run(node)

	go node.Listen()
	go node.Distribute()

	go syncer.Process()
	go nodeService.Process()
	syncer.SyncBlockchain()

	//running miner node
	err = CreateAndStartMiner()
	if err != nil {
		log.Error().Msgf(err.Error())
	}

	<-sigCh
	log.Info().Msgf("Elegantly shutting down timbre node")

	return nil
}
