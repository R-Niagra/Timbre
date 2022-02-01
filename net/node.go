package net

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Nik-U/pbc"
	gproto "github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/proto"
	"github.com/guyu96/go-timbre/core"
	"github.com/guyu96/go-timbre/core/blockstate"
	"github.com/guyu96/go-timbre/crypto"
	"github.com/guyu96/go-timbre/crypto/pos"
	"github.com/guyu96/go-timbre/dpos"
	"github.com/guyu96/go-timbre/log"
	"github.com/guyu96/go-timbre/protobuf"
	"github.com/guyu96/go-timbre/storage"
	"github.com/guyu96/go-timbre/wallet"
	"github.com/mbilal92/noise"
	"github.com/mbilal92/noise/network"
	// "github.com/mbilal92/pbc"
)

const (
	incomingChanSize = 128 // Default incoming channel buffer size
	blocksToAskFor   = 10  // value for z
	comSize          = 5
	keysPath         = "../keys/"
	// DealExpirtTime number of days -> sec
	DealExpirtTime = 300 // 14 * 24 * 60 * 60
)

type Keypair struct {
	PrivateKey noise.PrivateKey
	PublicKey  noise.PublicKey
}

// Node encapsulates a runnable node in the Timbre p2p network.
type Node struct {
	Db    *storage.Database // database connection
	Bc    *core.Blockchain  // blockchain
	Bp    *core.Blockpool   // blockpool
	BcMtx sync.Mutex        // blockchain mutex

	TimeOffset time.Duration
	Dpos       *dpos.Dpos
	Wm         *wallet.WalletManager //Wallet manager which will also contain my wallet
	StateStore *blockstate.StateStore
	keys       Keypair          // public and private keys
	ntw        *network.Network // network
	incoming   chan Message     // incoming messages from network are redirected here
	outgoing   *sync.Map
	LocalSync  bool
	Port       uint16
	//TODO: make struct for POS
	PosPairing *pbc.Pairing
	PosConfig  *pos.Config
	PosPk      *pos.PublicKey
	PosSk      *pos.SecretKey
	PosParam   *pbc.Params

	NodeType          string
	User              User // Need to discuss
	StorageProvider   StorageProvider
	Miner             Miner
	BandwidthProvider BandwidthProvider
	Syncer            Syncer
	Services          Services

	DealMtx     sync.RWMutex    // blockchain mutex
	Activedeals map[string]bool // Active Deals from blockchain

	TransactionNonce uint64
	QuitNonceUpChan  chan bool
	PendingTrans     int
	ThreadBase       map[string]([]string) // Maps threadRootHash -> all posts in the thread
	ThreadBaseMap    sync.Map
}

// HandlerFunc Function to be passed to requestAndresponse
type HandlerFunc func(msg Message) error

