package roles

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/Nik-U/pbc"
	"github.com/golang/protobuf/proto"
	"github.com/guyu96/go-timbre/core"
	"github.com/guyu96/go-timbre/crypto"
	"github.com/guyu96/go-timbre/crypto/pos"
	"github.com/guyu96/go-timbre/log"
	"github.com/guyu96/go-timbre/net"
	"github.com/guyu96/go-timbre/protobuf"
	"github.com/guyu96/go-timbre/storage"
	"github.com/guyu96/go-timbre/wallet"
	"github.com/mbilal92/noise"
	"github.com/oasislabs/ed25519"
)

const (
	minerChanSize              = 32     // default miner channel buffer size
	soCacheSize                = 256    // default storage offer cache size
	prCacheSize                = 2000   // default post request cache size
	name                       = "Test" // TODO: change me plz!!, I am just a file name
	NumberOfDealToVerify       = 5      //NumberOfDealToVerify: default number of deal to Verify
	TimeToWaitForProof         = 5
	MaxBlockSize         int64 = 60000 //60KB
)

var (
	testBlockNonce int64
)

func init() {
	testBlockNonce = 0
}

// Discuss: the storage offers are never removed from the cache, what if a storage provider does not
// want to provide storage anymore

// Miner / forger is responsible for matching posts with storage providers, collecting votes, verifying storage deals and extending the blockchain by minting blocks.
type Miner struct {
	node          *net.Node
	incoming      chan net.Message
	startedMining bool

	soCache              *storage.LRU // storage offer cache
	prCache              *storage.LRU // post request cache
	spKadIdCache         *storage.LRU // storage provider kad ID cache
	postToPrimarySpCache *storage.LRU // temp store primary storage provider of post based on ParentHash

	localspReqListCacheMtx sync.Mutex                                            // ActiveDeal mutex
	localspReqListCache    map[string]map[string]*protobuf.ThreadPostRequestList // Map: (Spid) -> (thread) -> requestList
	msgMapMtx              sync.Mutex                                            // ActiveDeal mutex
	msgMap                 map[string][]byte
	storageLeftMtx         sync.Mutex        // ActiveDeal mutex
	storageLeft            map[string]uint32 // To keep track of pledged storage for a post
	threadPostingTimeMtx   sync.Mutex        // ActiveDeal mutex
	threadPostingTime      map[string]int64

	ReceivedChPrPairMtx sync.Mutex                    // ActiveDeal mutex
	ReceivedChPrPair    map[string]*protobuf.ChPrPair // Active Deals from blockchain
	ChPrPair            []*protobuf.ChPrPair
	deals               []*protobuf.Deal // New Deals

	transactions *storage.TransactionPool // Transactions

	staleSoCache               *storage.LRU
	stalePrCache               *storage.LRU
	ExitBlockLoop              chan struct{}
	ReceivedChPrPairForTesting map[string]*protobuf.ChPrPair
	StoreDataforTesting        bool
}

// NewMiner creates a new miner.
func NewMiner(node *net.Node) *Miner {
	miner := &Miner{
		node:          node,
		incoming:      make(chan net.Message, minerChanSize),
		startedMining: false,
	}

	miner.soCache, _ = storage.NewLRU(soCacheSize)
	miner.prCache, _ = storage.NewLRU(prCacheSize)
	miner.staleSoCache, _ = storage.NewLRU(prCacheSize)
	miner.stalePrCache, _ = storage.NewLRU(prCacheSize)

	miner.spKadIdCache, _ = storage.NewLRU(prCacheSize)
	miner.postToPrimarySpCache, _ = storage.NewLRU(prCacheSize)

	miner.localspReqListCache = make(map[string]map[string]*protobuf.ThreadPostRequestList)
	miner.msgMap = make(map[string][]byte)
	miner.storageLeft = make(map[string]uint32)
	miner.threadPostingTime = make(map[string]int64)
	miner.ReceivedChPrPair = make(map[string]*protobuf.ChPrPair)
	miner.transactions = storage.NewTransactionPool(3 * prCacheSize)
	node.AddOutgoingChan(miner.incoming)
	miner.ReceivedChPrPairForTesting = make(map[string]*protobuf.ChPrPair)
	miner.StoreDataforTesting = false

	return miner
}

//EmptyCaches empties all the caches on the miner side
func (miner *Miner) EmptyCaches() {
	miner.soCache, _ = storage.NewLRU(soCacheSize)
	miner.prCache, _ = storage.NewLRU(prCacheSize)
	miner.stalePrCache, _ = storage.NewLRU(prCacheSize)
	miner.staleSoCache, _ = storage.NewLRU(prCacheSize)
	miner.spKadIdCache, _ = storage.NewLRU(prCacheSize)
	miner.postToPrimarySpCache, _ = storage.NewLRU(prCacheSize)

	miner.localspReqListCache = make(map[string]map[string]*protobuf.ThreadPostRequestList)
	miner.msgMap = make(map[string][]byte)
	miner.storageLeft = make(map[string]uint32)
	miner.threadPostingTime = make(map[string]int64)
	miner.ReceivedChPrPair = make(map[string]*protobuf.ChPrPair)
	miner.transactions.EmptyPool()

}

//StartMiner will start the miner processes upon recieving the signal
func (miner *Miner) StartMiner() {
	go miner.CreateBlockLoop() //Will start to listen on prapare and create block signal from miner

	go func() {
		listeningSignal := miner.node.Dpos.GetMinerListeningChannel() //Will signal miner to start listening and accepting transactions and posts from the network
		miningSignal := miner.node.Dpos.GetMinerMiningChannel()

		for {
			select {
			case <-listeningSignal:
				ctx, _ := context.WithTimeout(context.Background(), (miner.node.Dpos.GetCommitteeTime())-1*1e9) //The time should be equivalent to the miner timeslot
				miner.StartListening(ctx)
			case <-miningSignal:
				miner.StartMining()
			}
		}
	}()

}

//StartListening starts listening once the committee starts with the node as miner
func (miner *Miner) StartListening(ctx context.Context) {
	//TODO stop these processes
	go miner.Process(ctx)
}

//Node returns the node type
func (miner *Miner) Node() *net.Node {
	return miner.node
}

//StartMining starts the mining processes which only need to run in miner's slot
func (miner *Miner) StartMining() {
	go miner.StartProcessingStoredPostRequests()
}

