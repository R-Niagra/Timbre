package rpc

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/guyu96/go-timbre/core"
	"github.com/guyu96/go-timbre/log"
	"github.com/guyu96/go-timbre/net"
	"github.com/guyu96/go-timbre/roles"
	rpcpb "github.com/guyu96/go-timbre/rpc/pb"
	"github.com/guyu96/go-timbre/storage"
	"github.com/guyu96/go-timbre/wallet"
	"github.com/mbilal92/noise"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Bucket for user login/reiger
// var (
// 	userBucket []byte // name for node info storage bucket
// )

// func init() {
// 	userBucket = []byte("user-bucket")
// }

type Api struct {
	node *net.Node
}

func NewAPI(node *net.Node) *Api {
	return &Api{
		node: node,
	}
}

// TODO: Error handling for each

func (s *Api) GetBlock(ctx context.Context, req *rpcpb.GetBlockRequest) (*rpcpb.Block, error) {

	var block *core.Block
	var err error

	if s.node.Bc.IsEmpty() {
		return nil, status.Error(codes.NotFound, "Not Found")
	}

	if req.Hash != "" {
		by, err := hex.DecodeString(req.Hash)
		block, err = s.node.Bc.GetBlockByHash(by)
		if err != nil || block == nil {
			return nil, status.Error(codes.InvalidArgument, "Incorrect Argument")
		}
	}

	if req.Height == 0 {
		block = s.node.Bc.Genesis()
	}

	if req.Height != 0 {
		block, err = s.node.Bc.GetBlockByHeight(req.Height)
		if err != nil || block == nil {
			return nil, status.Error(codes.InvalidArgument, "Incorrect Argument")
		}
	}

	if block == nil {
		return nil, status.Error(codes.InvalidArgument, "Not Found")
	}

	return &rpcpb.Block{
		Height:         block.Height(),
		Hash:           hex.EncodeToString(block.Hash()),
		ParentHash:     hex.EncodeToString(block.ParentHash()),
		Timestamp:      block.GetTimestamp(),
		MinerPublicKey: hex.EncodeToString(block.PbBlock.Header.GetMinerPublicKey()),
		RoundNum:       block.GetRoundNum(),
	}, nil

}

// func (s *Api) GetNBlocks(ctx context.Context, req *rpcpb.GetNBlocksRequest) (*rpcpb.Blocks, error) {
// 	if s.node.Bc == nil {
// 		return nil, status.Error(codes.Internal, "Blockchain is empty")
// 	}
// 	blocks := s.node.Bc.GetLastNBlocks(5)

// 	return &rpcpb.Blocks{
// 		Blocks: blocks,
// 	}, nil
// }

// asd
func (s *Api) GetState(ctx context.Context, req *rpcpb.NonParamRequest) (*rpcpb.State, error) {

	if s.node.Bc.IsEmpty() {
		return nil, nil
	}

	return &rpcpb.State{
		Tail:   s.node.Bc.GetMainTail().String(),
		Height: int64(s.node.Bc.Length()),
		Forks:  int64(s.node.Bc.NumForks()),
	}, nil
}

func (s *Api) GetAccount(ctx context.Context, req *rpcpb.NonParamRequest) (*rpcpb.Account, error) {

	return &rpcpb.Account{ //TODO:ChangeToPK -> review
		Address:  s.node.PublicKey().String(),
		Balance:  int64(s.node.Wm.MyWallet.GetBalance()),
		NodeType: s.node.GetNodeType(),
	}, nil
}

func (s *Api) ListBalances(ctx context.Context, in *rpcpb.NonParamRequest) (*rpcpb.AllAccounts, error) {
	accounts := s.node.Wm.GetAccounts()
	allAccounts := new(rpcpb.AllAccounts)

	for _, acc := range accounts {
		allAccounts.Accounts = append(allAccounts.Accounts, &rpcpb.Account{ //
			Address:  acc.Address,
			Balance:  acc.Balance,
			NodeType: "Unknown",
		})
	}

	return allAccounts, nil
}