// NewNode creates a new node.
func NewNode(Path string, host string, port uint16, comSize int, blockchainName string) (*Node, error) {
	localSync := false
	storage.InitVariables(blockchainName)
	keyFile := strconv.Itoa(int(port)) + "_key.txt"      //stores the private key here
	if _, err := os.Stat(keysPath); os.IsNotExist(err) { //Create directory to the for the keys path
		fmt.Println("creating directory")
		os.Mkdir(keysPath, os.ModePerm)
	}
	dbPath := path.Join(Path+"db/", fmt.Sprintf("%d-%s.db", port, blockchainName))
	// os.Remove(dbPath)
	db, err := storage.OpenDatabase(dbPath)
	if err != nil {
		log.Info().Err(err)
		return nil, err
	}

	// Retrieve blockchain if it exists in database.
	var bc *core.Blockchain
	fmt.Println("Started loading bc ")
	if db.HasBucket(storage.BlockBucket) {
		//Loading local blocks
		bc, err = core.LoadBlockchain(db)

		if bc != nil {
			localSync = true
		}
		if err != nil {
			return nil, err
		}
		fmt.Println("Blockchain loaded from db.: ", localSync)
	} else {
		if err := db.NewBucket(storage.BlockBucket); err != nil {
			return nil, err
		}

		bc, err = core.InitBlockchain(db)
		if err != nil {
			return nil, err
		}
	}

	// Create blockpool.
	bp, err := core.NewBlockpool(core.DefaultBlockpoolCapacity)
	if err != nil {
		return nil, err
	}
	// Retrieve or generate keypair.
	var keys Keypair
	if _, err2 := os.Stat(keysPath + keyFile); err2 == nil {
		//key file exists
		log.Info().Msgf("keysPath + keyFile: %v", keysPath+keyFile)
		keys.PrivateKey = noise.LoadKey(keysPath + keyFile)
		keys.PublicKey = keys.PrivateKey.Public()
	} else if os.IsNotExist(err2) {
		var err error
		keys.PublicKey, keys.PrivateKey, err = noise.GenerateKeys(nil)

		db.DeleteBucket(storage.NodeBucket) //Deleting bucket to make node every time:- TODO:Change it to restore state
		err = db.NewBucket(storage.NodeBucket)
		if err != nil {
			log.Info().Err(err)
			return nil, err
		}

		noise.PersistKey(keysPath+keyFile, keys.PrivateKey)
	}

	if !db.HasBucket(storage.DealBucket) {
		if err := db.NewBucket(storage.DealBucket); err != nil {
			return nil, err
		}
	}

	ntw, err := network.New(host, port, keys.PrivateKey, core.DefaultConfig.ChainID, nil, false)
	if err != nil {
		return nil, err
	}

	stateStore, err := blockstate.NewStateStore(db)
	if err != nil {
		return nil, err
	}

	walletMan := wallet.NewWalletManager(keys.PublicKey.String())
	if db.HasBucket(storage.LastChVerificationTimeBucket) == false {
		log.Info().Msgf("Initializing LastChVerificationTime Bucket")
		db.NewBucket(storage.LastChVerificationTimeBucket)
	}
	if db.HasBucket(storage.PostStoredBucket) == false {
		log.Info().Msgf("Initializing PostStored Bucket")
		db.NewBucket(storage.PostStoredBucket)
	}

	// Initializing the dpos consensus protocol
	committeeSize := uint32(comSize)
	// newDpos := dpos.New(committeeSize, bc, wallet.GetAddress(), response.ClockOffset)
	newDpos := dpos.New(committeeSize, walletMan.MyWallet.GetAddress(), 0, walletMan, bc)

	posConfig := pos.MakeTestConfig()
	posPairing, posPk, posSk, param := pos.KeyGen(8, 16, posConfig) // TODO: make these values variable

	newNode := &Node{
		Db:               db,
		Bc:               bc,
		Bp:               bp,
		Wm:               walletMan,
		keys:             keys,
		ntw:              ntw,
		Dpos:             newDpos,
		PosPairing:       posPairing,
		PosConfig:        posConfig,
		PosPk:            posPk,
		PosSk:            posSk,
		PosParam:         param,
		TimeOffset:       0,
		incoming:         make(chan Message, incomingChanSize),
		outgoing:         new(sync.Map),
		LocalSync:        localSync,
		Activedeals:      make(map[string]bool),
		StateStore:       stateStore,
		NodeType:         "",
		TransactionNonce: 1,
		QuitNonceUpChan:  make(chan bool, 128),
		PendingTrans:     0,
		Port:             port,
	}

	walletMan.SetTxExecutor(newNode)
	walletMan.SetNode(newNode)

	return newNode, nil
}

// PublicKey returns the node's public key.
func (node *Node) PublicKey() noise.PublicKey {
	return node.keys.PublicKey
}

// PrivateKey returns the node's private key.
func (node *Node) PrivateKey() noise.PrivateKey {
	return node.keys.PrivateKey
}

// NumPeers returns the number of peers the node has.
func (node *Node) NumPeers() int {
	return node.ntw.GetNumPeers()
}