//PushTransaction pushes transaction in the cache
func (miner *Miner) PushTransaction(pbTrans *protobuf.Transaction) {
	miner.transactions.Push(pbTrans)

}

//Process processes the incoming messages
func (miner *Miner) Process(ctx context.Context) {
	if miner.startedMining == true {
		log.Info().Msgf("RETURNING from Miner process. It's already processing")
		return
	}
	miner.startedMining = true

	for {
		select {
		case <-ctx.Done():
			log.Info().Msgf("Stopping miner process")
			miner.startedMining = false
			return
		case msg := <-miner.incoming:
			switch msg.Code {
			case net.MsgCodeBroadcastTest:
				log.Info().Msgf("BroadCastMsg from %v msg %v", msg.From, string(msg.Data))
			case net.MsgCodeRelayTest:
				log.Info().Msgf("BroadCastMsg from %v msg %v", msg.From, string(msg.Data))
			case net.MsgCodeStorageOfferNew:
				go miner.handleStorageOffer(msg)
			case net.MsgCodePostNew:
				miner.handlePostRequest(msg)
			case net.MsgCodeStorageProviderPostStored:
				miner.handleSpPostRequestStoreReply(msg)
			case net.MsgCodeStorageProviderPostNotStored:
				go miner.handleSpSaysPostNotStored(msg)

			case net.MsgTx:
				go miner.handleRelayedAmountTx(msg)

			case net.MsgCodeSpChallengeProf2:
				go miner.HandleSpChallengeProf(msg)
			}
		}
	}
}

// onStorageOffer stores the storage offer to cache
func (miner *Miner) handleStorageOffer(msg net.Message) {

	ssto := new(protobuf.SignedStorageOffer)
	if err := proto.Unmarshal(msg.Data, ssto); err != nil {
		log.Info().Msgf("Miner: strogeOffer request Unmarshal Error:%v", err)
		return
	}

	if err := miner.verifyStorageOffer(ssto); err != nil {
		log.Error().Msgf("Miner: Failed to verify StorageOffer: %s", err)
		return
	}

	// Only the latest storage offer by any sp is kept
	miner.soCache.Add(string(ssto.Offer.Spid), ssto.Offer)
	miner.spKadIdCache.Add(string(ssto.Offer.Spid), msg.From)

	log.Info().Msgf("Miner: StorageOffer Saved")
}

// verifyStorageOffer verifies the offer based on hash and sign
func (miner *Miner) verifyStorageOffer(st *protobuf.SignedStorageOffer) error {
	offer := st.Offer
	if offer.MinPrice <= 0 { // QUESTION: set an upper bound of price?
		return fmt.Errorf("Miner: StorageOffer Price < 0")
	}

	if offer.Size <= 0 { // QUESTION: set lower bound of space?
		return fmt.Errorf("Miner: StorageOffer MaxStorage < 0")
	}

	offerBytes, _ := proto.Marshal(offer)
	offerBytes = append(offerBytes, []byte(core.DefaultConfig.ChainID)...)
	if !ed25519.Verify(offer.Spid, offerBytes, st.Sig) {
		return fmt.Errorf("Miner: Invalid StorageOffer signature")
	}

	return nil
}

// onPostRequest verify post, find storage provider and send
func (miner *Miner) handlePostRequest(msg net.Message) { //queryID uint64
	pr := new(protobuf.PostRequest)
	if err := proto.Unmarshal(msg.Data, pr); err != nil {
		log.Error().Msgf("Miner: PostRequest Unmarshal Error:%v", err)
		return
	}

	if err := miner.verifyPostRequest(pr); err != nil {
		panic(fmt.Sprintf("Miner: Failed to verify PostRequest: %s", err))
	}

	// PosterAdd is the public key
	posterAdd := hex.EncodeToString(pr.SignedInfo.Info.Metadata.GetAuthorAddress()) // This would be the oublic key
	chargedAmount := pr.SignedInfo.Info.GetParam().GetMaxCost()

	miner.node.Wm.CheckAndPutWalletByAddress(posterAdd, wallet.NewWallet(posterAdd)) //Putting miner in map if not present
	pWallet, err := miner.node.Wm.GetWalletByAddress(posterAdd)                      //for poster
	if err != nil {
		panic(fmt.Sprintf("Miner: Wallet error for :%v, %s", posterAdd, err))
		return
	}

	if pWallet.GetBalance()-1000 < int64(chargedAmount) {
		log.Error().Msgf("Miner: Insufficient balance for poster :%v", posterAdd)
		newMsg := net.NewMessage(net.MsgCodeInsufficientBalanceToPost, msg.Data)
		miner.node.Relay(msg.From, newMsg)
		return
	}

	hashbytes, _ := proto.Marshal(pr.SignedInfo.Info)
	hash := crypto.Sha256(hashbytes)
	threadheadPostHash := pr.SignedInfo.Info.Metadata.ThreadHeadPostHash
	if threadheadPostHash == nil { // if it is a new post then set threadheadPostHash to Hash of metadata
		threadheadPostHash = hash
	}

	dbpost, err := miner.node.Db.Get(storage.PostStoredBucket, hash)
	_ = dbpost
	if err == nil {
		return
	} else {
		postTime := time.Unix(pr.SignedInfo.Timestamp, 0)
		currentTime := time.Now().Add(-20 * time.Second)
		if postTime.Before(currentTime) {
			log.Error().Msgf("Miner: Lost Post received :%v", posterAdd)
			return
		}
	}

	miner.threadPostingTimeMtx.Lock()
	if tpt, ok := miner.threadPostingTime[string(threadheadPostHash)]; ok {
		threadExpiryTime := time.Unix(tpt, 0)
		if time.Now().Truncate(time.Second).After(threadExpiryTime) {
			log.Info().Msgf("Miner: Skipping Post because of thread expired already, %v, thread hash %v, Current Time %v, Expiry Time %v",
				hex.EncodeToString(hash), hex.EncodeToString(threadheadPostHash), time.Now(), threadExpiryTime)
			newMsg := net.NewMessage(net.MsgCodePostThreadExpired, msg.Data)
			miner.node.Relay(msg.From, newMsg)
			miner.threadPostingTimeMtx.Unlock()
			return
		}
	}
	miner.threadPostingTimeMtx.Unlock()
	miner.prCache.Add(string(hash), pr)

}