//ListVotes returns all accounts with the votes
func (s *Api) ListVotes(ctx context.Context, in *rpcpb.NonParamRequest) (*rpcpb.AllAccountVotes, error) {

	accounts := s.node.Wm.GetAccountsWithVotes()
	allAccounts := new(rpcpb.AllAccountVotes)

	for _, a := range accounts {
		var votes []*rpcpb.PercentVote
		for _, v := range a.Votes {
			votes = append(votes, &rpcpb.PercentVote{
				Address: v.Address,
				Percent: int32(v.Percent),
			})
		}
		accountEntry := &rpcpb.AccountVotes{
			Acc: &rpcpb.Account{
				Address: a.Address,
				Balance: a.Balance,
			},
			Votes: votes,
		}
		allAccounts.AccountsWithVotes = append(allAccounts.AccountsWithVotes, accountEntry)
	}
	return allAccounts, nil
}

//GetCurrentRound fetches the current roundNum of the node
func (s *Api) GetCurrentRound(ctx context.Context, in *rpcpb.NonParamRequest) (*rpcpb.CurrentRound, error) {

	curRound := s.node.Dpos.GetRoundNum()
	return &rpcpb.CurrentRound{
		RoundNum: curRound,
	}, nil
}

//GetCurrentMiners returns current miner addresses
func (s *Api) GetCurrentMiners(ctx context.Context, in *rpcpb.NonParamRequest) (*rpcpb.Miners, error) {

	curMiners := s.node.Dpos.GetMinersAddresses()
	return &rpcpb.Miners{
		Miners: curMiners,
	}, nil
}

//GetMinersByRound returns miners by round number
func (s *Api) GetMinersByRound(ctx context.Context, in *rpcpb.NonParamRequest) (*rpcpb.MinersByRound, error) {

	minersByRound, err := s.node.Dpos.GetMinersByRound()
	if err != nil {
		return nil, err
	}
	// res = new(rpcpb.MinersByRound)
	var allMiners []*rpcpb.MinerByRound
	for _, m := range minersByRound {
		var miners *rpcpb.Miners
		miners.Miners = append(miners.Miners, m.Miners...)
		allMiners = append(allMiners, &rpcpb.MinerByRound{
			Round:  m.Round,
			Miners: miners,
		})
	}

	return &rpcpb.MinersByRound{
		AllRoundMiners: allMiners,
	}, nil
}

//StartUser is a command to start user(should be authenticated somehow)
func (s *Api) StartUser(context.Context, *rpcpb.NonParamRequest) (*rpcpb.Empty, error) {
	err := roles.CreateAndStartUser(s.node)
	return &rpcpb.Empty{}, err
}

//StartMiner is a command to start miner instance- returns error if it is already running(should be authenticated somehow)
func (s *Api) StartMiner(context.Context, *rpcpb.NonParamRequest) (*rpcpb.Empty, error) {
	err := roles.CreateAndStartMiner(s.node)
	return &rpcpb.Empty{}, err
}

//StartStorageProvider is a command to start storage-provider instance- returns error if it is already running(should be authenticated somehow)
func (s *Api) StartStorageProvider(context.Context, *rpcpb.NonParamRequest) (*rpcpb.Empty, error) {
	err := roles.CreateAndStartMiner(s.node)
	return &rpcpb.Empty{}, err
}

func (s *Api) SendStorageOffer(ctx context.Context, req *rpcpb.SendStorageOfferRequest) (*rpcpb.SendStorageOfferResponse, error) {

	if s.node.StorageProvider == nil {
		return nil, status.Error(codes.Internal, "Node is not a valid storage provider")
	}

	s.node.StorageProvider.SendStorageOffer(req.MinPrice, req.MaxDuration, req.Size)

	return &rpcpb.SendStorageOfferResponse{
		Success: "1",
	}, nil
}

