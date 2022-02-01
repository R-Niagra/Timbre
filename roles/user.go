package roles

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/guyu96/go-timbre/core"
	"github.com/guyu96/go-timbre/crypto"
	"github.com/guyu96/go-timbre/log"
	"github.com/guyu96/go-timbre/net"
	"github.com/guyu96/go-timbre/protobuf"
	pb "github.com/guyu96/go-timbre/protobuf"
	rpcpb "github.com/guyu96/go-timbre/rpc/pb"
	"github.com/guyu96/go-timbre/storage"
)

const (
	userChanSize = 32 // default user channel buffer size
)

// var LocalPostBucket = []byte("local-post-bucket")

// User submits new posts to a Timbre forum.
type User struct {
	node     *net.Node
	incoming chan net.Message

	postCache *storage.LRU
}

// NewUser creates a new User.
func NewUser(node *net.Node) *User {
	user := &User{
		node:     node,
		incoming: make(chan net.Message, userChanSize),
	}
	user.postCache, _ = storage.NewLRU(128)
	node.AddOutgoingChan(user.incoming)
	return user
}

//Node returns the node of the user instance
func (user *User) Node() *net.Node {
	return user.node
}

// Setup creates a map for the posts from the blockchain
func (user *User) Setup() {
	user.node.Db.NewBucket([]byte("user-bucket")) // This would later go to indexer
}

// This is useless now, since the user making requests for the thread(s)
// will be handled by the callback function in request and response and blocks are handled by syncer
//Process for the User where it will be receiving messages
func (user *User) Process() {
	for msg := range user.incoming {
		switch msg.Code {
		case net.MsgCodeInsufficientBalanceToPost:
			user.handlePostNotStored(msg)

		}
	}
}

// makePostRequest returns a post request to be sent to the miners.
func (user *User) makePostRequest(content, parentPostHash, threadheadPostHash []byte, maxCost, minDuration uint32, authorAddress []byte) (*pb.PostRequest, error) {

	var authorpk []byte
	if authorAddress == nil {
		authorpk = user.node.PublicKey().ToByte()
	} else {
		authorpk = authorAddress
	}

	metadata := &pb.PostMetadata{
		Pid:                user.node.PublicKey().ToByte(),
		ContentHash:        crypto.Sha256(content),
		ContentSize:        uint32(len(content)),
		ParentPostHash:     parentPostHash,
		ThreadHeadPostHash: threadheadPostHash,
		AuthorAddress:      authorpk,
	}

	param := &pb.PostParameter{
		MaxCost: maxCost,
	}
	info := &pb.PostInfo{
		Metadata: metadata,
		Param:    param,
	}

	infoBytes, err := proto.Marshal(info)
	if err != nil {
		return nil, err
	}
	infoBytes = append(infoBytes, []byte(core.DefaultConfig.ChainID)...)
	sig := user.node.PrivateKey().SignB(infoBytes)

	timestamp := time.Now().Unix()
	timestampBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBytes, uint64(timestamp))
	timeStampSig := user.node.PrivateKey().SignB(timestampBytes)
	pr := &pb.PostRequest{
		SignedInfo: &pb.SignedPostInfo{
			Info:         info,
			Sig:          sig,
			Timestamp:    timestamp,
			TimeStampSig: timeStampSig,
		},
		Content: content,
	}

	return pr, nil
}

func (user *User) SendPostRequest(content, parentPostHash, threadheadPostHash []byte, maxCost, minDuration uint32, authorAddress []byte) string {

	post, err := user.makePostRequest(content, parentPostHash, threadheadPostHash, maxCost, minDuration, authorAddress)
	if err != nil {
		panic(err)
	}

	postbytes, _ := proto.Marshal(post)
	msg := net.NewMessage(net.MsgCodePostNew, postbytes)
	user.node.Broadcast(msg)

	// TODO: here tmporary just to get last thread post host hash for auto reply post hash
	hashbytes, _ := proto.Marshal(post.SignedInfo.Info)
	hash := crypto.Sha256(hashbytes)
	return hex.EncodeToString(hash)
}

func (user *User) GetModList() {
	// Assuming everyone knows the moderator ids
	var data []byte
	msg := net.NewMessage(net.MsgCodeGetModList, data)
	// This should not be a broadcast
	user.node.RequestAndResponse(msg, user.OnRecModList, net.MsgCodeModPosts)
}

func (user *User) OnRecModList(msg net.Message) error {
	return nil
}