//TODO: add method which handles reply in case of provider rejects the post request
// remove post request from cache and create a deal
func (miner *Miner) handleSpPostRequestStoreReply(msg net.Message) {
	// log.Info().Msgf("Miner: handleSpPostRequestStoreReply")

	sInfolist := new(protobuf.SignedInfoList)
	if err := proto.Unmarshal(msg.Data, sInfolist); err != nil {
		log.Error().Msgf("Miner: SignedInfoList Unmarshal Error:%v", err)
		return
	}

	for _, threadInfolist := range sInfolist.List {
		spIDstring := string(threadInfolist.Spid)
		if !ed25519.Verify(threadInfolist.Spid, crypto.HashStructs(threadInfolist.InfoList), threadInfolist.Sig) {
			log.Error().Msgf("Miner: Invalid SignedInfoList signature")
			return
		}

		if threadInfolist.InfoList != nil {

			threadheadPostHash := threadInfolist.InfoList[0].Info.Metadata.ThreadHeadPostHash
			if threadheadPostHash == nil { // if it is a new post then set threadheadPostHash to Hash of metadata
				threadheadPostHashbytes, _ := proto.Marshal(threadInfolist.InfoList[0].Info)
				threadheadPostHash = crypto.Sha256(threadheadPostHashbytes)
			}

			storageUsed := uint32(0)
			// Remove post request from cache
			for _, pr_item := range threadInfolist.InfoList {
				hashbytes, _ := proto.Marshal(pr_item.Info)
				hash := crypto.Sha256(hashbytes)
				miner.prCache.Remove(string(hash))
				miner.stalePrCache.Remove(string(hash))
				storageUsed += pr_item.Info.Metadata.GetContentSize()
			}

			miner.threadPostingTimeMtx.Lock()
			if _, ok := miner.threadPostingTime[string(threadheadPostHash)]; !ok {
				for _, infopr := range threadInfolist.InfoList {
					if infopr.Info.Metadata.ThreadHeadPostHash == nil {
						threadheadPostHashbytes, _ := proto.Marshal(infopr.Info)
						threadheadPostHash = crypto.Sha256(threadheadPostHashbytes)
						postingTime := time.Now()
						ExpiryTime := postingTime.Add(time.Second * time.Duration(net.DealExpirtTime)) //postingTime.Add(time.Second * time.Duration(int(infopr.Info.Param.MinDuration)))
						miner.threadPostingTime[string(threadheadPostHash)] = ExpiryTime.Unix()
						break
					}
				}
			}
			miner.threadPostingTimeMtx.Unlock()
			miner.msgMapMtx.Lock()
			data := miner.msgMap[string(threadheadPostHash)]
			miner.msgMapMtx.Unlock()
			if len(data) < 150 {
				j := len(data)
				for i := 0; i < 150-j; i++ {
					data = append(data, []byte("0")...)
				}
			}

			tags := pos.Setup(data, []byte(name), miner.node.PosPairing, miner.node.PosPk, miner.node.PosSk, miner.node.PosConfig)
			miner.threadPostingTimeMtx.Lock()
			threadPostingTimeL := miner.threadPostingTime[string(threadheadPostHash)]
			miner.threadPostingTimeMtx.Unlock()

			deal := &protobuf.Deal{
				Spid:       threadInfolist.Spid,
				SpSig:      threadInfolist.Sig,
				List:       threadInfolist.InfoList,
				PublicKey:  miner.node.PosPk.ToProto(),
				Timestamp:  time.Now().Unix(),
				ExpiryTime: threadPostingTimeL,
			}

			Dealbytes, _ := proto.Marshal(deal)
			dealHash := crypto.Sha256(Dealbytes)

			log.Info().Msgf("Miner --- Deal Made! -- DealHash: %v ", hex.EncodeToString(dealHash))

			dealTagPair := &protobuf.DealTagPair{
				DealHash: dealHash,
				Tags:     tags.ToByteArray(),
			}
			pbKey := new(pos.PublicKey)
			pbKey.FromProto(deal.PublicKey, miner.node.PosPairing)

			dealTagPairBytes, _ := proto.Marshal(dealTagPair)
			newMsg := net.NewMessage(net.MsgCodeReceiveVarificationTags, dealTagPairBytes)
			miner.node.Relay(msg.From, newMsg)

			miner.msgMapMtx.Lock()
			delete(miner.msgMap, string(threadheadPostHash))
			miner.msgMapMtx.Unlock()

			miner.localspReqListCacheMtx.Lock()
			delete(miner.localspReqListCache[spIDstring], string(threadheadPostHash))
			miner.localspReqListCacheMtx.Unlock()

			miner.deals = append(miner.deals, deal)
		}

		if threadInfolist.InfoListNotStoredPost != nil {
			threadheadPostHash := threadInfolist.InfoListNotStoredPost[0].Info.Metadata.ThreadHeadPostHash
			if threadheadPostHash == nil { // if it is a new post then set threadheadPostHash to Hash of metadata
				threadheadPostHashbytes, _ := proto.Marshal(threadInfolist.InfoListNotStoredPost[0].Info)
				threadheadPostHash = crypto.Sha256(threadheadPostHashbytes)
			}

			for _, pr_item := range threadInfolist.InfoList {
				postbytes, _ := proto.Marshal(pr_item)
				var posterID noise.PublicKey
				copy(posterID[:], pr_item.Info.Metadata.Pid)
				miner.node.Wm.RevertPreValidatePostWalletCache(pr_item)
				newMsg := net.NewMessage(net.MsgCodePostNotStored, postbytes) // TODO: find another storage provider foe these posts
				miner.node.RelayToPB(posterID, newMsg)
			}

			miner.msgMapMtx.Lock()
			delete(miner.msgMap, string(threadheadPostHash))
			miner.msgMapMtx.Unlock()

			miner.localspReqListCacheMtx.Lock()
			delete(miner.localspReqListCache[spIDstring], string(threadheadPostHash))
			miner.localspReqListCacheMtx.Unlock()
		}

	}
}

func (miner *Miner) handleSpSaysPostNotStored(msg net.Message) {
	log.Info().Msgf("Post not stored at %v , find new provider %v ", msg.From.String(), msg.From.ID.String())
	log.Info().Msgf("SPID handleSpSaysPostNotStored %v %v ", msg.From.ID, msg.From.ID.String())
	// Spid id of the sp goes into the staleCache and is only removed on deal expiration
	spid, _ := hex.DecodeString(msg.From.ID.String())
	val, exist := miner.soCache.Get(string(spid))
	if !exist {
		log.Info().Msgf("Storage Provider doesnt exist in the storage offers cache")
		return
	}
	miner.staleSoCache.Add(string(spid), val)
}