// Todo: Check if not miner
func (s *Api) GetCommittee(ctx context.Context, req *rpcpb.NonParamRequest) (*rpcpb.Candidates, error) {

	miners := s.node.Dpos.GetMinersAddresses()
	log.Info().Msgf("Asked for candidates......")
	return &rpcpb.Candidates{
		Candidates: miners,
	}, nil
}

func (s *Api) GetThread(ctx context.Context, req *rpcpb.GetThreadRequest) (*rpcpb.Thread, error) {

	go s.node.User.GetThread(req.Threadroothash) // Ask for the thread on the network
	thread, err := s.node.User.GetThreadFromCache(req.Threadroothash)
	if err != nil {
		log.Error().Msgf("Thread not found %v", req.Threadroothash)
		return nil, errors.New("Thread could not be found")
	}

	return &rpcpb.Thread{Posts: thread}, nil
}

func (s *Api) GetThreads(ctx context.Context, req *rpcpb.NonParamRequest) (*rpcpb.Threads, error) {

	go s.node.User.GetThreads() // Asks for threads on the network
	var threads []*rpcpb.Thread

	count := 0
	s.node.ThreadBaseMap.Range(func(key interface{}, val interface{}) bool {
		count++

		thread, err := s.node.User.GetThreadFromCache(key.(string))

		if err != nil {
			log.Info().Msgf("Thread could not be found. Moving to next thread")
			return true
		}

		if thread == nil {
			return true
		}

		threads = append(threads, &rpcpb.Thread{Posts: thread})

		return true
	})

	log.Error().Msgf("Number of threads %v", count)

	if threads == nil {
		return nil, status.Error(codes.NotFound, "Theads not found")
	}

	return &rpcpb.Threads{
		Threads: threads,
	}, nil

}

func (s *Api) SendPost(ctx context.Context, req *rpcpb.SendPostRequest) (*rpcpb.PostHash, error) {
	phash, _ := hex.DecodeString(req.ParentPostHash)
	threadheadPosthashh, _ := hex.DecodeString(req.ThreadheadPostHash)
	h := s.node.User.SendPostRequest([]byte(req.Content), phash, threadheadPosthashh, req.MaxCost, req.MinDuration, nil)
	return &rpcpb.PostHash{
		Hash: h,
	}, nil

}

func (s *Api) ChangeRole(ctx context.Context, req *rpcpb.ChangeRoleRequest) (*rpcpb.ChangeRoleResponse, error) {

	if req.Role == "SP" {
		s.node.SetAndStartStorageProvider(roles.NewStorageProvider(s.node))
	} else if req.Role == "MINER" {
		s.node.SetAndStartMiner(roles.NewMiner(s.node))
	}
	return &rpcpb.ChangeRoleResponse{
		Success: 1,
	}, nil
}

func (s *Api) TransferMoney(ctx context.Context, req *rpcpb.TransferMoneyRequest) (*rpcpb.MoneyTransferResponse, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Info().Msgf("TransferMoney: Just want to check: %s %d", req.Address, req.Amount)

	err := s.node.Services.DoAmountTrans(ctx, req.Address, req.Amount)
	if err != nil {
		log.Info().Err(err)
	}
	return &rpcpb.MoneyTransferResponse{
		Success: 1,
	}, nil
}

//DoVote votes for the candidate
func (s *Api) DoVote(ctx context.Context, req *rpcpb.VoteRequest) (*rpcpb.TxResponse, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Info().Msgf("Creating vote tx: %s %d", req.Address, req.Percentage)

	err := s.node.Services.DoPercentVote(ctx, req.Address, req.Percentage)
	if err != nil {
		log.Info().Err(err)
		return &rpcpb.TxResponse{
			Success: false,
		}, err
	}
	return &rpcpb.TxResponse{
		Success: true,
	}, nil
}

