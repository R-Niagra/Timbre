package main

import (
	"fmt"
	"math"
	myNet "net"
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
	//You can choose to create as many sps and posters as you want
	log.SetNewOutput("terminal")

	var num int
	fmt.Println("Enter the num of storage Providers to start: ")
	_, err := fmt.Scanf("%d", &num)
	if err != nil {
		return
	}
	if num > 0 {
		var sPort int
		fmt.Println("Enter the starting port: ")
		_, err := fmt.Scanf("%d", &sPort)
		if err != nil {
			return
		}
		sps := createSPNet(num, uint16(sPort))
		// runSPs(sps)
		sendStorageOffers(sps, 5)
	}

	num = 0

	fmt.Println("Enter the nums of poster to start: ")
	_, err = fmt.Scanf("%d", &num)
	if err != nil {
		return
	}
	if num > 0 {
		var sPort int
		fmt.Println("Enter the starting port: ")
		_, err := fmt.Scanf("%d", &sPort)
		if err != nil {
			return
		}
		var postRate float64
		fmt.Println("Enter the posts per sec against posters: ")
		_, err = fmt.Scanf("%f", &postRate)
		if err != nil {
			return
		}

		posters := createAndRunPosterNet(num, uint16(sPort))
		// runPosters(posters)
		// var postRate float64
		// fmt.Println("Enter the posts per sec against posters: ")
		// _, err = fmt.Scanf("%f", &postRate)
		// if err != nil {
		// 	return
		// }
		// fmt.Println("f is; ", postRate)
		createRandPosts(postRate, posters, 10) //Num of posts per sec
	}

	select {}
}

//SendStorageOffers sends the storage offer
func sendStorageOffers(sps []*roles.StorageProvider, startDelay int) {
	time.Sleep(time.Duration(startDelay) * time.Second)

	for _, sp := range sps {
		sp.SendStorageOffer(100, 300000000, 4294967295) // storage offer values
	}
}

//RunSPs runs the SPS
func runSPs(sps []*roles.StorageProvider) {
	for _, sp := range sps {
		sp.Node().Services.StartDpos()
	}
}

//createAndRunPosterNet creates the network of the users
func createAndRunPosterNet(num int, port uint16) []*roles.User {
	fmt.Println("Creating users network...")
	users := make([]*roles.User, num)
	var wg sync.WaitGroup

	for i := 0; i < num; i++ {
		wg.Add(1)
		go func(index int) {
			fmt.Println("Wg added")
			n := createNode(port+uint16(index), false)
			fmt.Println("Node created")
			u := roles.NewUser(n)
			n.SetUser(u)
			go n.User.Setup()
			// rpc := rpc.NewRPCServer(rpcPortFlag, httpPortFlag) // These all should go in the indexer including the getThreads
			// rpc.Run(n)
			users[index] = u
			runPosters([]*roles.User{u})
			wg.Done()
			fmt.Println("Wg done")
		}(i)

	}
	fmt.Println("Waiting on users")
	wg.Wait()
	fmt.Println("Done creation")
	return users
}

//RunPosters runs the posters
func runPosters(users []*roles.User) {
	for _, u := range users {
		u.Node().Services.StartDpos()
	}
}

//createRandPosts creates the random post per second
func createRandPosts(rate float64, users []*roles.User, startDelay int) {
	time.Sleep(time.Duration(startDelay) * time.Second)
	count := 0
	randPost := "This is a rand TestPOST by a programmer who is himself stressed out and wants to stress the network as well. Lets all enjoy a stressful life. Wish u best of luck :-)>>> "
	for _, u := range users {
		go func(u *roles.User, post string) {
			for {
				var sleepTime time.Duration = 1 * time.Second
				if rate < 1 {
					sec := int(float64(1) / rate)
					sleepTime = time.Duration(sec) * time.Second
					// fmt.Println("Sleep sec: ", sleepTime)
				}
				time.Sleep(sleepTime)

				for i := 0; i < int(math.Ceil(rate)); i++ {
					fmt.Println("Sending Post...")
					u.SendPostRequest([]byte(post+fmt.Sprint(count)+" >>>>>"), nil, nil, 100, 300, nil)
					count++
				}
			}

		}(u, randPost)
	}
}

//createSPNet create storage provider network
func createSPNet(num int, port uint16) []*roles.StorageProvider {
	fmt.Println("Creating Storage provider Network...")
	sps := make([]*roles.StorageProvider, num)
	var wg sync.WaitGroup
	for i := 0; i < num; i++ {
		wg.Add(1)
		go func(index int) {
			n := createNode(port+uint16(index), false)
			sp := roles.NewStorageProvider(n)
			n.SetStorageProvider(sp)
			go sp.Process()
			sps[index] = sp
			runSPs([]*roles.StorageProvider{sp})
			wg.Done()
		}(i)
	}
	wg.Wait()
	return sps
}

//CreateNode creates a general node.
func createNode(port uint16, isMiner bool) *net.Node {
	dbPath := path.Join(dbDir, fmt.Sprintf("%d-default.db", port))
	os.Remove(dbPath)
	node, err := net.NewNode("../", getLocalIP(), port, 2, false, "default")
	if err != nil {
		panic(err)
	}

	nodeService := services.NewServices(node) //Starts the service instance of the node
	syncer := roles.NewSyncer(node)
	node.SetSyncer(syncer)
	node.Setup(nodeService, syncer, bootstrapPath, isMiner)
	return node
}

//Fetches the local address of the user
func getLocalIP() string {
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