func (miner *Miner) getStorageOfferValues() []*protobuf.StorageOffer {
	// Filter the ones which are in staleSoCache -> they are kept on KadID though
	var values []*protobuf.StorageOffer
	for _, key := range miner.soCache.Keys() {
		val, keyExist := miner.soCache.Get(key)
		if keyExist && !miner.staleSoCache.Contains(key) {
			values = append(values, val.(*protobuf.StorageOffer))
		}
	}
	return values
}

func (miner *Miner) getPostRequestsValues() []*protobuf.PostRequest {
	// Filter the ones which are in staleSoCache -> they are kept on KadID though
	var values []*protobuf.PostRequest
	for _, key := range miner.prCache.Keys() {
		val, keyExist := miner.prCache.Get(key)
		if keyExist {
			values = append(values, val.(*protobuf.PostRequest))
		}
	}
	return values
}

func (miner *Miner) verifyPostRequest(pr *protobuf.PostRequest) error {

	posterPubKey := pr.SignedInfo.Info.Metadata.Pid
	if pr.SignedInfo.Info.Param.MaxCost < 0 {
		return fmt.Errorf("Miner: Invalid MaxCost from poster: %s", posterPubKey)
	}

	if infoBytes, err := proto.Marshal(pr.SignedInfo.Info); err != nil {
		panic(fmt.Sprintf("Failed to marshal SimpleRequest: %s", err))
	} else {
		infoBytes = append(infoBytes, []byte(core.DefaultConfig.ChainID)...)
		if !crypto.Verify(posterPubKey, infoBytes, pr.SignedInfo.Sig) {
			return fmt.Errorf("Invalid PostRequest signature")
		}

		timestampBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(timestampBytes, uint64(pr.SignedInfo.Timestamp))
		if !crypto.Verify(posterPubKey, timestampBytes, pr.SignedInfo.TimeStampSig) {
			return fmt.Errorf("Invalid PostRequest TimeStamp signature ")
		}
	}

	return nil
}

func (miner *Miner) handleRelayedAmountTx(msg net.Message) {
	aTrans, err := core.TransactionFromBytes(msg.Data)
	if err != nil {
		log.Error().Err(err)
		return
	}
	pbTransaction := aTrans.GetProtoTransaction()
	miner.transactions.Push(pbTransaction)
	log.Info().Msgf("transaction added to the cache")
}

//UpdateCache updates the cache by removing the redundent transactions(already added in block) from the cache
func (miner *Miner) UpdateCache(block *core.Block) {
	var err error

	blockTransactions := block.GetTransactions()
	interfaceTransactions := make([]interface{}, len(blockTransactions))
	for x := range blockTransactions {
		interfaceTransactions[x] = blockTransactions[x]
	}
	err = miner.transactions.RemoveRedundantTx(interfaceTransactions)
	if err != nil {
		// panic(err)
		log.Warn().Msgf(err.Error())
	}

	for _, deal := range block.GetDeals() {

		for _, infopr := range deal.List {
			hashbytes, _ := proto.Marshal(infopr.Info)
			hash := crypto.Sha256(hashbytes)
			miner.prCache.Remove(string(hash))
			miner.stalePrCache.Remove(string(hash))
			miner.node.Db.Put(storage.PostStoredBucket, hash, []byte("0"))
		}

		threadheadPostHash := deal.List[0].Info.Metadata.ThreadHeadPostHash
		if threadheadPostHash == nil {
			threadheadPostHashbytes, _ := proto.Marshal(deal.List[0].Info)
			threadheadPostHash = crypto.Sha256(threadheadPostHashbytes)
		}

		miner.threadPostingTimeMtx.Lock()
		if _, ok := miner.threadPostingTime[string(threadheadPostHash)]; !ok {
			miner.threadPostingTime[string(threadheadPostHash)] = deal.ExpiryTime
		}
		miner.threadPostingTimeMtx.Unlock()
	}

	miner.ReceivedChPrPairMtx.Lock()
	for _, cProof := range block.GetChPrPair() {
		dealHash := string(cProof.Dealhash)
		delete(miner.ReceivedChPrPair, dealHash)

	}
	miner.ReceivedChPrPairMtx.Unlock()

}

//PrintPostStats prints the post stats
func (miner *Miner) PrintPostStats() {

	fmt.Println("Miner: Found Offers ,", miner.soCache.Size(), " PostRequest Cache: ", miner.prCache.Size())
	fmt.Println("Active deals: ", len(miner.node.Activedeals))
	// fmt.Println("Active deals: ", miner.node.Activedeals)
	fmt.Println("Miner: localspReqListCache ", len(miner.localspReqListCache), " msgMap ", len(miner.msgMap))
	// fmt.Println("Miner: localspReqListCache ", miner.localspReqListCache, " msgMap ", miner.msgMap)
	fmt.Println("Miner: threadPostingTime ", len(miner.threadPostingTime))
	// fmt.Println("Miner: threadPostingTime ", miner.threadPostingTime)
	fmt.Println("Miner: ReceivedChPrPair ", len(miner.ReceivedChPrPair), ", ChPrPair: ", len(miner.ChPrPair), ", Deal ", len(miner.deals))
	// fmt.Println("Miner: ReceivedChPrPair ", miner.ReceivedChPrPair, ", ChPrPair: ", miner.ChPrPair, ", Deal", miner.deals)
	fmt.Println("Miner: staleSoCache : ", miner.staleSoCache.Size(), ", stalePrCache Cache: ", miner.stalePrCache.Size())
	// fmt.Println("Miner: staleSoCache : ", miner.staleSoCache, ", stalePrCache Cache: ", miner.stalePrCache)
	miner.node.Wm.PrintWalletBalances()

}

// StartProcessingStoredPostRequests process postRequest that are stored temporarily in cache
func (miner *Miner) StartProcessingStoredPostRequests() {

	time.Sleep(3 * time.Second) // to process incoming block // should be based on signal/channel

	// miner.node.Wm.ResetWalletCache()
	miner.deals = nil
	miner.ChPrPair = nil
	log.Info().Msgf("Miner: StartProcessingStoredPostRequests Exit")
}