// GetPeerAddress returns the address of the peers
func (node *Node) GetPeerAddress() []noise.ID {
	return node.ntw.GetPeerAddrs()
}

//GetNet returns the node network type
func (node *Node) GetNet() *network.Network {
	return node.ntw
}

// GetNodeID returns the ID of the node
func (node *Node) GetNodeID() noise.ID {
	return node.ntw.GetNodeID()
}

func (node *Node) GetNodeIDBytes() []byte {
	return node.ntw.GetNodeID().Marshal()
}

// AddOutgoingChan adds an outgoing channel to node. All incoming messages will be redirected to all outoging channels.
func (node *Node) AddOutgoingChan(out chan Message) {
	node.outgoing.Store(out, out)
}

// RemoveOutgoingChan deletes an outgoing chan
func (node *Node) RemoveOutgoingChan(out chan Message) {

	node.outgoing.Delete(out)
}

// Broadcast broadcasts message to the entire network.
func (node *Node) Broadcast(msg Message) {
	// log.Warn().Msgf("Broadcast msg code-length:- %v %v", msg.Code, len(msg.Data))
	node.ntw.Broadcast(msg.Code, msg.Data)
}

// Relay relays message to peer with given ID.
// func (node *Node) Relay(peerID noise.PublicKey, msg Message) { //TODO:ChangeToPK
func (node *Node) Relay(peerID noise.ID, msg Message) { //TODO:ChangeToPK
	// log.Info().Msgf("From %v, TO %v, DATA %v", node.GetNodeID().Address(), peerID.Address(), len(msg.Data))
	// log.Warn().Msgf("Relay msg code-length:- %v %v", msg.Code, len(msg.Data))

	node.ntw.Relay(peerID, msg.Code, msg.Data)
}

func (node *Node) RelayToPB(peerID noise.PublicKey, msg Message) { //TODO:ChangeToPK
	node.ntw.RelayToPB(peerID, msg.Code, msg.Data)
}

// RequestAndResponse broadcast message and exit if receive response from one
func (node *Node) RequestAndResponse(msg Message, f HandlerFunc, filterMsgCode byte) {
	// Temporary response channel
	respChan := make(chan Message, 128)
	node.AddOutgoingChan(respChan)
	defer node.RemoveOutgoingChan(respChan)

	node.Broadcast(msg)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second) // 3 second timeout
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-respChan:
			if msg.Code == filterMsgCode {
				f(msg) // Add error handling
				return
			}
		}
	}
}

//GetResponse listens on the channel for the respose on the provided msg code
func (node *Node) GetResponse(replyMsgCode byte) (Message, error) {
	// Creating temporary response channel
	respChan := make(chan Message, 128)
	node.AddOutgoingChan(respChan)
	defer node.RemoveOutgoingChan(respChan)
	ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Second) // 3 second timeout
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return Message{}, errors.New("No response in 3sec")
		case msg := <-respChan:
			if msg.Code == replyMsgCode {
				return msg, nil
			}
		}
	}

}

// BootstrapFromFile bootstraps the network from peers stored in the peers file.
func (node *Node) BootstrapFromFile(filepath string) error {
	f, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer f.Close()

	peerAddrs := []string{}
	r := bufio.NewReader(f)
	for {
		l, err := r.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		peerAddrs = append(peerAddrs, strings.TrimSpace(l))
	}

	log.Info().Msgf("Bootstrap File Peers %v", peerAddrs)
	node.ntw.BootstrapDefault(peerAddrs)
	return nil
}

// PersistPeerAddrs persists the node's peers to file.
func (node *Node) PersistPeerAddrs(filepath string) error {
	f, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, addr := range node.ntw.GetPeerAddrs() {
		_, err := f.WriteString(addr.Address + "\n")
		if err != nil {
			return err
		}
	}
	return nil
}