//DoUnVote un0votes for the already voted candidate
func (s *Api) DoUnVote(ctx context.Context, req *rpcpb.VoteRequest) (*rpcpb.TxResponse, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Info().Msgf("Creating vote tx: %s %d", req.Address, req.Percentage)

	err := s.node.Services.DoPercentUnVote(ctx, req.Address, req.Percentage)
	if err != nil {
		log.Info().Err(err)
		return &rpcpb.TxResponse{
			Success: false,
		}, err
	}
	return &rpcpb.TxResponse{
		Success: true,
	}, nil
}

//RegisterDelegate registers the delegate as a miner candidate
func (s *Api) RegisterDelegate(ctx context.Context, req *rpcpb.NonParamRequest) (*rpcpb.TxResponse, error) {

	log.Info().Msgf("Creating register delegate tx")

	_, err := s.node.Services.DoDelegateRegisteration()
	if err != nil {
		log.Info().Err(err)
		return &rpcpb.TxResponse{
			Success: false,
		}, err
	}
	return &rpcpb.TxResponse{
		Success: true,
	}, nil

}

//UnRegisterDelegate registers the delegate as a miner candidate
func (s *Api) UnRegisterDelegate(ctx context.Context, req *rpcpb.NonParamRequest) (*rpcpb.TxResponse, error) {

	log.Info().Msgf("Creating register delegate tx")

	err := s.node.Services.DoDelegateQuit()
	if err != nil {
		log.Info().Err(err)
		return &rpcpb.TxResponse{
			Success: false,
		}, err
	}
	return &rpcpb.TxResponse{
		Success: true,
	}, nil

}

func (s *Api) GetStorageProviderStats(ctx context.Context, req *rpcpb.NonParamRequest) (*rpcpb.StorageProviderStats, error) {

	if s.node.StorageProvider == nil {
		return nil, status.Error(codes.Internal, "Node is not a valid storage provider")
	}

	stats := s.node.StorageProvider.GetStorageProviderStats()
	return stats, nil

}

// These functions would only called via the indexer
func (s *Api) Register(ctx context.Context, req *rpcpb.RegisterRequest) (*rpcpb.RegisterResponse, error) {

	if !s.node.Db.HasBucket(storage.UserBucket) {
		return nil, status.Error(codes.Internal, "User Bucket not found")
	}

	val, _ := s.node.Db.Get(storage.UserBucket, []byte(req.Username))

	if val == nil {

		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 2)
		if err != nil {
			return nil, status.Error(codes.Internal, "Could not hash the password")
		}

		err = s.node.Db.Put(storage.UserBucket, []byte(req.Username), hash)
		if err != nil {
			return nil, status.Error(codes.Internal, "Could not store user data")
		}

		// Generate keypair and save
		var keys net.Keypair
		keys.PublicKey, keys.PrivateKey, err = noise.GenerateKeys(nil)
		// keypair := kad.RandomKeys()
		// kad.PersistKeypairs("../Keys/Keys"+fmt.Sprintf("%s", req.Username)+".txt", []*kad.Keypair{keypair})
		noise.PersistKey("../Keys/Keys"+fmt.Sprintf("%s", req.Username)+".txt", keys.PrivateKey)

		wallet := wallet.NewWallet(req.Username)
		s.node.Wm.CheckAndPutWalletByAddress(keys.PublicKey.String(), wallet)

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"username": req.Username,
		})

		tokenString, err := token.SignedString([]byte("alizohaib")) //

		if err != nil {
			return nil, status.Error(codes.Internal, "Could not generate token")
		}

		return &rpcpb.RegisterResponse{
			Result: "Registration Successful",
			Token:  tokenString,
		}, nil
	}

	return nil, status.Error(codes.AlreadyExists, "User already exists")
}

type Claims struct {
	Username string `json:"username"`
	jwt.StandardClaims
}