//TestRelayToSp tests relay msg to the storage provider
func (miner *Miner) TestRelayToSp() {
	fmt.Println("Testing sp relay msg...")
	if miner.spKadIdCache.Size() == 0 {
		fmt.Println("No one to test relay to.")
		return
	}

	id, got := miner.spKadIdCache.Pop()
	if got != nil {
		fmt.Println("Didn't get id")
		return
	}

	bs := []byte("This is a test msg" + time.Now().String())
	log.Info().Msgf("Relaying test msg %v", id.(noise.ID))
	msg := net.NewMessage(net.TestMsg, bs)
	miner.node.Relay(id.(noise.ID), msg)

}

//GetDelegateRegTx returns the DR tx from the cache
func (miner *Miner) GetDelegateRegTx() []*protobuf.Transaction {
	var delegateTx []*protobuf.Transaction
	delSize := miner.transactions.LengthOfPool()
	for i := 0; i < delSize; i++ {
		val, _ := miner.transactions.Pop()
		aTrans := val.(*protobuf.Transaction)
		if aTrans.GetType() == core.TxBecomeCandidate {
			delegateTx = append(delegateTx, aTrans)
		} else {
			miner.transactions.Push(aTrans)

		}

	}
	return delegateTx
}

//CheckCandWithDelRegTx checks the delegate reg tx
func (miner *Miner) CheckCandWithDelRegTx(delTx []*protobuf.Transaction) error {
	if len(delTx) != miner.node.Dpos.State.TotalCandidates() { //Should also include itself's
		fmt.Println("tx: ", len(delTx), "cand: ", miner.node.Dpos.State.TotalCandidates())
		panic("Some del-reg tx have been missed. Re-run")
	}
	for _, tx := range delTx {
		fmt.Println("id is: ", tx.GetBody().GetSenderId())
		if !miner.node.Dpos.State.CheckCandExistance(tx.GetBody().GetSenderId()) {
			panic("Some cand announcement has been missed")
		}
	}

	return nil
}

// GenerateBlock creats the blocks containing the amount tx,votes, proofs and deals
func (miner *Miner) GenerateBlock(transactions []*protobuf.Transaction, votes []*protobuf.Vote, deals []*protobuf.Deal, chPrPair []*protobuf.ChPrPair, podfs []*protobuf.PoDF, roundNum, bTs int64, broadcast bool) *core.Block {

	var block *core.Block
	var err error
	node := miner.node
	node.BcMtx.Lock()
	defer node.BcMtx.Unlock()
	minerPk := miner.node.PublicKey()
	minerAdd := minerPk.String()

	if node.Bc.Genesis() == nil || (core.DefaultConfig.Revolt && node.Bc.IsRevoltingTip()) { //if no genesis block
		log.Info().Msgf("Creating genesis block")
		trans := miner.GetDelegateRegTx()
		err := miner.CheckCandWithDelRegTx(trans)
		if err != nil {
			fmt.Println(err.Error())
			return nil
		}

		if broadcast == false {
			if core.DefaultConfig.Revolt {
				block, err = core.NewGenesisBlockWithTs(core.DefaultConfig.RevoltBlockHash, minerPk, miner.node.TimeOffset, trans, bTs, 0)
			} else {
				block, err = core.NewGenesisBlockWithTs(nil, minerPk, miner.node.TimeOffset, trans, bTs, 0)
			}
		} else {
			if core.DefaultConfig.Revolt {
				block, err = core.NewGenesisBlock(core.DefaultConfig.RevoltBlockHash, minerPk, miner.node.TimeOffset, trans, 0)
			} else {
				block, err = core.NewGenesisBlock(nil, minerPk, miner.node.TimeOffset, trans, 0)
			}
		}

		block.Sign(node.PrivateKey())

	} else {
		parent := node.Bc.GetMainTail()
		if broadcast == false {
			block = core.CreateNewBlockWithTs(parent, node.PublicKey(), node.PrivateKey(), transactions, votes, deals, chPrPair, podfs, miner.node.TimeOffset, roundNum, bTs) //Extra transaction variable as comp to MakeTestBlock
		} else {
			var tchPrPair []*protobuf.ChPrPair
			for _, chPr := range miner.ReceivedChPrPairForTesting {

				chPr.Timestamp = time.Now().Unix()
				tchPrPair = append(tchPrPair, chPr)
			}
			block = core.CreateNewBlock(parent, node.PublicKey(), node.PrivateKey(), transactions, votes, deals, tchPrPair, podfs, miner.node.TimeOffset, roundNum) //Extra transaction variable as comp to MakeTestBlock
		}
	}

	isValid := block.IsValid()
	if isValid != true {
		log.Info().Msgf("Block is not valid. Returning")
		return nil
	}

	err = node.Wm.CheckAndUpdateWalletStates(block, minerAdd) // only added validated transaction
	if err != nil {
		log.Error().Msgf("Miner -- Wallet state could not be updated")
		log.Error().Msgf(err.Error())
	}

	miner.node.AddDeals(deals, true)
	for _, deal := range deals {
		for _, infopr := range deal.List {
			hashbytes, _ := proto.Marshal(infopr.Info)
			hash := crypto.Sha256(hashbytes)
			miner.prCache.Remove(string(hash))
			miner.stalePrCache.Remove(string(hash))
			miner.node.Db.Put(storage.PostStoredBucket, hash, []byte("0"))
		}
	}
	miner.node.RemoveExpiredDeals(block.GetTimestamp(), block.GetChPrPair())
	miner.node.Bc.UpdateIrreversibleBlockNumber(minerAdd, miner.node.Dpos.GetComitteeSize(), miner.node.Dpos.NumberOfToMineBlockPerSlot, miner.node.Port)
	if miner.node.StorageProvider != nil {
		miner.node.StorageProvider.ProcessNewBlock(block)
	}
	if miner.node.User != nil {
		miner.node.UpdateThreadBase(block)
	}

	if err != nil {
		log.Error().Err(err)
		log.Error().Msgf("Miner -- Block Mined but Not added")
		return nil
	}

	blockBytes, err := block.ToBytes()
	if err != nil {
		log.Error().Err(err)
		return nil
	}
	if broadcast == false {
		log.Info().Msgf("Prepared %s", block)
	} else {
		log.Info().Msgf("CREATED %s", block)
	}
	node.Bc.Append(block)
	node.Bc.UpdateMainTailIndex()
	log.Info().Msgf("New Blockchain: %s", node.Bc)
	miner.node.Wm.ResetWalletCache()
	miner.ChPrPair = nil

	if broadcast == true {
		msg := net.NewMessage(net.MsgCodeBlockNew, blockBytes)
		node.Broadcast(msg)
	}

	return block
}