// Listen directs incoming relay and broadcast messages to incoming chan.
func (node *Node) Listen() {
	relayChan := node.ntw.GetRelayChan()
	broadcastChan := node.ntw.GetBroadcastChan()
	for {
		select {
		case msg := <-relayChan:
			node.incoming <- NewMessageFromRelay(msg)
		case msg := <-broadcastChan:
			node.incoming <- NewMessageFromBroadcast(msg)
		}
	}
}

//CloseInc closes the incoming channel
func (node *Node) CloseInc() {
	close(node.incoming)
}

// Distribute distributes messages from the incomming channel to all outgoing channels.
func (node *Node) Distribute() {
	for msg := range node.incoming {
		node.outgoing.Range(func(key, value interface{}) bool {
			c, _ := node.outgoing.Load(key)
			c.(chan Message) <- msg
			return true
		})
	}
}

//BlockChainString converts it into string
func (node *Node) BlockChainString() string {
	return node.Bc.String()
}

//OnDisconnectCleanup cleans up on peer disconnection
func (node *Node) OnDisconnectCleanup(id noise.ID) {
	//Donot remove the commeted code!
	// pAdd := id.Address()
	// pPk := id.PublicKey()

	// node.Dpos.State.DelCandidate(pAdd)
	// node.Dpos.State.DelCandidate(string(pPk))
	// fmt.Println("DPOS cleaned up after disconnection")
}

// SetUser sets poster for node
func (node *Node) SetUser(u User) {
	node.User = u
	node.NodeType += "| User |"
}

//SetBootstrapper sets the nodetype tag to bootstrapper
func (node *Node) SetBootstrapper() {
	node.NodeType = "Bootstrapper"
}

//GetHexID return the HexId of the node
func (node *Node) GetHexID() string {
	return node.PublicKey().String()
}

// SetStorageProvider sets storageProvider for node
func (node *Node) SetStorageProvider(sp StorageProvider) {
	node.StorageProvider = sp
	node.NodeType += "| Storage Provider |"
}

// SetMiner sets miner for node
func (node *Node) SetMiner(m Miner) {
	node.Miner = m
	node.NodeType += "| Miner |"
}

func (node *Node) SetAndStartMiner(m Miner) {
	node.Miner = m
	node.NodeType += "| Miner |"

	if node.Bc.IsEmpty() != true && node.Dpos.State.CheckCandByAdd(node.keys.PublicKey.String()) == false { //If first block has been created than only register with the explicit delegateRegister transaction
		_, err := node.Services.DoDelegateRegisteration() //Does the automatic delegate registration if the falg is provided
		if err != nil {
			log.Info().Msgf(err.Error())
		}
	}
	m.StartMiner()
	log.Info().Msgf("Miner started")
}

func (node *Node) SetAndStartBandwidthProvider(bp BandwidthProvider) {
	node.BandwidthProvider = bp
	node.NodeType += "| Bandwidth Provider |"
	go node.BandwidthProvider.Setup()
	go node.BandwidthProvider.Process()
	// go node.BandwidthProvider.GetThreads()
	log.Info().Msgf("Bandwidth started")
}

func (node *Node) SetAndStartStorageProvider(sp StorageProvider) {
	node.StorageProvider = sp
	go node.StorageProvider.Process()
	node.NodeType += "| Storage Provider |"
	log.Info().Msgf("Storage Provider started")
}

func (node *Node) SetAndStartPoster(u User) {
	node.User = u
	node.NodeType += "| User |"
	go node.User.Setup()
	go node.User.Process()
	log.Info().Msgf("Poster started")
}

// SettBandwidthProvider sets bandwidth Provider for node
func (node *Node) SetBandwidthProvider(bp BandwidthProvider) {
	node.BandwidthProvider = bp
	node.NodeType += "| Bandwidth Provider |"
}

func (node *Node) SetService(s Services) {
	node.Services = s
}

//SetSyncer sets the syncer interface in the node
func (node *Node) SetSyncer(syncer Syncer) {
	node.Syncer = syncer
}