func (s *Api) Login(ctx context.Context, req *rpcpb.LoginRequest) (*rpcpb.LoginResponse, error) {

	if !s.node.Db.HasBucket(storage.UserBucket) {
		return nil, status.Error(codes.Internal, "User Bucket not found")
	}

	hash, err := s.node.Db.Get(storage.UserBucket, []byte(req.Username))
	if hash == nil {
		return nil, status.Error(codes.Unauthenticated, "Username doesn't exist")
	}

	err = bcrypt.CompareHashAndPassword(hash, []byte(req.Password))

	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "Incorrect password")
	}

	expirationTime := time.Now().Add(5 * time.Minute)

	claims := &Claims{
		Username: req.Username,
		StandardClaims: jwt.StandardClaims{
			// In JWT, the expiry time is expressed as unix milliseconds
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString([]byte("alizohaib"))

	if err != nil {
		return nil, err
	}

	return &rpcpb.LoginResponse{
		Token: tokenString,
	}, nil
}

func (s *Api) GetAccountViaUsername(ctx context.Context, req *rpcpb.GetAccountViaUsernameReq) (*rpcpb.Account, error) {

	path := "../Keys/Keys" + fmt.Sprintf("%s", req.Username) + ".txt"
	// Check if keypair file exists
	_, err := os.Stat(path)

	if err != nil {
		return nil, status.Error(codes.NotFound, "Keys File does not exist")
	}

	// Find public key
	// keysFromFile := kad.LoadKeypairs(path)
	// keys := keysFromFile[0]
	var keys net.Keypair
	keys.PrivateKey = noise.LoadKey(path)
	keys.PublicKey = keys.PrivateKey.Public()
	pkString := keys.PublicKey.String()

	// Find address
	w, err := s.node.Wm.GetWalletByAddress(pkString)
	if err != nil {
		return nil, status.Error(codes.NotFound, "Wallet not found for username")
	}

	return &rpcpb.Account{
		Address:  pkString,
		Balance:  int64(w.GetBalance()),
		NodeType: "Subscriber",
	}, nil
}

func (s *Api) SendPostViaIndexer(ctx context.Context, req *rpcpb.SendPostRequestViaIndexer) (*rpcpb.PostHash, error) {

	// GetPublicKeyVia username
	// keysFromFile := kad.LoadKeypairs("../Keys/Keys" + fmt.Sprintf("%s", req.Username) + ".txt")
	// keys := keysFromFile[0]
	var keys net.Keypair
	keys.PrivateKey = noise.LoadKey("../Keys/Keys" + fmt.Sprintf("%s", req.Username) + ".txt")
	keys.PublicKey = keys.PrivateKey.Public()
	pk := keys.PublicKey[:] //keys.PublicKey()

	phash, _ := hex.DecodeString(req.ParentPostHash)
	threadheadPosthashh, _ := hex.DecodeString(req.ThreadheadPostHash)
	h := s.node.User.SendPostRequest([]byte(req.Content), phash, threadheadPosthashh, req.MaxCost, req.MinDuration, pk)
	return &rpcpb.PostHash{
		Hash: h,
	}, nil

}

func (s *Api) SendVoteViaIndexer(ctx context.Context, req *rpcpb.SendVoteRequestViaIndexer) (*rpcpb.Votes, error) {

	err := s.node.Services.DoPercentVoteDuplicate(ctx, req.PublicKey, req.Percent)

	if err != nil {
		return nil, status.Error(codes.NotFound, "Could not vote")
	}
	return &rpcpb.Votes{
		PublicKey: req.PublicKey,
	}, nil
}

func (s *Api) LikePost(ctx context.Context, req *rpcpb.LikeAPostRequest) (*rpcpb.LikeAPostResponse, error) {
	s.node.User.UpdateLikes(req.Hash, req.Votes)

	return &rpcpb.LikeAPostResponse{
		Hash: req.Hash,
	}, nil
}