//GenerateDigestBlock generates the digest
func (miner *Miner) GenerateDigestBlock(roundNum, bTs int64, broadcast bool) (*core.Block, error) {
	fmt.Println("Creating digest block instead")
	var block = new(core.Block)
	var err error

	node := miner.node
	node.BcMtx.Lock()
	defer node.BcMtx.Unlock()

	if node.Bc.IsEmpty() {
		return nil, errors.New("Blockchain is nil")
	}
	parent := node.Bc.GetMainTail()

	accEntries := miner.node.Wm.TotalWallets()
	block, err = core.CreateEmptyBlock(parent, node.PublicKey(), miner.node.TimeOffset, roundNum, bTs, accEntries)
	if err != nil {
		return nil, err
	}
	bcHeight := miner.node.Bc.GetMainForkHeight()
	curEpochNum := float64(bcHeight) / float64(core.TestEpoch.Duration)
	miner.node.Wm.CleanAndAddBlockState(block)                        //Adding the balances of the users
	block.PbBlock.State.Miners = miner.node.Dpos.GetMinersAddresses() //Setting the miner addresses
	block.PbBlock.State.NextRoundMiners = miner.node.Dpos.GetMinersForNextRound()
	block.PbBlock.State.SlotsPassed = miner.node.Dpos.GetTailSlotsPassed()
	block.PbBlock.State.Epoch = int32(curEpochNum)

	err = block.HashAndSign(node.PrivateKey())
	if err != nil {
		return nil, err
	}

	isValid := block.IsValid()
	if isValid != true {
		log.Info().Msgf("Block is not valid. Returning")
		return nil, errors.New("Invalid block")
	}
	blockBytes, err := block.ToBytes()
	fmt.Println("Digest Block bytes:-", len(blockBytes))
	if err != nil {
		log.Error().Err(err)
		return nil, errors.New("Error converting block to bytes")
	}

	node.Bc.Append(block)
	log.Info().Msgf("New Blockchain: %s", node.Bc)
	node.Bc.UpdateMainTailIndex()

	miner.deals = nil
	miner.ChPrPair = nil
	if broadcast == true {
		msg := net.NewMessage(net.MsgCodeBlockNew, blockBytes)
		node.Broadcast(msg)
	}

	minerPk := block.MinerPublicKey()
	minerAdd := minerPk.String()
	err = node.Wm.CheckAndUpdateWalletStates(block, minerAdd) //Skip/remove transactions from block which are invalid
	if err != nil {
		log.Error().Msgf("Miner -- Wallet state could not be updated")
		log.Error().Msgf(err.Error())
	}
	miner.node.EmptyRoleCaches()

	return block, nil
}

//PrepareBlock prepares the blocks
func (miner *Miner) PrepareBlock(roundNum int64) *core.Block {
	bTs := miner.node.Dpos.NextTickTime(time.Now().Unix())
	if miner.node.Bc.IsEmpty() || (core.DefaultConfig.Revolt && miner.node.Bc.IsRevoltingTip()) {
		var transactions []*protobuf.Transaction
		var voteList []*protobuf.Vote
		var deals []*protobuf.Deal

		b := miner.GenerateBlock(transactions, voteList, deals, nil, nil, roundNum, bTs*1e9, false)
		return b
	}
	bcHeight := int64(miner.node.Bc.GetMainForkHeight())
	if bcHeight%core.TestEpoch.Duration == 0 && bcHeight != 0 {
		//Time to mine a digest block
		b, err := miner.GenerateDigestBlock(roundNum, bTs*1e9, false)
		if err != nil {
			log.Info().Err(err)
		}
		return b
	}

	miner.GenerateChallenges(int64(binary.BigEndian.Uint64(miner.node.Bc.GetMainTail().Hash()[0:8])))
	deals, proofs, transactions, votes, podfs := miner.FillBlock()
	b := miner.GenerateBlock(transactions, votes, deals, proofs, podfs, roundNum, bTs*1e9, false)
	return b
}

//CreateBlockLoop creates block when the ticker notifies in DPOS
func (miner *Miner) CreateBlockLoop() {

	prepared := false
	var block *core.Block = nil

	createBlock := miner.node.Dpos.GetMinerCreateBlockChannel()
	prepareBlock := miner.node.Dpos.GetMinerPreparingChannel()

	for {
		select {
		case <-miner.ExitBlockLoop:
			return
		case <-createBlock: //This signal will be given from dpos

			if prepared == true {

				if block != nil { //Make sure block is not stale

					if block.GetTimestamp() < (time.Now().UnixNano() - 2*int64(miner.node.Dpos.BlockPrepTime().Nanoseconds())) {
						fmt.Println("Got old prepared block. Removing it")
						block = nil //Removing the stale block
						prepared = false
					} else {
						log.Info().Msgf("Broadcasting block now!!!")
						log.Info().Msgf("Block: %v", block)
						blockBytes, err := block.ToBytes()
						if err != nil {
							log.Error().Err(err)
							continue
						}
						msg := net.NewMessage(net.MsgCodeBlockNew, blockBytes)
						miner.node.Broadcast(msg)
						miner.ResetCache()

						block = nil //Resetting the block
						prepared = false

						continue
					}
				}
			}

			if miner.node.Bc.IsEmpty() {
				var transactions []*protobuf.Transaction
				var voteList []*protobuf.Vote
				var deals []*protobuf.Deal
				miner.GenerateBlock(transactions, voteList, deals, nil, nil, miner.node.Dpos.GetRoundNum(), time.Now().UnixNano(), true)
				continue
			}
			bcHeight := int64(miner.node.Bc.GetMainForkHeight())

			if bcHeight%core.TestEpoch.Duration == 0 && bcHeight != 0 {
				//Time to mine a digest block

				_, err := miner.GenerateDigestBlock(miner.node.Dpos.GetRoundNum(), time.Now().UnixNano(), true)
				if err != nil {
					log.Info().Err(err)
				}
				continue
			}

			log.Info().Msgf("Let's mine")

			deals, proofs, transactions, votes, podfs := miner.FillBlock()
			miner.GenerateBlock(transactions, votes, deals, proofs, podfs, miner.node.Dpos.GetRoundNum(), time.Now().UnixNano(), true)
			miner.deals = nil
			miner.ChPrPair = nil

		case <-prepareBlock:

			if miner.node.Dpos.GetCurMinerAdd() == miner.node.GetHexID() { //once again checking for the current miner
				if block != nil && block.GetTimestamp() < (time.Now().UnixNano()-miner.node.Dpos.GetTickInterval().Nanoseconds()) {
					fmt.Println("Has already prepared a block. Will use that one")
					continue
				}
				block = miner.PrepareBlock(miner.node.Dpos.GetRoundNum())
				if block != nil {
					prepared = true
				}
			}
			// }
		}
	}
}