func (node *Node) GetNodeType() string {
	return node.NodeType
}

//FindCandByAdd checks if the candidate is present in the state or not
func (node *Node) FindCandByAdd(add string) bool {
	return node.Dpos.State.CheckCandByAdd(add)
}

//EmptyRoleCaches empties the caches of the sp and miner
func (node *Node) EmptyRoleCaches() {
	//EMpting the caches of the miner
	if node.Miner != nil {
		log.Info().Msgf("Emptying the caches of the miner")
		node.Miner.EmptyCaches()
	}
	if node.StorageProvider != nil {
		log.Info().Msgf("Emptying the caches of the SP")
		node.StorageProvider.EmptyCaches()
	}
}

func (node *Node) Setup(services Services, syncer Syncer, bootstrapPath string, isMiner bool) {
	node.BootstrapFromFile(bootstrapPath)
	node.Services = services
	node.Syncer = syncer
	// doneSync := make(chan struct{})

	go node.Listen()
	go node.Distribute()
	go node.Syncer.Process()
	go node.Services.Process()
	// go node.Syncer.SyncBlockchainAtStart(blocksToAskFor, doneSync)
	node.Syncer.SyncBlockchain()
	if isMiner {
		go node.Services.AnnounceAndRegister()
	}

}

//SetupWithoutSync does the setup with out the sync
func (node *Node) SetupWithoutSync(services Services, syncer Syncer, bootstrapPath string, isMiner bool) {
	node.BootstrapFromFile(bootstrapPath)
	time.Sleep(1 * time.Second) // wait for boostrap
	node.Services = services
	node.Syncer = syncer

	go node.Listen()
	go node.Distribute()
	go node.Syncer.Process()
	go node.Services.Process()

}

func (node *Node) SyncTransActionNonceAfterTenSec() {
	log.Info().Msgf("Sleeping for 10 sec to update nonce later..")
	time.Sleep(22 * time.Second)
	confirmedNonce := node.Wm.MyWallet.GetNonce()
	log.Info().Msgf("Temp Nonce: %v, Confirmed Nonce: %v ", node.TransactionNonce, confirmedNonce)

	select {
	case <-node.QuitNonceUpChan:
		log.Info().Msgf("Not updating NONCE")
		return
	default:
		log.Info().Msgf("Updating NONCE")
		node.TransactionNonce = confirmedNonce + 1
		node.PendingTrans = 0
	}

}

func (node *Node) AddDeals(deals []*protobuf.Deal, addToCahce bool) {
	node.DealMtx.Lock()

	for _, deal := range deals {
		dealBytes, _ := proto.Marshal(deal)
		dealhashBytes := crypto.Sha256(dealBytes)
		node.Db.Put(storage.DealBucket, dealhashBytes, dealBytes)
		if addToCahce {
			node.Activedeals[string(dealhashBytes)] = true
		}
	}
	node.DealMtx.Unlock()
}

func (node *Node) RemoveDeals(deals []*protobuf.Deal) {
	node.DealMtx.Lock()

	for _, deal := range deals {
		dealBytes, _ := proto.Marshal(deal)
		dealhashBytes := crypto.Sha256(dealBytes)
		delete(node.Activedeals, string(dealhashBytes))
	}
	node.DealMtx.Unlock()
}

func (node *Node) GetDealbyHash(dealHash string) *protobuf.Deal {
	node.DealMtx.RLock()
	defer node.DealMtx.RUnlock()

	dealBytes, err := node.Db.Get(storage.DealBucket, []byte(dealHash))
	if err != nil {
		log.Error().Msgf("GetDealbyHash - Unable to find Deal in db %v", hex.EncodeToString([]byte(dealHash)))
		return nil
	}

	deal := new(protobuf.Deal)
	if err := proto.Unmarshal(dealBytes, deal); err != nil {
		log.Error().Msgf("GetDealbyHash- dealBytes Unmarshal Error:%v", err)
		return nil
	}

	return deal
}