// GetThread gets thread based on threadroothash
// Either ask the storage provider or locally broadcast
// Todo: locally broadcast
// This does not send the thread back. Just asks and caches it.
func (user *User) GetThread(threadHeadHash string) {

	// Find in threadbase
	posts, ok := user.node.ThreadBaseMap.Load(threadHeadHash)
	if !ok {
		log.Info().Msgf("Thread does not exist in local base here")
		return
	}
	// Adding the thread root itself
	thread := posts.([]string)
	thread = append([]string{threadHeadHash}, thread...)

	for _, post := range thread {

		postHashInBytes, _ := hex.DecodeString(post)

		_, ok := user.postCache.Get(post)
		if ok {
			log.Info().Msgf("Post already exists in cache...continuing")
			continue
		}

		// If user is also a storage provider
		if user.node.StorageProvider != nil {
			data, err1 := user.node.Db.Get(storage.PostBucket, postHashInBytes)
			// And post exists in my db
			if err1 == nil {
				post := &protobuf.StoredPost{}
				err := proto.Unmarshal(data, post)
				if err != nil {
					log.Error().Msgf("Can not marshal post")
					return
				}
				log.Error().Msgf("I am sp as well, adding to cache %v", hex.EncodeToString(post.Hash))

				bytesRPost, _ := proto.Marshal(user.ToRPCPost(post))

				user.postCache.Add(hex.EncodeToString(post.Hash), bytesRPost)
				continue
			}
		}

		msg := net.NewMessage(net.MsgCodeGetPost, postHashInBytes)
		user.node.RequestAndResponse(msg, user.onGetPostContent, net.MsgCodePostContent)
	}

}

// GetThreads retrieves all available threads
func (user *User) GetThreads() {

	user.node.ThreadBaseMap.Range(func(key interface{}, val interface{}) bool {
		user.GetThread(key.(string)) // Get Every thread
		return true
	})

}

func (user *User) GetThreadRoots() []string {
	var hashes []string

	for headHash, _ := range user.node.ThreadBase {
		hashes = append(hashes, headHash)
	}
	return hashes
}

func (user *User) GetThreadFromCache(threadHeadHash string) ([]*rpcpb.Post, error) {

	var postsToReturn []*rpcpb.Post
	posts, ok := user.node.ThreadBaseMap.Load(threadHeadHash)

	if !ok {
		log.Info().Msgf("Thread does not exist in local base")
		return nil, errors.New("Thread does not exist in local store")
	}
	posts2 := posts.([]string)
	posts2 = append([]string{threadHeadHash}, posts.([]string)...)
	for _, post := range posts2 {

		p, err := user.GetPostFromCache(post)
		if err != nil {
			log.Info().Msgf("Post not found. Continuing...%v", post)
			continue
		}
		postsToReturn = append(postsToReturn, p)
	}
	return postsToReturn, nil
}

func (user *User) GetPostFromCache(key string) (*rpcpb.Post, error) {

	v, ok := user.postCache.Get(key)
	if !ok {
		log.Info().Msgf("Can not find post from cache")
		return nil, errors.New("Can not find post from cache")
	}

	rpcPost := v.([]byte)

	toReturn := &rpcpb.Post{}
	err := proto.Unmarshal(rpcPost, toReturn)
	if err != nil {
		log.Error().Msgf("Could not unmarshal post from db")
	}
	return toReturn, nil
}

func (user *User) GetTs() [][]byte {

	var posts [][]byte
	for _, v := range user.postCache.Keys() {
		p, ok := user.postCache.Get(v)
		if !ok {
			continue
		}
		posts = append(posts, p.([]byte))
	}
	return posts
}

func (user *User) GetStoredP() []*protobuf.StoredPost {

	var posts []*protobuf.StoredPost
	for _, v := range user.postCache.Keys() {
		p, ok := user.postCache.Get(v)
		if !ok {
			continue
		}
		p2 := &protobuf.StoredPost{}
		err := proto.Unmarshal(p.([]byte), p2)
		if err != nil {
			return nil
		}
		posts = append(posts, p2)
	}
	return posts
}

func (user *User) onGetPostContent(msg net.Message) error {

	// TODO: change it to post info and postParentHash
	post := &protobuf.StoredPost{}
	err := proto.Unmarshal(msg.Data, post)
	if err != nil {
		return err
	}

	bytesRPost, _ := proto.Marshal(user.ToRPCPost(post))

	user.postCache.Add(hex.EncodeToString(post.Hash), bytesRPost)

	return nil
}

func (user *User) handlePostNotStored(msg net.Message) {
	pr := new(protobuf.PostRequest)
	if err := proto.Unmarshal(msg.Data, pr); err != nil {
		log.Info().Msgf("Miner: PostRequest Unmarshal Error:%v", err)
		return
	}

	log.Info().Msgf("POSTER: Post Not stored: %v", pr)
}

func (user *User) UpdateLikes(hash string, votes uint32) {

	// Update Likes on local cache
	log.Error().Msgf(hash, votes)
	hashBytes, e := hex.DecodeString(hash)
	if e != nil {
		return
	}
	postToUpdate := &protobuf.StoredPost{
		Hash:  hashBytes,
		Likes: int64(votes),
	}

	postBytes, err := proto.Marshal(postToUpdate)
	if err != nil {
		return
	}
	// Broadcast for storage providers
	msg := net.NewMessage(net.MsgCodeUpdateLikes, postBytes)
	user.node.Broadcast(msg)

}

func (user *User) ToRPCPost(post *protobuf.StoredPost) *rpcpb.Post {

	rpcPost := &rpcpb.Post{
		Hash:           hex.EncodeToString(post.Hash),
		Content:        string(post.Content),
		ParentHash:     hex.EncodeToString(post.ParentHash),
		RootThreadHash: hex.EncodeToString(post.ThreadHeadHash),
		Timestamp:      post.GetTimestamp(),
		Likes:          uint32(post.GetLikes()),
	}

	return rpcPost
}