//FillBlock fills the block in ascending order till the limit is reached
func (miner *Miner) FillBlock() ([]*protobuf.Deal, []*protobuf.ChPrPair, []*protobuf.Transaction, []*protobuf.Vote, []*protobuf.PoDF) {
	//0.1 mb size
	log.Info().Msgf("FILLING BLOCK")
	type BlockObject int
	var dealTy, proofTy, txTy BlockObject = 0, 1, 2
	limit := MaxBlockSize
	if limit > MaxBlockSize {
		limit = MaxBlockSize //Ensuring that the limit is not greater than the block
	}

	curSize := 0
	var bTrans []*protobuf.Transaction
	var bVotes []*protobuf.Vote
	var bDeals []*protobuf.Deal
	var bProofs []*protobuf.ChPrPair
	var bPodfs []*protobuf.PoDF

	for {
		if int64(curSize) >= limit || miner.AllEmptyCaches() == true {
			break
		}

		var lowestTs int64 = math.MaxInt64
		var toChoose BlockObject //Var for choosing which object

		if len(miner.deals) > 0 {
			firstDeal := miner.deals[0]
			if lowestTs > firstDeal.GetTimestamp() {
				toChoose = dealTy
				lowestTs = firstDeal.GetTimestamp()
			}
		}
		if len(miner.ChPrPair) > 0 {
			firstProof := miner.ChPrPair[0]
			if lowestTs > firstProof.GetTimestamp() {
				toChoose = proofTy
				lowestTs = firstProof.GetTimestamp()

			}
		}
		if miner.transactions.LengthOfPool() > 0 {
			txhead, err := miner.transactions.GetHead()
			if err != nil {
				continue
			}
			firstTx := txhead.(*protobuf.Transaction)
			if lowestTs > firstTx.Body.GetTimeStamp() {
				toChoose = txTy
				lowestTs = firstTx.Body.GetTimeStamp()

			}
		}

		bRound := miner.node.Dpos.GetRoundNum()
		switch toChoose {
		case dealTy:
			err := miner.node.Wm.ValidateDeal(miner.deals[0], bRound)
			if err != nil {
				log.Info().Err(err)
				miner.deals = miner.deals[1:] //Removing deal
				continue
			}
			bDeals = append(bDeals, miner.deals[0])
			curSize += miner.deals[0].XXX_Size()
			miner.deals = miner.deals[1:] //Removing deal

		case proofTy:
			bProofs = append(bProofs, miner.ChPrPair[0])
			curSize += miner.ChPrPair[0].XXX_Size()
			miner.ChPrPair = miner.ChPrPair[1:]

		case txTy:
			val, _ := miner.transactions.Pop()
			aTrans := val.(*protobuf.Transaction)
			fmt.Println("popped tx: ", aTrans.GetType())

			valid, err := miner.node.Wm.IsExecutable(aTrans, bRound)
			if valid != true || err != nil {
				log.Error().Msgf(err.Error())
				continue
			}
			fmt.Println("ADDING tx: ", aTrans.GetType())
			bTrans = append(bTrans, aTrans)
			curSize += aTrans.XXX_Size()

		default:
			continue
		}

	}
	return bDeals, bProofs, bTrans, bVotes, bPodfs
}

//AllEmptyCaches returns true in case all caches are emptu
func (miner *Miner) AllEmptyCaches() bool {
	if miner.transactions.IsEmpty() && len(miner.deals) == 0 && len(miner.ChPrPair) == 0 {
		return true
	}
	return false
}

func (miner *Miner) HandleSpChallengeProf(msg net.Message) {
	miner.ReceivedChPrPairMtx.Lock()
	defer miner.ReceivedChPrPairMtx.Unlock()

	challengeProofPair := new(protobuf.ChPrPair)
	if err := proto.Unmarshal(msg.Data, challengeProofPair); err != nil {
		log.Info().Msgf("Miner: challengeProofPair Unmarshal Error:%v", err)
		return
	}

	log.Info().Msgf("Miner: Received chprpair for deal :%v", hex.EncodeToString(challengeProofPair.Dealhash))
	miner.ReceivedChPrPair[string(challengeProofPair.Dealhash)] = challengeProofPair
	if miner.StoreDataforTesting {
		miner.ReceivedChPrPairForTesting[string(challengeProofPair.Dealhash)] = challengeProofPair
	}
}