func (node *Node) GetActiveAndExpiredDealsHash() ([]string, []string) {
	i := 0
	numofdeal := len(node.Activedeals) // - len(node.LastRoundExpiredDeals)
	if numofdeal <= 0 {
		return nil, nil
	}

	dealhashKeys := make([]string, numofdeal)
	var expiredDeal []string
	node.DealMtx.RLock()

	currentTime := time.Unix(node.Dpos.NextTickTime(time.Now().Unix()), 0).Truncate(time.Second)
	for dealHash := range node.Activedeals {
		if dealHash != "" {

			dealhashKeys[i] = dealHash
			i++
			deal := node.GetDealbyHash(dealHash)
			if deal == nil {
				log.Error().Msgf("GetActiveAndExpiredDealsHash Reference deal cannot be found %v", hex.EncodeToString([]byte(dealHash)))
				continue
			}
			ExpiryTime := time.Unix(int64(deal.ExpiryTime), 0) //.Add(time.Second * 15) // wait extra for 1 minning round
			if currentTime.After(ExpiryTime) {                 // TODO: DPOS time for block window, change it to block window time
				expiredDeal = append(expiredDeal, dealHash)
			}

		} else {
			log.Error().Msgf("GetActiveAndExpiredDealsHash DealHahs empty %v", dealHash)
		}
	}
	node.DealMtx.RUnlock()
	fmt.Printf("\t\tNode - GetActiveAndExpiredDealsHash node.Activedeals size %v dealhashKeys :%v expire deal: %v I: %v \n", len(node.Activedeals), len(dealhashKeys), len(expiredDeal), i)
	return dealhashKeys, expiredDeal
}

func (node *Node) RemoveExpiredDealsWhileSync(blocktimestamp int64) {

	currentTime := time.Unix(0, blocktimestamp).Add(-20 * time.Second) // wait extra for 1 minning round
	for dealHash := range node.Activedeals {
		deal := node.GetDealbyHash(dealHash)
		if deal == nil {
			log.Error().Msgf("RemoveExpiredDealsWhileSync Reference deal cannot be found %v", hex.EncodeToString([]byte(dealHash)))
			continue
		}
		ExpiryTime := time.Unix(int64(deal.ExpiryTime), 0) // wait extra for 1 minning round
		if currentTime.After(ExpiryTime) {                 // TODO: DPOS time for block window, change it to block window time
			delete(node.Activedeals, dealHash)
		}
	}
}

//UpdateThreadBase ThreadBase is a map for all posts on the network
func (node *Node) UpdateThreadBase(b *core.Block) {

	deals := b.GetDeals()
	for _, deal := range deals {

		posts := deal.List
		for _, post := range posts {

			rootHash := hex.EncodeToString(post.Info.GetMetadata().GetThreadHeadPostHash())
			postInfobytes, err := gproto.Marshal(post.Info)
			if err != nil {
				log.Error().Msgf("Cannot unmarshal post..continuing to next post in deal")
				continue
			}
			postMetaDataHash := crypto.Sha256(postInfobytes)
			postMetaDataHashStr := hex.EncodeToString(postMetaDataHash)

			if rootHash == "" { // Root Post
				// Initiate a thread
				node.ThreadBaseMap.Store(postMetaDataHashStr, make([]string, 0))

				if node.User != nil {
					go node.User.GetThread(postMetaDataHashStr) // To make retrieval faster for now
				}
			} else {
				// Check if the root post of the reply exists
				val, ok := node.ThreadBaseMap.Load(rootHash)
				if !ok {
					log.Error().Msgf("Root Post for the reply does not exist..continuing to next post in deal")
					continue // move to nex post
				}
				// Add to the threads' posts
				newval := append(val.([]string), postMetaDataHashStr)
				node.ThreadBaseMap.Store(rootHash, newval)

				if node.User != nil {
					go node.User.GetThread(rootHash) // To make retrieval faster for now
				}
			}

		}

	}
}

// func (node *Node) RemoveExpiredDeals(blocktimestamp int64) {
func (node *Node) RemoveExpiredDeals(blocktimestamp int64, chPrP []*protobuf.ChPrPair) { // TODO: change it deals from block only
	log.Info().Msgf("Node -- RemoveExpiredDeals ")

	currentTime := time.Unix(0, blocktimestamp).Truncate(time.Second)
	for _, cProof := range chPrP {
		dealHash := string(cProof.Dealhash)
		deal := node.GetDealbyHash(dealHash)
		if deal == nil {
			log.Error().Msgf("RemoveExpiredDeals Reference deal cannot be found %v", hex.EncodeToString([]byte(dealHash)))
			continue
		}

		ExpiryTime := time.Unix(int64(deal.ExpiryTime), 0) // wait extra for 1 minning round
		if currentTime.After(ExpiryTime) {                 // TODO: DPOS time for block window, change it to block window time
			log.Info().Msgf("Node -- Removed Expired DealHahs %v Remaining %v", hex.EncodeToString([]byte(dealHash)), len(node.Activedeals))
			if node.StorageProvider != nil {
				node.StorageProvider.RemoveExpiredDealDataFromCahce(deal)
			}
			if node.Miner != nil {
				node.Miner.RemoveExpiredDealDataFromCahce(deal)
			}

			delete(node.Activedeals, dealHash)
		}
	}

}

func (node *Node) AddExpiredDealsWhileReverting(chPrP []*protobuf.ChPrPair) { // TODO: change it deals from block only
	log.Info().Msgf("Node -- RemoveExpiredDeals ")
	node.DealMtx.Lock()
	for _, cProof := range chPrP {

		_, err := node.Db.Get(storage.DealBucket, cProof.Dealhash)
		if err != nil {
			log.Error().Msgf("AddExpiredDealsWhileReverting - Unable to find Deal in db %v", hex.EncodeToString(cProof.Dealhash))
			continue
		} else {
			node.Activedeals[string(cProof.Dealhash)] = true
		}
		// }
	}
	node.DealMtx.Unlock()
}
func (node *Node) GetPosPairing() *pbc.Pairing {
	return node.PosPairing
}

func (node *Node) DeleteDealsFromDB() {
	node.Db.DeleteBucket(storage.DealBucket)
	err := node.Db.NewBucket(storage.DealBucket)
	if err != nil {
		log.Error().Err(err)
	}
}

func (node *Node) Close() {
	node.ntw.RemovePeers()
	node.ntw.Close()
}

func (node *Node) NewNetwork(host string) {
	ntw, err := network.New(host, node.Port, node.keys.PrivateKey, core.DefaultConfig.ChainID, nil, false)
	if err != nil {
		log.Error().Msgf("NewNetwork network Error:%v", err)
	}
	node.ntw = ntw
	go node.Listen()
}

//TakeAndStoreStateSnapShot takes the state snapshot and stores it in the stateStore
func (node *Node) TakeAndStoreStateSnapShot() error {
	//This should be an atomic operation
	fmt.Println("Taking and storing state snapshot...")
	_, err := node.Bc.GetBlockByHash(node.Bc.GetMainTail().Hash())
	if err != nil {
		panic(err.Error())

	}
	tail := node.Bc.GetMainTail()
	blockState := blockstate.NewBlockState(tail.Hash(), tail.Height(), node.Wm, node.Dpos)
	err = node.StateStore.PersistBlockState(blockState, node.Port)
	if err != nil {
		return err
	}
	fmt.Println("Snapshot taken")
	return nil
}