func (miner *Miner) GenerateChallenges(seed int64) {

	challengeIssuedDeal2 := make(map[string]bool)

	miner.ReceivedChPrPairMtx.Lock()
	defer miner.ReceivedChPrPairMtx.Unlock()

	dealhashKeys, expiredDeal := miner.node.GetActiveAndExpiredDealsHash()
	if dealhashKeys == nil {
		return
	}

	sort.Strings(dealhashKeys)
	sort.Strings(expiredDeal)

	NumberOfDealToVerifyL := NumberOfDealToVerify
	if NumberOfDealToVerifyL > len(dealhashKeys) {
		NumberOfDealToVerifyL = len(dealhashKeys)
	}

	randNum := rand.New(rand.NewSource(seed))
	pbc.SetRandRandom(randNum)

	currentTime := time.Now().Add(miner.node.Dpos.TimeLeftToMine(time.Now().Unix()))
	log.Info().Msgf("Miner -- ISSUE CHAllenge active deals size: len %v, cap %v, NumberOfDealToVerifyL %v", len(dealhashKeys), cap(dealhashKeys), NumberOfDealToVerifyL)

	for j := 0; j < NumberOfDealToVerifyL; j++ {
		r := randNum.Intn(len(dealhashKeys))
		log.Info().Msgf("\t Seed %v  Random Number: %v", seed, r)
		dealhash := dealhashKeys[r]
		for {
			if _, ok := challengeIssuedDeal2[dealhash]; !ok {
				break
			}
			r = randNum.Intn(len(dealhashKeys))
			log.Info().Msgf("\t Seed %v  Random Number: %v", seed, r)
			dealhash = dealhashKeys[r]
		}

		challengeIssuedDeal2[dealhash] = true
		if dealhash != "" {
			log.Info().Msgf("Miner %v -- Issued Challenge DealHash %v", miner.node.Port, hex.EncodeToString([]byte(dealhash)))
			deal := miner.node.GetDealbyHash(dealhash)
			if deal != nil {

				miner.generateChPrPair(randNum, dealhash, deal, currentTime)
			} else {
				log.Error().Msgf("\t Deal Not found for Hash %v for deals callenge %v ", hex.EncodeToString([]byte(dealhash)), dealhash)
			}
		} else {
			log.Error().Msgf("GenerateChallenges DealHash empty %v", dealhash)
		}
	}

	for _, dealhash := range expiredDeal {
		r := randNum.Intn(len(dealhashKeys))
		log.Info().Msgf("\t Seed %v  Random Number: %v", seed, r)
		if dealhash != "" {
			deal := miner.node.GetDealbyHash(dealhash)
			if deal != nil {
				if _, ok := challengeIssuedDeal2[dealhash]; !ok {
					log.Info().Msgf("miner %v -- Issued Challenge Expired DealHash %v ", miner.node.Port, hex.EncodeToString([]byte(dealhash)))
					miner.generateChPrPair(randNum, dealhash, deal, currentTime)
				}
			} else {
				log.Error().Msgf("GetDealbyHash deal not found in generateHcallenges: %v %v", hex.EncodeToString([]byte(dealhash)), dealhash)

			}
		} else {
			log.Error().Msgf("GenerateChallenges Expired DealHash empty %v", hex.EncodeToString([]byte(dealhash)))
		}
	}
}

func (miner *Miner) generateChPrPair(randNum *rand.Rand, dealhash string, deal *protobuf.Deal, currentTime time.Time) {

	contentSize := int(0)
	for _, infoPr := range deal.List {
		contentSize += int(infoPr.Info.Metadata.ContentSize)
	}

	if contentSize < 150 {
		contentSize = 150
	}

	blocksTag := pos.GetNumberofBlocksforTag(contentSize, miner.node.PosConfig)
	ch := pos.Challenge(randNum, blocksTag, 3, miner.node.PosPairing)

	var chPrPair *protobuf.ChPrPair
	var ok bool
	if chPrPair, ok = miner.ReceivedChPrPair[dealhash]; !ok {
		log.Error().Msgf("Proof not found for for Deal %v", hex.EncodeToString([]byte(dealhash)))
		chPrPair = &protobuf.ChPrPair{
			Challenge:      ch.ToProto(),
			Proof:          nil,
			Sig:            nil,
			Spid:           deal.Spid,
			Dealhash:       []byte(dealhash),
			DealExpiryTime: deal.ExpiryTime,
			Timestamp:      time.Now().Unix(),
			DealStartTime:  deal.Timestamp,
		}
	}

	ExpiryTime := time.Unix(int64(deal.ExpiryTime), 0) //ExpiryTime := time.Unix(0, int64(deal.ExpiryTime))
	if currentTime.After(ExpiryTime) {
		chPrPair.Timestamp = deal.ExpiryTime
	} else {
		chPrPair.Timestamp = time.Now().Unix()
	}

	miner.ChPrPair = append(miner.ChPrPair, chPrPair)
	delete(miner.ReceivedChPrPair, dealhash)

	pbKey := new(pos.PublicKey)
	pbKey.FromProto(deal.PublicKey, miner.node.PosPairing)
	chl := new(pos.Chall)
	chl.FromProto(chPrPair.Challenge, miner.node.PosPairing)

	valid := false
	valid2 := false
	if chPrPair.Proof != nil {
		prf := new(pos.Proof)
		prf.FromProto(chPrPair.Proof, miner.node.PosPairing)
		valid = pos.Verify([]byte(name), miner.node.PosPairing, pbKey, ch, prf)
		valid2 = pos.Verify([]byte(name), miner.node.PosPairing, pbKey, chl, prf)
		if !(valid2 && valid) {
			log.Error().Msgf("Miner  -- %v **Failed: %v** - Challenge: %v Proof %v \n for Deal %v", miner.node.Port, valid, ch, prf, hex.EncodeToString([]byte(dealhash)))
			log.Error().Msgf("Miner2 -- %v **Failed: %v** - Challenge: %v Proof %v \n for Deal %v", miner.node.Port, valid2, chl, prf, hex.EncodeToString([]byte(dealhash)))
		}
	}

}

func (miner *Miner) ResetCache() {
	for _, key := range miner.stalePrCache.Keys() {
		val1, _ := miner.stalePrCache.Get(key)
		pr := val1.(*protobuf.PostRequest)
		miner.prCache.Add(key, pr)
		miner.stalePrCache.Remove(pr)
	}

	miner.localspReqListCache = make(map[string]map[string]*protobuf.ThreadPostRequestList)
	miner.msgMap = make(map[string][]byte)
	miner.ReceivedChPrPair = make(map[string]*protobuf.ChPrPair)
	miner.transactions.EmptyPool()
}

func (miner *Miner) RemoveExpiredDealDataFromCahce(deal *protobuf.Deal) {

	threadheadPostHash := deal.List[0].Info.Metadata.ThreadHeadPostHash
	if threadheadPostHash == nil {
		threadheadPostHashbytes, _ := proto.Marshal(deal.List[0].Info)
		threadheadPostHash = crypto.Sha256(threadheadPostHashbytes)
	}

	miner.postToPrimarySpCache.Remove(string(threadheadPostHash))
	miner.threadPostingTimeMtx.Lock()
	delete(miner.threadPostingTime, (string(threadheadPostHash)))
	miner.threadPostingTimeMtx.Unlock()

	val, exist := miner.staleSoCache.Get(string(deal.Spid))
	if exist {
		miner.soCache.Add(string(deal.Spid), val)
	}
}