//GenerateStateSnapshot generates the state till the block provided
func (node *Node) GenerateStateSnapshot(bHash []byte) error {

	block, err := node.Bc.GetBlockByHash(bHash)
	if err != nil {
		return err
	}

	state, err := node.StateStore.GetClosestState(node.GetHexID(), node.Bc, block.Height())
	if err != nil {
		fmt.Println("state is nil.State snapshot failed to generate")
		return err
	}
	state.CWallets.SetNode(node) //Setting the node inteface in the wallets

	fmt.Println("Closest State fetched")
	stateBlock, err := node.Bc.GetBlockByHash(state.BlockHash)
	if err != nil {
		return err
	}

	parent := block
	toApply := []*core.Block{}
	for !node.Bc.IsInMainFork(parent) {
		toApply = append([]*core.Block{parent}, toApply...)

		parent, err = node.Bc.GetBlockByHash(parent.ParentHash())
		if err != nil {
			return err
		}
	}

	toRevert := []*core.Block{}
	for i := stateBlock.Height(); i > parent.Height(); i-- {
		b, err := node.Bc.GetBlockByHeight(i)
		if err != nil {
			return err
		}
		toRevert = append(toRevert, b)
	}

	err = state.CWallets.RevertBatchBlocks(toRevert)
	if err != nil {
		return err
	}
	if len(toRevert) > 0 {
		revertParent, err := node.Bc.GetBlockByHash(toRevert[len(toRevert)-1].ParentHash())
		if err != nil {
			return err
		}
		log.Info().Msgf("Reverting round till: %v", revertParent.GetRoundNum())
		state.CDpos.RevertDposTill(revertParent.GetRoundNum())
	}

	log.Info().Msgf("Total blocks to apply: %v", len(toApply))
	for _, block := range toApply {
		err := node.Services.UpdateDpos(block, state.CDpos)
		if err != nil {
			return err
		}
		err = state.CWallets.CheckAndUpdateWalletStates(block, block.MinerPublicKey().String())
		if err != nil {
			return err
		}
	}

	//////////////////////////////////////////////////////
	//Updating hash and height
	state.UpdateStateComp(bHash, block.Height())

	log.Info().Msgf("state has been updated to the given block: ", block.Height())
	log.Info().Msgf("caching the state component")
	node.StateStore.CacheStateComp(state)
	log.Info().Msgf("State has been cached")

	return nil
}

func (node *Node) AddLastChVerificationsToDb(lastVrf []int64, dealHash []byte) {
	var byteData [][]byte
	for _, lstVrf := range lastVrf {
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, uint64(lstVrf))
		byteData = append(byteData, b)
	}

	node.Db.Put(storage.LastChVerificationTimeBucket, dealHash, bytes.Join(byteData, []byte("timbre")))
}

func (node *Node) GetLastChVerificationsFromDb(dealHash []byte) []int64 {

	var LastVrf []int64
	dataBytes, err := node.Db.Get(storage.LastChVerificationTimeBucket, dealHash)
	if err != nil {
		log.Error().Msgf("GetLastChVerifications Unable to find Deal in db %v", hex.EncodeToString(dealHash))
		return nil
	}

	LastVrfbytes := bytes.Split(dataBytes, []byte("timbre"))
	for _, lstVrf := range LastVrfbytes {
		LastVrf = append(LastVrf, int64(binary.LittleEndian.Uint64(lstVrf)))
	}
	return LastVrf
}

func (node *Node) DeleteLastChVerificationsFromDb(dealHash []byte) {
	node.Db.Delete(storage.LastChVerificationTimeBucket, dealHash)
}

func (node *Node) SetNetworkChainID(chainID string) {
	node.ntw.SetChainID(chainID)
}

//BootstrapFromEpochBlock loads from the epoch block. Note it will note validate the epoch block
func (node *Node) BootstrapFromEpochBlock(b *core.Block) error {

	eState := b.PbBlock.State //Epoch state

	epochWm, err := wallet.NewWmFromDigestBlock(b, node.GetHexID())
	if err != nil {
		return err
	}

	err = node.Dpos.InjectEpochState(epochWm, eState.Miners, eState.NextRoundMiners, b.GetRoundNum(), eState.SlotsPassed)
	if err != nil {
		return err
	}

	node.Wm = epochWm

	return nil
}
