package wallet

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/guyu96/go-timbre/core"
	"github.com/guyu96/go-timbre/crypto"
	"github.com/guyu96/go-timbre/crypto/pos"
	"github.com/guyu96/go-timbre/log"
	pb "github.com/guyu96/go-timbre/protobuf"
	// "github.com/mbilal92/pbc"
)

const (
	name                            = "Test"
	blockThresholdToBan             = 3
	banDuration                     = 100    //No of blocks for which the block is banned
	GenesisRounds                   = 1      //No of rounds untill free transactions will be entertained
	RewardIncrementDuration int64   = 100000 //Height threshold
	RewardIncrementPercent  float64 = 1      //1 percent

)

//WalletManager keeps track of wallet state of users
type WalletManager struct {
	walletByAddress map[string]*Wallet //Only updating state of this for now
	walletCache     map[string]*Wallet //cache to validate the posts(validation could be dependent on one another)
	MyWallet        *Wallet
	// Bc                    blockChain
	node                   Node
	Executor               TxExecutor
	mutex                  *sync.Mutex
	LastChVerificationTime map[string][]int64
}

//NewWalletManager returns the new WalletManager object
func NewWalletManager(userAddress string) *WalletManager {
	wm := &WalletManager{
		walletByAddress:        make(map[string]*Wallet),
		walletCache:            make(map[string]*Wallet), //cache to validate the posts(validation could be dependent on one another)
		LastChVerificationTime: make(map[string][]int64),

		MyWallet: NewWallet(userAddress),
		mutex:    &sync.Mutex{},
	}
	wm.walletByAddress[userAddress] = wm.MyWallet //Populate the walletByAddress with current node's wallet
	return wm
}

//MyBalance returns the balance associated with the node's wallet(MyWallet)
func (wm *WalletManager) MyBalance() int64 {
	return wm.MyWallet.balance
}

//InjectWalletManager injects the new wm in the existing object
func (wm *WalletManager) InjectWalletManager(newWm *WalletManager) error {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	newWm.mutex.Lock()
	defer newWm.mutex.Unlock()

	myAdd := wm.MyWallet.address
	wm.walletByAddress = make(map[string]*Wallet)
	for k, v := range newWm.walletByAddress {
		if k == myAdd {
			wm.MyWallet = v.DeepCopy()
			wm.walletByAddress[k] = wm.MyWallet
			continue
		}
		wm.walletByAddress[k] = v.DeepCopy()
	}

	log.Info().Msgf("Changes injected successfully")
	//TODO:- Add latest challenge verification time

	return nil
}

//SnapShotWalletsUsingPb takes the snapshot of the wallets
func (wm *WalletManager) SnapShotWalletsUsingPb() *PbWalletsSnapShot {

	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	var snapShot = new(PbWalletsSnapShot)
	snapShot.PbWallets.Wallets = []*pb.Wallet{}

	for _, w := range wm.walletByAddress {
		snapShot.PbWallets.Wallets = append(snapShot.PbWallets.Wallets, w.DeepCopyToPbWallet())
		fmt.Println("Appended wallet: ", w.address, " size: ", snapShot.PbWallets.XXX_Size())
	}

	for k, v := range wm.LastChVerificationTime {
		entry := &pb.ProofEntry{
			DealHash:   hex.EncodeToString([]byte(k)), //dealhahs,
			TimeStamps: v,
		}
		snapShot.PbWallets.ProofTimes = append(snapShot.PbWallets.ProofTimes, entry)
	}

	return snapShot
}

//NewWalletManagerFromPb returns the new WalletManager object
func NewWalletManagerFromPb(wallets *pb.WalletStates, userAddress string) *WalletManager {
	wm := &WalletManager{
		walletByAddress:        make(map[string]*Wallet),
		LastChVerificationTime: make(map[string][]int64),
		mutex:                  &sync.Mutex{},
	}

	for _, wal := range wallets.Wallets {
		w := FormProtoWallet(wal)

		if w.address == userAddress {
			wm.MyWallet = w
			wm.walletByAddress[w.address] = wm.MyWallet
		} else {
			wm.walletByAddress[w.address] = w
		}
	}

	if wm.MyWallet == nil { //If my wallet is still not set then setting it here coz my wallet may have been inavtive
		wm.MyWallet = NewWallet(userAddress)
		wm.walletByAddress[userAddress] = wm.MyWallet
	}

	for _, entry := range wallets.ProofTimes {
		dealhash, _ := hex.DecodeString(entry.DealHash)
		wm.LastChVerificationTime[string(dealhash)] = entry.TimeStamps
	}

	return wm
}

//NewWmFromDigestBlock creates the instance of the walletManager from the epoch/digest block
func NewWmFromDigestBlock(digest *core.Block, userAddress string) (*WalletManager, error) {
	wm := &WalletManager{
		walletByAddress:        make(map[string]*Wallet),
		LastChVerificationTime: make(map[string][]int64),
		walletCache:            make(map[string]*Wallet),
		mutex:                  &sync.Mutex{},
	}

	walletStates := digest.PbBlock.GetState().GetAccountStates()
	for _, entry := range walletStates {
		w, err := NewWalletFromDigestEntry(entry)
		if err != nil {
			return nil, err
		}

		if w.address == userAddress {
			wm.MyWallet = w
			wm.walletByAddress[w.address] = wm.MyWallet
		} else {
			wm.walletByAddress[w.address] = w
		}
	}
	if wm.MyWallet == nil { //If my wallet is still not set then setting it here coz my wallet may have been inavtive
		wm.MyWallet = NewWallet(userAddress)
		wm.walletByAddress[userAddress] = wm.MyWallet
	}

	err := wm.UpdateVotePowerOfAll()
	if err != nil {
		return nil, err
	}

	return wm, nil
}

//UpdateVotePowerOfAll updates the votepower of every wallet in walletManager after wallet from digest blocks are created
func (wm *WalletManager) UpdateVotePowerOfAll() error {

	for _, w := range wm.walletByAddress {
		for cand, percent := range w.Voted {
			if cand, ok := wm.walletByAddress[cand]; ok {
				toAdd := uint64((float64(percent) / float64(100)) * float64(w.GetBalance()))
				cand.AddVotePower(toAdd)
			} else {
				return errors.New("Voted cand doesn't exists")
			}
		}
	}
	return nil
}

//TotalWallets returns the total wallet by address
func (wm *WalletManager) TotalWallets() int {
	return len(wm.walletByAddress)
}

//GetWalletByAddress returns the wallet instance by key
func (wm *WalletManager) GetWalletByAddress(sampleAddress string) (*Wallet, error) {

	if wallet, ok := wm.walletByAddress[sampleAddress]; ok {
		return wallet, nil
	}
	str := sampleAddress + " wallet doesn't exist in the record,Can't fetch"
	return nil, errors.New(str)
}

//SetTxExecutor Sets the tx executor
func (wm *WalletManager) SetTxExecutor(executor TxExecutor) {
	wm.Executor = executor
}

//SetNode Sets the node
func (wm *WalletManager) SetNode(node Node) {
	wm.node = node
}

//SetCandidacy sets the candidacy of the wallet
func (wm *WalletManager) SetCandidacy(pk string) error {
	w, err := wm.GetWalletByAddress(pk)
	if err != nil {
		return err
	}
	w.SetCandidate(true)
	return nil
}

//UnSetCandidacy sets the candidacy of the wallet
func (wm *WalletManager) UnSetCandidacy(pk string) error {
	w, err := wm.GetWalletByAddress(pk)
	if err != nil {
		return err
	}
	w.SetCandidate(false)
	return nil
}

//CheckAndPutWalletByAddress inserts the wallet entry by Address
func (wm *WalletManager) CheckAndPutWalletByAddress(sampleAddress string, sampleWallet *Wallet) error {

	if _, ok := wm.walletByAddress[sampleAddress]; ok {
		return errors.New("wallet entry already present")
	}
	wm.walletByAddress[sampleAddress] = sampleWallet

	return nil
}

//CheckEntryByAddress returns true if entry is already present
func (wm *WalletManager) CheckEntryByAddress(sampleAddress string) bool {

	if _, ok := wm.walletByAddress[sampleAddress]; ok {
		return true
	}

	return false
}

//RevertBatchBlocks reverts the blocks in batch
func (wm *WalletManager) RevertBatchBlocks(blocks []*core.Block) error {

	for _, block := range blocks {
		err := wm.RevertWalletStates(block, block.MinerPublicKey().String())
		if err != nil {
			return err //TODO: Undo the changes in case of error
		}

	}
	return nil
}

//ApplyBatchBlocks applies the block in batches
func (wm *WalletManager) ApplyBatchBlocks(blocks []*core.Block) error {
	for _, block := range blocks {
		err := wm.CheckAndUpdateWalletStates(block, block.MinerPublicKey().String())
		if err != nil {
			return err
		}

	}
	return nil
}

//RevertWalletStates reverts the state of all the wallets in WalletByAddress
func (wm *WalletManager) RevertWalletStates(block *core.Block, minerAdd string) error {

	log.Info().Msgf("RevertWalletStates Reverting block...")
	mWallet, err := wm.GetWalletByAddress(minerAdd) //for miner
	if err != nil {
		return err
	}

	if block.IsDigestBlock() {
		wm.UndoDigestReset(core.TestEpoch.RollBackPercentage)
		wm.RevertBlockReward(block, mWallet)

		return nil
	}
	wm.RevertBlockReward(block, mWallet)

	transactions := block.GetTransactions()
	err = wm.RevertTransactions(transactions, mWallet, block.GetRoundNum())
	if err != nil {
		return err //Should never hit this
	}

	deals := block.GetDeals()
	err = wm.RevertDeals(deals, mWallet, block.GetRoundNum())
	if err != nil {
		return err //Should never hit this
	}

	chPrPair := block.GetChPrPair()
	err = wm.RevertChPrPair(chPrPair) //Handle the verifications
	if err != nil {
		return err //Should never hit this as well
	}

	return nil
}

//ValidateBlockTx validate if the transaction can be applied
func (wm *WalletManager) ValidateBlockTx(block *core.Block) (bool, error) {

	transactions := block.GetTransactions()
	for _, tx := range transactions {
		valid, err := wm.IsExecutable(tx, block.GetRoundNum()) //validating retrieval,becomeCand,QuitCandidate transaction
		if valid == false || err != nil {
			log.Info().Msgf("Err is: %v", err.Error())
			return false, err
		}
	}

	deals := block.GetDeals() //Validate the deals in the block
	for _, deal := range deals {
		err := wm.ValidateDeal(deal, block.GetRoundNum())
		if err != nil {
			return false, err
		}
	}

	return true, nil
}

//ValidatePodf validates the podf
func (wm *WalletManager) ValidatePodf(podf *pb.Transaction, bRound int64) error {

	_, err := core.HasValidSign(podf)
	if err != nil {
		return err
	}

	h1, h2, err := core.PodfPayloadFromByte(podf.Body.Payload)
	if err != nil {
		return err
	}

	if h1.Height != h2.Height { //Heights should be same
		return errors.New("Heights doesn't match in podf")
	}
	if bytes.Equal(h1.Hash, h2.Hash) == true { //Headers hash should be different
		return errors.New("Both headers have the same hash")
	}
	err = core.ValidateHeaderSign(h1) //Validating signatures of header 1
	if err != nil {
		return err
	}
	err = core.ValidateHeaderSign(h2) //Validating signatures of header 2
	if err != nil {
		return err
	}

	//Checking if reported miner is already a candidate
	minerAdd := hex.EncodeToString(h1.GetMinerPublicKey())
	mWallet, err := wm.GetWalletByAddress(minerAdd)
	if err != nil {
		return err
	}

	if mWallet.isCandidate == false {
		return errors.New("Reported miner is not a candidate")
	}

	// Checking if the tx-Maker has the balance to pay podf tx
	txFee := core.GetTxFee(podf)
	if bRound <= GenesisRounds {
		txFee = 0 //Free transactions for the genesis round
	}
	makerPk := podf.Body.GetSenderId()
	txMaker, err := wm.GetWalletByAddress(makerPk)
	if err != nil {
		wm.CheckAndPutWalletByAddress(makerPk, NewWallet(makerPk))
		txMaker, err = wm.GetWalletByAddress(makerPk)
	}

	pNonce := podf.Body.Nonce
	if wm.CheckNonce(pNonce, txMaker) != true { //1check:- Nonce should be in order //Shouldn't this be pwallet + 1?
		log.Info().Msgf("Out of order.Nonce in vote")
		return errors.New("out of order nonce")
	}

	if txMaker.balance < txFee {
		return errors.New("Unable to pay txFee")
	}

	//Checking if the reported miner is already banned till the reported height
	if mWallet.bannedTill >= h1.GetHeight() {
		return errors.New("Reported miner is already banned")
	}

	//check if the user has already been punished at the given height
	for i := len(mWallet.podfHeights) - 1; i >= 0; i-- {
		if mWallet.podfHeights[i] == h1.Height {
			return errors.New("Misbehaving miner is already reported at the given height")
		} else if mWallet.podfHeights[i] < (h1.Height - 21) {
			break //No need to look further
		}
	}
	fmt.Println("PODF has been validated")
	return nil
}

//ValidateVote validates the vote transaction
func (wm *WalletManager) ValidateVote(vote *pb.Transaction, bRound int64) (bool, error) {
	voterID := vote.Body.GetSenderId()
	txFee := core.GetTxFee(vote)
	if bRound <= GenesisRounds {
		txFee = 0 //No transaction fee for the genesis rounds
	}
	valid, err := core.HasValidSign(vote)
	if err != nil {
		fmt.Println("err is: ", err.Error())
		return valid, err
	}

	vWallet, err := wm.GetWalletByAddress(voterID) //Fetching the voter wallet
	if err != nil {
		wm.CheckAndPutWalletByAddress(voterID, NewWallet(voterID))
		vWallet, _ = wm.GetWalletByAddress(voterID) // creating new wallet with new vote
	}

	vNonce := vote.Body.Nonce
	if wm.CheckNonce(vNonce, vWallet) != true { //1check:- Nonce should be in order //Shouldn't this be pwallet + 1?
		log.Info().Msgf("Out of order.Nonce in vote")
		return false, errors.New("out of order nonce")
	}

	if vWallet.balance < txFee {
		return false, errors.New("Insufficient balance to pay vote fee")
	}

	votedCands, err := core.VotePayloadFromByte(vote.Body.Payload)
	if err != nil {
		return false, err
	}

	for _, votedCand := range votedCands {
		action, percent, candID, err := ParseVote(votedCand)
		if err != nil {
			return false, err
		}
		if action == "-" {
			err = vWallet.CheckUnvoteValidity(percent, candID)
			if err != nil {
				return false, err
			}
		} else {
			err = vWallet.CheckVoteValidity(percent, candID)
			if err != nil {
				return false, err
			}
		}
	}
	return true, nil
}

//ApplyGenesis update the wallet with Genesis block- For now it only entertains delegate reg transaction however other can be added easily
func (wm *WalletManager) ApplyGenesis(b *core.Block) error {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	fmt.Println("Applying Genesis...")
	minerAdd := b.MinerPublicKey().String()
	wm.CheckAndPutWalletByAddress(minerAdd, NewWallet(minerAdd)) //Putting miner in map if not present
	wm.SetCandidacy(minerAdd)
	mWallet, err := wm.GetWalletByAddress(minerAdd) //for miner
	if err != nil {
		return err
	}

	transactions := b.GetTransactions()
	for _, tx := range transactions {
		if tx.GetType() == core.TxBecomeCandidate {
			wm.ApplyDelRegTxForGenesis(tx, 0, b.GetRoundNum())
		}
	}
	// wm.GenesisDelegateReg(b)

	wm.GiveBlockReward(b, mWallet)

	return nil
}

//CheckAndUpdateWalletStates updates the state of all the wallets in WalletByAddress
func (wm *WalletManager) CheckAndUpdateWalletStates(block *core.Block, minerAdd string) error {
	if block.IsGenesis() {
		return wm.ApplyGenesis(block)
	}

	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	log.Info().Msgf("Checking Updating wallet states")

	wm.CheckAndPutWalletByAddress(minerAdd, NewWallet(minerAdd)) //Putting miner in map if not present
	// wm.SetCandidacy(minerAdd)
	mWallet, err := wm.GetWalletByAddress(minerAdd) //for miner
	if err != nil {
		return err
	}

	if block.IsDigestBlock() {
		err := wm.ValidateDigestBlock(block)
		if err != nil {
			log.Info().Msgf(err.Error())
			return err
		}
		log.Info().Msgf("Wallet state has been resetted")
		wm.GiveBlockReward(block, mWallet)
		wm.ResetWalletManagerState(core.TestEpoch.RollBackPercentage)
		return nil
	}

	wm.GiveBlockReward(block, mWallet)

	transactions := block.GetTransactions()
	appliedTxs, err := wm.ApplyTransactions(transactions, mWallet, block.GetRoundNum())
	if err != nil {
		log.Info().Msgf("Faulty tx found. Reverting block... %v", err)
		err1 := wm.InitiateRevertBlock(appliedTxs, 0, 0, 0, 0, block)
		if err1 != nil {
			log.Error().Msgf(err.Error())
		}
		return err
	}

	deals := block.GetDeals()
	appliedDeals, err := wm.applyDeals(deals, mWallet, block.GetRoundNum())
	if err != nil {
		log.Info().Msgf("Faulty deal found. Reverting block... %v", err)
		err1 := wm.InitiateRevertBlock(len(transactions), appliedDeals, 0, 0, 0, block)
		if err1 != nil {
			log.Error().Msgf(err.Error())
		}
		return err
	}

	chPrPair := block.GetChPrPair()                  // no error in case if it fails
	appliedProofs, err := wm.ApplyChPrPair(chPrPair) //Handle the verifications

	if err != nil {
		fmt.Println("Faulty proof found. Reverting block...", err)
		err1 := wm.InitiateRevertBlock(len(transactions), len(deals), appliedProofs, 0, 0, block)
		if err1 != nil {
			log.Error().Msgf(err.Error())
		}
		return err
	}

	if minerAdd == wm.MyWallet.address {
		// fmt.Println("Reward reverted for block Creation")
	}

	return nil
}

//GiveBlockReward rewards miner for the block creation
func (wm *WalletManager) GiveBlockReward(block *core.Block, miner *Wallet) {
	if block.IsEmpty() { //Giving reward to the miner for the block creation
		emptyBlockReward := int64(float64(wm.RewardForEmptyBlock()) * math.Pow(1+RewardIncrementPercent/100, float64((block.Height())/RewardIncrementDuration)))
		miner.balance += emptyBlockReward
		miner.blockReward += emptyBlockReward
		wm.ReAssignVotePower(miner, int64(emptyBlockReward))

	} else {
		filledBlockReward := int64(float64(wm.RewardForBlockCreation()) * math.Pow(1+RewardIncrementPercent/100, float64((block.Height())/RewardIncrementDuration)))
		miner.balance += filledBlockReward
		miner.blockReward += filledBlockReward
		wm.ReAssignVotePower(miner, int64(filledBlockReward))
	}
}

//RevertBlockReward rewards miner for the block creation
func (wm *WalletManager) RevertBlockReward(block *core.Block, miner *Wallet) {
	if block.IsEmpty() { //Giving reward to the miner for the block creation
		emptyBlockReward := int64(float64(wm.RewardForEmptyBlock()) * math.Pow(1+RewardIncrementPercent/100, float64((block.Height())/RewardIncrementDuration)))
		miner.balance -= emptyBlockReward
		miner.blockReward -= emptyBlockReward
		wm.ReAssignVotePower(miner, int64(emptyBlockReward))

	} else {
		filledBlockReward := int64(float64(wm.RewardForBlockCreation()) * math.Pow(1+RewardIncrementPercent/100, float64((block.Height())/RewardIncrementDuration)))
		miner.balance -= filledBlockReward
		miner.blockReward -= filledBlockReward
		wm.ReAssignVotePower(miner, int64(filledBlockReward))
	}
}

//InitiateRevertBlock initiates the revert block protocol. It creates a new block with only applied txs that need to be reverted
func (wm *WalletManager) InitiateRevertBlock(numTy, numDeal, numProof, numPodf, numVotes int, origBlock *core.Block) error {

	newBlock := new(core.Block)

	var rTrans []*pb.Transaction
	var rDeals []*pb.Deal
	var rProofs []*pb.ChPrPair
	var rPodfs []*pb.PoDF
	var rVotes []*pb.Vote

	transactions := origBlock.GetTransactions()
	for i, tx := range transactions {
		if i >= numTy {
			break
		}
		rTrans = append(rTrans, tx)
	}

	deals := origBlock.GetDeals()
	for i, deal := range deals {
		if i >= numDeal {
			break
		}
		rDeals = append(rDeals, deal)
	}

	proofs := origBlock.GetChPrPair()
	for i, proof := range proofs {
		if i >= numProof {
			break
		}
		rProofs = append(rProofs, proof)
	}

	podfs := origBlock.GetPodf()
	for i, podf := range podfs {
		if i >= numPodf {
			break
		}
		rPodfs = append(rPodfs, podf)
	}
	votes := origBlock.GetVotes()
	for i, vote := range votes {
		if i >= numVotes {
			break
		}
		rVotes = append(rVotes, vote)
	}

	newBlock = core.NewBareBlock(origBlock.ParentHash(), origBlock.MinerPublicKey(), rTrans, rDeals, rProofs, rVotes, rPodfs, 0, origBlock.GetRoundNum())

	err := wm.RevertWalletStates(newBlock, origBlock.MinerPublicKey().String())
	if err != nil {
		log.Error().Msgf(err.Error())
		return err
	}

	return nil
}

//ApplyPodf validates and applies the podf. In case validation is failed it reports back the number of applied podfs
func (wm *WalletManager) ApplyPodf(podf *pb.Transaction, transactionFee, bRound int64) error {

	err := wm.ValidatePodf(podf, bRound) //Validateing the Podf
	if err != nil {
		return err
	}

	//Penalize block reward from the miner
	h1, _, _ := core.PodfPayloadFromByte(podf.Body.Payload)
	repMiner := hex.EncodeToString(h1.GetMinerPublicKey()) //Address of the reported miner
	rmWallet, err := wm.GetWalletByAddress(repMiner)
	if err != nil {
		log.Error().Msgf(err.Error()) //This should never hit because it is already validated
	}

	rmWallet.balance -= wm.RewardForBlockCreation() // He is penalized full block reward
	rmWallet.blockReward -= wm.RewardForBlockCreation()
	wm.ReAssignVotePower(rmWallet, 0-int64(wm.RewardForBlockCreation()))
	log.Info().Msgf("Misbehaving miner has been punished...")

	rmWallet.podfHeights = append(rmWallet.podfHeights, h1.Height)
	if len(rmWallet.podfHeights)%blockThresholdToBan == 0 {
		rmWallet.bannedTill = h1.GetHeight() + banDuration
	}

	//Rewarding the node who reported the misbehaving the miner
	senderAdd := podf.Body.GetSenderId()
	if wm.CheckEntryByAddress(senderAdd) == false { //Not present in map
		wm.CheckAndPutWalletByAddress(senderAdd, NewWallet(senderAdd))
	}
	sWallet, err := wm.GetWalletByAddress(senderAdd)
	if err != nil {
		log.Error().Msgf(err.Error()) //This should never hit because it is already validated
	}
	sWallet.balance += (wm.RewardForPodf() - transactionFee)
	wm.ReAssignVotePower(sWallet, int64(wm.RewardForPodf()-transactionFee))

	return nil
}

//RevertPodf reverts the podf
func (wm *WalletManager) RevertPodf(podf *pb.Transaction, transactionFee int64) error {

	//Penalize block reward from the miner
	h1, _, _ := core.PodfPayloadFromByte(podf.Body.Payload)
	repMiner := hex.EncodeToString(h1.GetMinerPublicKey()) //Address of the reported miner
	rmWallet, err := wm.GetWalletByAddress(repMiner)
	if err != nil {
		return err
	}

	rmWallet.balance += wm.RewardForBlockCreation() // He is penalized full block reward
	rmWallet.blockReward += wm.RewardForBlockCreation()
	wm.ReAssignVotePower(rmWallet, int64(wm.RewardForBlockCreation()))

	if len(rmWallet.podfHeights)%blockThresholdToBan == 0 {
		rmWallet.bannedTill = -1 //resetting the banned height
	}

	rmWallet.podfHeights = rmWallet.podfHeights[:len(rmWallet.podfHeights)-1] //Removing the last element

	//Rewarding the node who reported the misbehaving the miner
	senderAdd := podf.Body.GetSenderId()

	sWallet, err := wm.GetWalletByAddress(senderAdd)
	if err != nil {
		return err
	}
	sWallet.balance -= (wm.RewardForPodf() - transactionFee)

	wm.ReAssignVotePower(sWallet, 0-int64(wm.RewardForPodf()-transactionFee))

	return nil
}

//ApplyChPrPair makes payment of the successful verification of the storage deals
func (wm *WalletManager) ApplyChPrPair(chPrP []*pb.ChPrPair) (int, error) {
	//First validate the CHPrPair

	for i, cProof := range chPrP {

		dealHash := string(cProof.Dealhash)

		var ok bool
		deal := wm.node.GetDealbyHash(dealHash)
		if deal == nil {
			return i, errors.New(fmt.Sprintf("Reference deal cannot be found %v", hex.EncodeToString(cProof.Dealhash)))
		}
		//Validating the CHPrPair
		var LastTs []int64
		if LastTs, ok = wm.LastChVerificationTime[dealHash]; !ok {
			LastTs = append(LastTs, deal.GetTimestamp())
		}

		curTs := time.Unix(cProof.Timestamp, 0)
		dealExpTs := time.Unix(deal.GetExpiryTime(), 0)
		if curTs.After(dealExpTs) {

			curTs = dealExpTs //deal.GetExpiryTime()
			wm.node.AddLastChVerificationsToDb(LastTs, cProof.Dealhash)
			delete(wm.LastChVerificationTime, dealHash)
		} else {
			wm.LastChVerificationTime[dealHash] = append(wm.LastChVerificationTime[dealHash], cProof.Timestamp)
		}

		valid := wm.CheckChPrPair(deal, cProof, dealHash)
		if !valid {
			log.Info().Msgf("WalletManager: SP failed validation for deal,, Didn't pay SP %v", hex.EncodeToString(cProof.Dealhash)) // TODO: penalize SP
			continue
		}

		lastVrfTime := time.Unix(LastTs[len(LastTs)-1], 0)
		timediff := curTs.Sub(lastVrfTime).Seconds()

		wm.VerificationPayment(deal, timediff)
	}
	return 0, nil
}

//RevertChPrPair makes payment of the successful verification of the storage deals
func (wm *WalletManager) RevertChPrPair(chPrP []*pb.ChPrPair) error {
	//First validate the CHPrPair

	for _, cProof := range chPrP {
		dealHash := string(cProof.Dealhash)
		var ok bool
		deal := wm.node.GetDealbyHash(dealHash)
		if deal == nil {
			return errors.New(fmt.Sprintf("Reference deal cannot be found %v", hex.EncodeToString(cProof.Dealhash)))
		}
		//Validating the CHPrPair
		var LastTs []int64
		if LastTs, ok = wm.LastChVerificationTime[dealHash]; !ok {
			LastTs = wm.node.GetLastChVerificationsFromDb(cProof.Dealhash)
		}

		LastTs = LastTs[:len(LastTs)-1]

		if len(LastTs) <= 0 {
			LastTs = nil
			LastTs = append(LastTs, deal.GetTimestamp())
			wm.node.DeleteLastChVerificationsFromDb(cProof.Dealhash)
			delete(wm.LastChVerificationTime, dealHash)
		} else {
			wm.LastChVerificationTime[dealHash] = LastTs
		}

		valid := wm.CheckChPrPair(deal, cProof, dealHash)
		if !valid {
			log.Info().Msgf("WalletManager: SP failed validation for deal,, Didn't pay SP %v", hex.EncodeToString(cProof.Dealhash)) // TODO: penalize SP
			continue
		}

		curTs := time.Unix(cProof.Timestamp, 0)
		lastVrfTime := time.Unix(LastTs[len(LastTs)-1], 0)
		timediff := curTs.Sub(lastVrfTime).Seconds()
		wm.RevertVerificationPayment(deal, timediff)
	}
	return nil
}

//CheckChPrPair validates the ChPrPair before making changes to the wallet
func (wm *WalletManager) CheckChPrPair(deal *pb.Deal, cProof *pb.ChPrPair, dealHash string) bool {

	if cProof.Proof == nil {
		return false
	}

	pairing := wm.node.GetPosPairing()

	chl := new(pos.Chall)
	chl.FromProto(cProof.Challenge, pairing)
	p := new(pos.Proof)
	p.FromProto(cProof.Proof, pairing)

	pbKey := new(pos.PublicKey)
	pbKey.FromProto(deal.PublicKey, pairing)

	valid := pos.Verify([]byte(name), pairing, pbKey, chl, p)

	return valid
}

//VerificationPayment is the payment against verified storage time
func (wm *WalletManager) VerificationPayment(deal *pb.Deal, storageDuration float64) error {
	spAdd := hex.EncodeToString(deal.Spid)

	wm.CheckAndPutWalletByAddress(spAdd, NewWallet(spAdd))
	posts := deal.List

	for _, post := range posts {
		posterID := post.GetInfo().GetMetadata().GetPid()
		posterAdd := hex.EncodeToString(posterID)
		wm.CheckAndPutWalletByAddress(posterAdd, NewWallet(posterAdd))
		totalDealTime := deal.GetExpiryTime() - deal.GetTimestamp()

		chargedAmount := float64(post.GetInfo().GetParam().GetMaxCost()) * (float64(storageDuration) / float64(totalDealTime))
		sWallet, err := wm.GetWalletByAddress(spAdd) //for storage provider
		if err != nil {
			return err
		}
		sWallet.balance += int64(chargedAmount)
		wm.ReAssignVotePower(sWallet, int64(chargedAmount))

		if spAdd == wm.MyWallet.GetAddress() {
			log.Info().Msgf("I Got payment for storing!!")
		} else {
			log.Info().Msgf("Amount paid to the SP")
		}
	}
	return nil
}

//RevertVerificationPayment is the payment against verified storage time
func (wm *WalletManager) RevertVerificationPayment(deal *pb.Deal, storageDuration float64) error {
	spAdd := hex.EncodeToString(deal.Spid)

	wm.CheckAndPutWalletByAddress(spAdd, NewWallet(spAdd))
	posts := deal.List

	for _, post := range posts {
		posterID := post.GetInfo().GetMetadata().GetPid()
		posterAdd := hex.EncodeToString(posterID)
		wm.CheckAndPutWalletByAddress(posterAdd, NewWallet(posterAdd))
		totalDealTime := deal.GetExpiryTime() - deal.GetTimestamp()

		chargedAmount := float64(post.GetInfo().GetParam().GetMaxCost()) * (float64(storageDuration) / float64(totalDealTime))

		sWallet, err := wm.GetWalletByAddress(spAdd) //for storage provider
		if err != nil {
			return err
		}
		sWallet.balance -= int64(chargedAmount)
		wm.ReAssignVotePower(sWallet, int64(0-chargedAmount))

		log.Info().Msgf("Storage amount reverted")

		if spAdd == wm.MyWallet.GetAddress() {
			log.Info().Msgf("my storage payment reverted...")
		}
	}
	return nil
}

func getDealHash(deal *pb.Deal) []byte {
	dealBytes, _ := proto.Marshal(deal)
	dealHash := crypto.Sha256(dealBytes)
	return dealHash
}

func getDealFromBlock(b *core.Block, sampleHash []byte) (*pb.Deal, error) {

	deals := b.GetDeals()

	for _, deal := range deals {
		dealHash := getDealHash(deal)
		if string(dealHash) == string(sampleHash) {
			return deal, nil
		}
	}
	return nil, errors.New("Deal not found in the block")
}

//RevertTransactions reverts all the transactions
func (wm *WalletManager) RevertTransactions(transactions []*pb.Transaction, mWallet *Wallet, bRound int64) error {
	for _, tx := range transactions {
		transactionFee := core.GetTxFee(tx)

		if bRound <= GenesisRounds {
			transactionFee = 0 //No transaction fee for the Genesis round/s
		}

		switch txType := tx.GetType(); txType {
		case core.TxTransfer:
			err := wm.RevertTransferTx(tx, transactionFee)
			if err != nil {
				return err
			}
		case core.TxBecomeCandidate:
			err := wm.RevertDelegateRegistrationTx(tx, transactionFee)
			if err != nil {
				return err
			}
		case core.TxQuitCandidate:
			err := wm.RevertQuitDelegateTx(tx, transactionFee)
			if err != nil {
				return err
			}
		case core.TxVote:
			err := wm.RevertVote(tx, transactionFee)
			if err != nil {
				return err
			}
		case core.TxPodf:
			err := wm.RevertPodf(tx, transactionFee)
			if err != nil {
				return err
			}
		default:
			log.Info().Msgf("Invalid transaction type!!")
		}

		log.Info().Msgf("Tx fee reverting: %d", transactionFee)

		mWallet.balance -= transactionFee
		wm.ReAssignVotePower(mWallet, 0-int64(transactionFee))
		mWallet.transactionFee -= transactionFee
	}
	return nil
}

//ApplyTransactions applies transaction to the wallets. If any transaction is found in valid it returns the error and the number of transactions successfully applied
func (wm *WalletManager) ApplyTransactions(transactions []*pb.Transaction, mWallet *Wallet, bRound int64) (int, error) {

	for i, tx := range transactions {

		transactionFee := core.GetTxFee(tx)
		if bRound <= GenesisRounds {
			transactionFee = 0 //No transaction fee for the Genesis rounds
		}

		switch txType := tx.GetType(); txType {
		case core.TxTransfer:
			err := wm.ApplyTransferTx(tx, transactionFee, bRound)
			if err != nil {
				return i, err
			}
		case core.TxBecomeCandidate:
			err := wm.ApplyDelegateRegistrationTx(tx, transactionFee, bRound)
			if err != nil {
				return i, err
			}
		case core.TxQuitCandidate:
			err := wm.ApplyQuitDelegateTx(tx, transactionFee, bRound)
			if err != nil {
				return i, err
			}
		case core.TxVote:
			err := wm.ApplyVote(tx, transactionFee, bRound)
			if err != nil {
				return i, err
			}
		case core.TxPodf:
			err := wm.ApplyPodf(tx, transactionFee, bRound)
			if err != nil {
				return i, err
			}

		default:
			log.Info().Msgf("Invalid transaction type!!")
			return i, errors.New("Invalid type found in transaction")
		}

		mWallet.balance += transactionFee
		wm.ReAssignVotePower(mWallet, int64(transactionFee))
		mWallet.transactionFee += transactionFee

	}

	return 0, nil
}

//ApplyTransferTx validates and applies the retrieval/transfer transaction. If invalid it returns the error
func (wm *WalletManager) ApplyTransferTx(tx *pb.Transaction, transactionFee, bRound int64) error {
	//transactionFee will be zero in case bRound is less than the GenesisRound
	payerAdd := tx.GetBody().GetSenderId()
	recipientAdd := tx.GetBody().GetRecipientId()
	//Check if the transaction is executable
	//For payer
	if wm.CheckEntryByAddress(payerAdd) == false { //Not present in map
		wm.CheckAndPutWalletByAddress(payerAdd, NewWallet(payerAdd))
	}
	//For receiver
	if wm.CheckEntryByAddress(recipientAdd) == false { //If recipient is not present in the map
		wm.CheckAndPutWalletByAddress(recipientAdd, NewWallet(recipientAdd))
	}

	_, err := wm.IsExecutable(tx, bRound) //Validating the transactions
	if err != nil {
		log.Info().Msgf("Transaction is not valid: %s", err.Error())
		return err
	}

	pWallet, err := wm.GetWalletByAddress(payerAdd) //for payer
	if err != nil {
		return err
	}
	pWallet.balance -= (tx.GetBody().GetDecibels() + transactionFee)
	delta := 0 - int64(tx.GetBody().GetDecibels()+transactionFee)
	wm.ReAssignVotePower(pWallet, delta)

	pWallet.SetNonce(pWallet.GetNonce() + 1)

	rWallet, err := wm.GetWalletByAddress(recipientAdd) //For recipient
	if err != nil {
		return err
	}
	rWallet.balance += tx.GetBody().GetDecibels()
	wm.ReAssignVotePower(rWallet, int64(tx.GetBody().GetDecibels()))

	if recipientAdd == wm.MyWallet.address { //If transaction's recepient address is in the transaction
		log.Info().Msgf("yay I received some decibels!!!")
	}
	if payerAdd == wm.MyWallet.address {
		log.Info().Msgf("Decibels got deducted")
	}

	log.Info().Msgf("Updated Wallet with ret/transfer tx")

	return nil
}

//RevertTransferTx reverts the retrieval transaction
func (wm *WalletManager) RevertTransferTx(tx *pb.Transaction, transactionFee int64) error {
	//transactionFee will be zero in case bRound is less than the GenesisRound

	payerAdd := tx.GetBody().GetSenderId()
	recipientAdd := tx.GetBody().GetRecipientId()

	pWallet, err := wm.GetWalletByAddress(payerAdd) //for payer
	if err != nil {
		return err
	}
	pWallet.balance += (tx.GetBody().GetDecibels() + transactionFee)
	delta := int64(tx.GetBody().GetDecibels() + transactionFee)
	wm.ReAssignVotePower(pWallet, delta)

	pWallet.SetNonce(pWallet.GetNonce() - 1)

	rWallet, err := wm.GetWalletByAddress(recipientAdd) //For recipient
	if err != nil {
		return err
	}
	rWallet.balance -= tx.GetBody().GetDecibels()
	wm.ReAssignVotePower(rWallet, 0-int64(tx.GetBody().GetDecibels()))

	if recipientAdd == wm.MyWallet.address { //If transaction's recepient address is in the transaction
		log.Info().Msgf("My Decibels deducted-> Revert tx")
	}
	if payerAdd == wm.MyWallet.address {
		log.Info().Msgf("I got back decibels-> Revert tx")
	}

	log.Info().Msgf("Reverted Wallet with ret/transfer tx")

	return nil
}

//ApplyDelegateRegistrationTx  validates and applies the delegate registration. Return invalid in case error is found
func (wm *WalletManager) ApplyDelegateRegistrationTx(tx *pb.Transaction, transactionFee, bRound int64) error {
	//transactionFee will be zero in case bRound is less than the GenesisRound
	payerAdd := tx.GetBody().GetSenderId()
	//For payer
	if wm.CheckEntryByAddress(payerAdd) == false { //Not present in map
		wm.CheckAndPutWalletByAddress(payerAdd, NewWallet(payerAdd))
	}
	valid, err := wm.IsExecutable(tx, bRound)
	if valid != true {
		log.Info().Msgf("Transaction is not valid: %s", err.Error())
		return err
	}
	pWallet, err := wm.GetWalletByAddress(payerAdd) //for payer
	if err != nil {
		return err
	}
	err = wm.Executor.ExecuteRegisterCandidate(tx)
	if err != nil {
		return err
	}
	pWallet.isCandidate = true //setting candidacy as true

	pWallet.balance -= (tx.GetBody().GetDecibels() + transactionFee)
	delta := 0 - int64(tx.GetBody().GetDecibels()+transactionFee)
	wm.ReAssignVotePower(pWallet, delta)

	pWallet.SetNonce(pWallet.GetNonce() + 1)

	if payerAdd == wm.MyWallet.address {
		log.Info().Msgf("Decibels got deducted")
	}
	log.Info().Msgf("Updated Wallet with delegate registration tx")
	return nil
}

//ApplyDelRegTxForGenesis applies the delegate registration
func (wm *WalletManager) ApplyDelRegTxForGenesis(tx *pb.Transaction, transactionFee, bRound int64) error {
	//transactionFee will be zero in case bRound is less than the GenesisRound
	payerAdd := tx.GetBody().GetSenderId()
	//For payer
	if wm.CheckEntryByAddress(payerAdd) == false { //Not present in map
		wm.CheckAndPutWalletByAddress(payerAdd, NewWallet(payerAdd))
	}
	_, err := wm.IsExecutable(tx, bRound)
	pWallet, err := wm.GetWalletByAddress(payerAdd) //for payer
	if err != nil {
		return err
	}
	wm.Executor.ExecuteRegisterCandidate(tx)

	pWallet.isCandidate = true

	pWallet.SetNonce(pWallet.GetNonce() + 1)

	if payerAdd == wm.MyWallet.address {
		log.Info().Msgf("Got Officially registered through DR-transaction(Ignore errors)")
	}
	log.Info().Msgf("Updated Wallet with delegate registration tx")
	return nil
}

//RevertDelegateRegistrationTx applies the delegate registration
func (wm *WalletManager) RevertDelegateRegistrationTx(tx *pb.Transaction, transactionFee int64) error {
	payerAdd := tx.GetBody().GetSenderId()

	pWallet, err := wm.GetWalletByAddress(payerAdd) //for payer
	if err != nil {
		return err
	}
	err = wm.Executor.ExecuteQuitCandidate(tx)
	if err != nil {
		return err
	}
	pWallet.isCandidate = false

	pWallet.balance += (tx.GetBody().GetDecibels() + transactionFee)
	delta := int64(tx.GetBody().GetDecibels() + transactionFee)
	wm.ReAssignVotePower(pWallet, delta)

	pWallet.SetNonce(pWallet.GetNonce() - 1)

	if payerAdd == wm.MyWallet.address {
		log.Info().Msgf("Got decibels-> RevertRegisterCand")
	}
	log.Info().Msgf("Revert Wallet with delegate registration tx")
	return nil
}

//ApplyQuitDelegateTx applies the delegate quit transaction
func (wm *WalletManager) ApplyQuitDelegateTx(tx *pb.Transaction, transactionFee, bRound int64) error {
	payerAdd := tx.GetBody().GetSenderId()
	//For payer
	if wm.CheckEntryByAddress(payerAdd) == false { //Not present in map
		wm.CheckAndPutWalletByAddress(payerAdd, NewWallet(payerAdd))
	}
	valid, err := wm.IsExecutable(tx, bRound)
	if valid != true {
		log.Info().Msgf("Transaction is not valid: %s", err.Error())
		return err
	}
	pWallet, err := wm.GetWalletByAddress(payerAdd) //for payer
	if err != nil {
		return err
	}
	err = wm.Executor.ExecuteQuitCandidate(tx)
	if err != nil {
		return err
	}
	pWallet.isCandidate = false
	pWallet.balance -= (tx.GetBody().GetDecibels() + transactionFee)
	delta := 0 - int64(tx.GetBody().GetDecibels()+transactionFee)
	wm.ReAssignVotePower(pWallet, delta)

	pWallet.SetNonce(pWallet.GetNonce() + 1)

	if payerAdd == wm.MyWallet.address {
		log.Info().Msgf("Decibels got deducted")
	}
	log.Info().Msgf("Updated Wallet with delegate quit tx")
	return nil
}

//RevertQuitDelegateTx reverts the quit delegate transaction by registering the candidate again
func (wm *WalletManager) RevertQuitDelegateTx(tx *pb.Transaction, transactionFee int64) error {
	payerAdd := tx.GetBody().GetSenderId()

	pWallet, err := wm.GetWalletByAddress(payerAdd) //for payer
	if err != nil {
		return err
	}
	err = wm.Executor.ExecuteRegisterCandidate(tx)
	if err != nil {
		return err
	}
	pWallet.isCandidate = true
	pWallet.balance += (tx.GetBody().GetDecibels() + transactionFee)
	delta := int64(tx.GetBody().GetDecibels() + transactionFee)
	wm.ReAssignVotePower(pWallet, delta)

	pWallet.SetNonce(pWallet.GetNonce() - 1)

	if payerAdd == wm.MyWallet.address {
		log.Info().Msgf("Got decibels-> RevertQuitDelegate")
	}
	log.Info().Msgf("Reverted Wallet against delegate quit tx")
	return nil
}

//ReAssignVotePower assigns the votepower among those who have been voted. Called when balance is changed(_,+/-)
func (wm *WalletManager) ReAssignVotePower(voter *Wallet, delta int64) {
	for cand, percent := range voter.Voted {
		if candWallet, ok := wm.walletByAddress[cand]; ok {
			if delta > 0 {
				oldAmount := uint64((float64(percent) / float64(100)) * float64(int64(voter.balance)-delta))
				newAmount := uint64((float64(percent) / float64(100)) * float64(voter.balance))
				toAdd := newAmount - oldAmount
				candWallet.AddVotePower(toAdd)
			} else { // In case delta is negative
				oldAmount := uint64((float64(percent) / float64(100)) * float64(int64(voter.balance)-delta)) //- delta -(-)=+
				newAmount := uint64((float64(percent) / float64(100)) * float64(voter.balance))
				toSub := oldAmount - newAmount
				candWallet.SubVotePower(toSub)
			}
		}
	}
}

//IsExecutable checks if the transaction is valid
func (wm *WalletManager) IsExecutable(tx *pb.Transaction, bRound int64) (bool, error) { //Only for the transaction
	payerAdd := tx.GetBody().GetSenderId()
	txNonce := tx.GetBody().GetNonce()

	validSign, err := core.HasValidSign(tx)
	if err != nil || !validSign {
		fmt.Println("err is: ", err.Error())
		return false, err
	}

	pWallet, err := wm.GetWalletByAddress(payerAdd) //for payer
	if err != nil {
		if txNonce == 1 { //Putting wallet only when nonce is first
			wm.CheckAndPutWalletByAddress(payerAdd, NewWallet(payerAdd))
			pWallet, _ = wm.GetWalletByAddress(payerAdd) //Updating pWallet in case the new wallet has been added to the wallet manager
		} else {
			return false, err
		}
	}
	if wm.CheckNonce(txNonce, pWallet) != true { //1check:- Nonce should be in order //Shouldn't this be pwallet + 1?
		fmt.Println("got: ", txNonce, " expecting: ", pWallet.GetNonce()+1)
		log.Info().Msgf("Out of order.Nonce!! TODO:- Fix this")
		return false, errors.New("out of order nonce")
	}
	txFee := core.GetTxFee(tx)

	if bRound <= GenesisRounds {
		txFee = 0
	}

	if pWallet.GetBalance() < txFee {
		return false, errors.New("Payer can't pay fee")
	}

	switch txType := tx.Type; txType {

	case core.TxBecomeCandidate:
		exist := wm.Executor.FindCandByAdd(pWallet.GetAddress()) //Checking if candidate is already present
		if exist == true {
			return false, errors.New("Cand already exist")
		}
	case core.TxQuitCandidate:
		exist := wm.Executor.FindCandByAdd(pWallet.GetAddress()) //Checking if candidate is present
		if exist == false {
			return false, errors.New("Cand doesn't exist")
		}
	case core.TxTransfer:
		if pWallet.GetBalance() < (tx.GetBody().GetDecibels() + txFee) { //Checking if payer have sufficient balance
			return false, errors.New("Payer doesn't hold enough decibels")
		}
	case core.TxPodf:
		err := wm.ValidatePodf(tx, bRound)
		if err != nil {
			return false, err
		}
	case core.TxVote:
		valid, err := wm.ValidateVote(tx, bRound)
		if err != nil || !valid {
			return false, err
		}

	default:
		return false, errors.New("Invalid type transaction") //Checking if the type is invalid
	}

	//////////////////////////////////////////////////////////////////////////////////////
	// pWallet.SetNonce(txNonce) //Updating the nonce
	return true, nil
}

//CheckNonce checks if the nonce of the transaction is correct, as expected
func (wm *WalletManager) CheckNonce(txNonce uint64, wallet *Wallet) bool {
	return txNonce == (wallet.GetNonce() + 1)
}

//applyDeals validates and applies the changes corrosponding to the deal amount in the wallet
func (wm *WalletManager) applyDeals(deals []*pb.Deal, mWallet *Wallet, bRound int64) (int, error) {

	for i, deal := range deals {

		err := wm.ValidateDeal(deal, bRound) //validates the deal in case error is found return the successful applied deals
		if err != nil {
			return i, err
		}

		spAdd := hex.EncodeToString(deal.Spid)
		wm.CheckAndPutWalletByAddress(spAdd, NewWallet(spAdd))
		posts := deal.List

		var totalFee float64 = 0
		var perPostFee float64 = 0

		for _, post := range posts {
			perPostFee = float64(core.GetTxFee(post)) //New post fee
			if bRound <= GenesisRounds {
				perPostFee = 0 //No transaction fee for the Genesis rounds
			}

			posterAdd := hex.EncodeToString(post.GetInfo().GetMetadata().GetAuthorAddress())

			wm.CheckAndPutWalletByAddress(posterAdd, NewWallet(posterAdd))

			chargedAmount := post.GetInfo().GetParam().GetMaxCost()

			pWallet, err := wm.GetWalletByAddress(posterAdd) //for poster
			if err != nil {
				log.Error().Msgf(err.Error())
			}

			if pWallet.balance < (int64(chargedAmount) + int64(perPostFee)) {
				panic("Deal payment not possible-> Balance will go in negative. Handle this")
			}
			pWallet.balance -= (int64(chargedAmount) + int64(perPostFee))
			toSub := int64(0 - chargedAmount - uint32(perPostFee))
			wm.ReAssignVotePower(pWallet, int64(toSub))

			totalFee += perPostFee

		}

		mWallet.balance += int64(totalFee)
		wm.ReAssignVotePower(mWallet, int64(totalFee))
		mWallet.transactionFee += int64(totalFee)
	}

	return 0, nil
}

//PreValidatePost prevalidate the post before adding in the block
func (wm *WalletManager) PreValidatePost(post *pb.SignedPostInfo) bool {

	posterID := post.GetInfo().GetMetadata().GetPid()
	posterAdd := hex.EncodeToString(posterID)

	if _, ok := wm.walletCache[posterAdd]; !ok {

		if wm.CheckEntryByAddress(posterAdd) {
			pWallet, _ := wm.GetWalletByAddress(posterAdd) //for poster
			wm.walletCache[posterAdd] = pWallet.DeepCopy()
		} else {
			wm.walletCache[posterAdd] = NewWallet(posterAdd) //If it is a new poster
		}
	}
	posterWallet := wm.walletCache[posterAdd]
	chargedAmount := post.GetInfo().GetParam().GetMaxCost()
	perPostFee := float64(core.GetTxFee(post)) //post fee

	// if !wm.CheckPostNonce(post.GetInfo().GetMetadata().GetNonce(), posterWallet) {

	// 	// invalid = append(invalid, post) //array for the invalid posts
	// 	// invalidIndex[i] = true
	// 	log.Info().Msgf("WM: Invalid post, because of nonce, current nonce %v expected %v", post.GetInfo().GetMetadata().GetNonce(), posterWallet.GetPostNonce())
	// 	return false
	// } else if posterWallet.balance < (int64(chargedAmount) + int64(perPostFee)) {
	if posterWallet.balance < (int64(chargedAmount) + int64(perPostFee)) {
		log.Info().Msgf("WM: Invalid post, because of balance current %v charged amount %v", posterWallet.balance, (int64(chargedAmount) + int64(perPostFee)))
		return false
	} else {
		posterWallet.balance -= (int64(chargedAmount) + int64(perPostFee))

	}

	return true
}

//PreValidatePostRevertWalletCache revert balcne anc nonce incase post is not stores at SP
func (wm *WalletManager) RevertPreValidatePostWalletCache(post *pb.SignedPostInfo) {

	posterID := post.GetInfo().GetMetadata().GetPid()
	posterAdd := hex.EncodeToString(posterID)

	if _, ok := wm.walletCache[posterAdd]; !ok {
		return
	}

	posterWallet := wm.walletCache[posterAdd]
	chargedAmount := post.GetInfo().GetParam().GetMaxCost()
	perPostFee := float64(core.GetTxFee(post)) //post fee

	posterWallet.balance += (int64(chargedAmount) + int64(perPostFee))

	return
}

//ValidateDeal checks if deal can be applied of is valid or not
func (wm *WalletManager) ValidateDeal(deal *pb.Deal, bRound int64) error {
	posts := deal.List
	transactionFee := core.GetTxFee(deal)
	perPostFee := float64(transactionFee) / float64(len(posts))
	localWalletCache := make(map[string]*Wallet)
	for _, post := range posts {
		perPostFee = float64(core.GetTxFee(post)) //New post fee
		if bRound <= GenesisRounds {
			perPostFee = 0 //No transaction fee for the Genesis rounds
		}

		posterID := post.GetInfo().GetMetadata().GetPid()
		posterAdd := hex.EncodeToString(posterID)
		var pWallet, pWallet1 *Wallet
		var ok bool
		var err error
		if pWallet, ok = localWalletCache[posterAdd]; !ok {
			pWallet1, err = wm.GetWalletByAddress(posterAdd) //for poster
			if err != nil {
				pWallet1 = NewWallet(posterAdd) //If it is a new poster
			}
			localWalletCache[posterAdd] = pWallet1.DeepCopy()
			pWallet = localWalletCache[posterAdd]
		}

		chargedAmount := post.GetInfo().GetParam().GetMaxCost()

		if pWallet.balance < (int64(chargedAmount) + int64(perPostFee)) {
			return errors.New("insufficient payer balance")
		}
	}
	return nil
}

//RevertDeals reverse the deals in the block
func (wm *WalletManager) RevertDeals(deals []*pb.Deal, mWallet *Wallet, bRound int64) error {

	for _, deal := range deals {

		spAdd := hex.EncodeToString(deal.Spid)
		wm.CheckAndPutWalletByAddress(spAdd, NewWallet(spAdd))
		posts := deal.List

		var totalFee float64 = 0
		var perPostFee float64 = 0

		for _, post := range posts {
			perPostFee = float64(core.GetTxFee(post)) //New post fee
			if bRound <= GenesisRounds {
				perPostFee = 0 //No deal fee for the Genesis rounds
			}

			posterID := post.GetInfo().GetMetadata().GetPid()
			posterAdd := hex.EncodeToString(posterID)

			chargedAmount := post.GetInfo().GetParam().GetMaxCost()

			pWallet, err := wm.GetWalletByAddress(posterAdd) //for poster
			if err != nil {
				return err
			}

			pWallet.balance += (int64(chargedAmount) + int64(perPostFee))
			toAdd := int64(chargedAmount + uint32(perPostFee))
			wm.ReAssignVotePower(pWallet, int64(toAdd))

			if posterAdd == wm.MyWallet.GetAddress() {
				log.Info().Msgf("I am Paid for posting!!")
			}
			totalFee += perPostFee
		}

		mWallet.balance -= int64(totalFee)
		wm.ReAssignVotePower(mWallet, int64(0-totalFee))
		mWallet.transactionFee -= int64(totalFee)
	}
	return nil
}

//PrintWalletBalances prints the balance of all the users node has encountered from blocks
func (wm *WalletManager) PrintWalletBalances() {

	fmt.Println("Printing the states of all the wallets...")
	fmt.Println("Currently MY balance is: ", wm.MyWallet.GetBalance(), ". Block reward: ", wm.MyWallet.blockReward, " Tx fee: ", wm.MyWallet.transactionFee)
	for add, wallet := range wm.walletByAddress {
		fmt.Println(add, " has decibels ", wallet.GetBalance(), ". Block reward: ", wallet.blockReward, " Tx Fee: ", wallet.transactionFee, " VP: ", wallet.votePower, "Roll back: ", wallet.rollBackAmount)
	}
	fmt.Println("**********************")
}

//PrintAllAccounts prints the add of all the accounts
func (wm *WalletManager) PrintAllAccounts() {

	fmt.Println("Printing all accounts:")
	fmt.Println("My Address: ", wm.MyWallet.address)
	for add := range wm.walletByAddress {
		fmt.Println(add)
	}
	fmt.Println("**********************")
}

//RewardForBlockCreation returns the amount of the block creation reward
func (wm *WalletManager) RewardForBlockCreation() int64 {
	return 10
}

//RewardForEmptyBlock adds the reward for the empty block
func (wm *WalletManager) RewardForEmptyBlock() int64 {
	return 5
}

//RewardForPodf returns the reward for podf
func (wm *WalletManager) RewardForPodf() int64 {
	return wm.RewardForBlockCreation() / 2
}

//TransactionFeePercentage returns the percentage of the transaction fee
func (wm *WalletManager) TransactionFeePercentage() float64 {
	return 0.1
}

//ApplyVote checks and applies the vote in the vote transation.
func (wm *WalletManager) ApplyVote(vote *pb.Transaction, transactionFee, bRound int64) error {
	log.Info().Msgf("Applying vote...")

	_, err := wm.ValidateVote(vote, bRound) //validating the vote
	if err != nil {
		return err
	}

	voterID := vote.Body.SenderId

	vWallet, err := wm.GetWalletByAddress(voterID) //Fetching the voter wallet
	if err != nil {
		log.Warn().Msgf("No transaction done before to put wallet entry. For now just adding the wallet")
		wm.CheckAndPutWalletByAddress(voterID, NewWallet(voterID))
		vWallet, _ = wm.GetWalletByAddress(voterID)
	}

	vWallet.SetNonce(vWallet.GetNonce() + 1)

	//////// Make payment for the transaction against the vote
	vWallet.balance -= transactionFee

	wm.ReAssignVotePower(vWallet, 0-transactionFee)

	votedCands, _ := core.VotePayloadFromByte(vote.GetBody().Payload)
	for _, votedCand := range votedCands {

		action, percent, candID, err := ParseVote(votedCand)
		if err != nil { //Will never hit coz it has already bein verified

			return err
		}

		if action == "-" { //Corrosponding to unvote

			err = vWallet.CheckUnvoteValidity(percent, candID)
			if err != nil { //Will never hit coz it has already been verified
				return err
			}
			vWallet.ExecuteUnvote(percent, candID)
			vWallet.AddVoteLeft(percent)
			wm.SubVotePower(candID, percent, vWallet)

		} else { //corrosponding to the vote
			err = vWallet.CheckVoteValidity(percent, candID)
			if err != nil { //Will never hit coz it has already been verified
				return err
			}
			vWallet.ExecuteVote(percent, candID)
			vWallet.SubVoteLeft(percent)
			wm.AddVotePower(candID, percent, vWallet)
		}

	}

	return nil
}

//AddVotePower updates the votepower(+ + +)
func (wm *WalletManager) AddVotePower(candID string, percentChange int, voter *Wallet) error {
	if cand, ok := wm.walletByAddress[candID]; ok {
		totalPercentage := voter.Voted[candID]
		totalAmount := uint64((float64(totalPercentage) / float64(100)) * float64(voter.GetBalance()))
		prevAmount := uint64((float64(totalPercentage-percentChange) / float64(100)) * float64(voter.GetBalance()))
		toAdd := totalAmount - prevAmount
		cand.AddVotePower(toAdd)
		return nil
	}
	return errors.New("Cand doesn't exist")
}

//SubVotePower subtracts the votePower of the candidate(+ + +)
func (wm *WalletManager) SubVotePower(candID string, percentChange int, voter *Wallet) error {
	if cand, ok := wm.walletByAddress[candID]; ok {
		totalPercentage := voter.Voted[candID] //This contains an updated percentage
		totalAmount := uint64((float64(totalPercentage) / float64(100)) * float64(voter.GetBalance()))
		prevAmount := uint64((float64(totalPercentage+percentChange) / float64(100)) * float64(voter.GetBalance()))
		toSub := prevAmount - totalAmount //Coz previous amount should be greater
		cand.SubVotePower(toSub)
		return nil
	}
	return errors.New("Cand doesn't exist")
}

//RevertVote undo the votes- Possible in case block is found corrupt
func (wm *WalletManager) RevertVote(vote *pb.Transaction, transactionFee int64) error {
	fmt.Println("Reverting votes...")
	voterID := vote.Body.GetSenderId()

	vWallet, err := wm.GetWalletByAddress(voterID)
	if err != nil {
		return err //Should never hit this
	}

	vWallet.SetNonce(vWallet.GetNonce() - 1) //decrementing the nonce

	vWallet.balance += transactionFee
	wm.ReAssignVotePower(vWallet, transactionFee)

	votedCands, _ := core.VotePayloadFromByte(vote.GetBody().Payload)
	for _, votedCand := range votedCands {

		action, percent, candID, _ := ParseVote(votedCand)

		if action == "-" { //Corrosponding to unvote

			vWallet.ExecuteVote(percent, candID)
			vWallet.SubVoteLeft(percent)
			wm.AddVotePower(candID, percent, vWallet)

		} else {

			vWallet.ExecuteUnvote(percent, candID)
			vWallet.AddVoteLeft(percent)
			wm.SubVotePower(candID, percent, vWallet)
		}
	}
	return nil
}

//GetCandVotePower returns the votepower of the candidate
func (wm *WalletManager) GetCandVotePower(candID string) (uint64, int64, error) {
	if cand, ok := wm.walletByAddress[candID]; ok {
		return cand.GetVotePower(), cand.bannedTill, nil
	}

	return 0, -1, errors.New("Candidate doesn't exist")
}

//GetRegisteredCandidates returns if the add is also the candidate(has done cand reg tx in the past)
func (wm *WalletManager) GetRegisteredCandidates() []string {

	var cands []string

	for add, w := range wm.walletByAddress {
		if w.IsCandidate() {
			cands = append(cands, add)
		}
	}
	return cands
}

//PrintWalletVotes prints the votes of all the wallets
func (wm *WalletManager) PrintWalletVotes() {
	fmt.Println("Printing the votes of all the wallets...")
	for add, wallet := range wm.walletByAddress {
		fmt.Println(add, " voted: ")
		wallet.PrintVotes()
	}
	fmt.Println("**********************")
}

//PrintVotePower prints the votepower of the candidate
func (wm *WalletManager) PrintVotePower() {
	fmt.Println("Printing votepower(VP)...")
	for add, wallet := range wm.walletByAddress {
		fmt.Println(add, " VP: ", wallet.GetVotePower())
	}
	fmt.Println("*************************")
}

//CleanAndAddBlockState puts the wallet state in the block
func (wm *WalletManager) CleanAndAddBlockState(block *core.Block) {

	//Removing wallets which were created and not used
	for k, wal := range wm.walletByAddress {
		if wal.UnusedWallet() {
			delete(wm.walletByAddress, k)
		}
	}

	//Appending the block state in the block
	count := 0
	for id, wallet := range wm.walletByAddress { //putting all the balances in the block
		// block.PbBlock.State.AccountState[id] = wallet.GetBalance()
		entry := &pb.AccState{
			Pk:          id,
			Balance:     wallet.GetBalance(),
			Votes:       wallet.GetVoteArray(),
			IsCandidate: wallet.isCandidate,
			Nonce:       wallet.nonce,
		}
		if len(wallet.podfHeights) > 0 { //will only create field if the podf heights exists
			entry.PodfHeights = make([]int64, len(wallet.podfHeights))
			copy(entry.PodfHeights, wallet.podfHeights)
		}

		block.PbBlock.State.AccountStates[count] = entry
		count++
	}
	wm.walletByAddress[wm.MyWallet.address] = wm.MyWallet //in case my unused wallet gets deleted

}

//ResetWalletManagerState resets the amount in the wallet to the rollback amount
func (wm *WalletManager) ResetWalletManagerState(rollBack int) {
	var rollBackMoney int64
	for _, wallet := range wm.walletByAddress {
		initBal := wallet.GetBalance()
		rollBackMoney = int64((float64(rollBack) / float64(100)) * float64(initBal))
		wallet.SetRollBackAmount(rollBackMoney)
		wallet.SetBalance(rollBackMoney)

		delta := int64(rollBackMoney) - int64(initBal) //Will be negative
		wm.ReAssignVotePower(wallet, delta)
		//Setting all components to zero
		wallet.SetBlockReward(0)
		wallet.SetTransactionFee(0)
		wallet.SetProofReward(0)
	}

}

//UndoDigestReset undo the wallet reset done by the digest block
func (wm *WalletManager) UndoDigestReset(rollBack int) {
	var rollBackMoney int64
	for _, wallet := range wm.walletByAddress {
		initBal := wallet.GetBalance()
		rollBackMoney = int64((float64(100) / float64(rollBack)) * float64(initBal))
		wallet.SetBalance(rollBackMoney)
		wallet.SetRollBackAmount(0)

		delta := int64(rollBackMoney) - int64(initBal) //Will be positive (current - previous amount)
		wm.ReAssignVotePower(wallet, delta)

	}
}

//GetDigestBlockReward is a function to claim the digest block reward
func (wm *WalletManager) GetDigestBlockReward() {
	myWallet := wm.walletByAddress[wm.MyWallet.GetAddress()]

	myWallet.balance += 10 * wm.RewardForBlockCreation() //More reward for the epoch block
	myWallet.blockReward += 10 * wm.RewardForBlockCreation()
	wm.ReAssignVotePower(myWallet, int64(10*wm.RewardForBlockCreation()))
}

//ValidateDigestBlock validates if the local balances matches with the ones appended in the block
func (wm *WalletManager) ValidateDigestBlock(digest *core.Block) error {

	//Cleaning up wallet which is not used
	for k, wal := range wm.walletByAddress {
		if wal.UnusedWallet() {
			delete(wm.walletByAddress, k)
		}
	}
	defer func() {
		wm.walletByAddress[wm.MyWallet.address] = wm.MyWallet //in case my unused wallet gets deleted
	}()

	bWalletState := digest.PbBlock.GetState().GetAccountStates()
	if len(bWalletState) != len(wm.walletByAddress) {
		log.Info().Err(errors.New("No of entries doesn't match"))
		return errors.New("No of entries doesn't match")
	}

	duplicateCheck := make(map[string]bool) //To ensure that no duplicate entry of a user is added to the digest block

	for _, entry := range bWalletState {
		id, bal := entry.GetPk(), entry.GetBalance()
		if _, ok := duplicateCheck[id]; ok {
			log.Info().Msgf("Duplicate entry found")
			return errors.New("Duplicate entry found")
		}
		duplicateCheck[id] = true

		votes := entry.GetVotes()
		if wal, ok := wm.walletByAddress[id]; ok {
			if wal.balance != bal { //checking the balance
				//Balance mismath
				log.Info().Err(errors.New("Entries balance mismath"))
				return errors.New("Entries balance mismath")
			}
			if wal.nonce != entry.Nonce {
				return errors.New("Mismatch nonce")
			}
			if wal.isCandidate != entry.IsCandidate {
				return errors.New("Wrong candidate flag")
			}
			if len(wal.podfHeights) != len(entry.PodfHeights) {
				return errors.New("Wrong podf lengths")
			} else if len(wal.podfHeights) != 0 {
				for i, val := range wal.podfHeights {
					if val != entry.PodfHeights[i] {
						return errors.New("Wrong podfs entry in slice")
					}
				}
			}

			for _, vote := range votes {
				_, percentVote, cand, err := ParseVote(vote)
				if err != nil {
					log.Info().Err(err)
					return err
				}
				if walPercent, ok := wal.Voted[cand]; ok {
					if walPercent != percentVote {
						fmt.Println(walPercent, percentVote)
						log.Info().Msgf("Vote percentage not equal")
						return errors.New("Vote percentage is not equal")
					}

				} else {
					log.Info().Msgf("Vote entry doesn't exists")
					return errors.New("Vote entry doesn't exists")
				}

			}

		} else {
			log.Info().Err(errors.New("Entry doesn't exist"))
			return errors.New("Entry doesn't exist")
		}
	}
	return nil
}

//ResetWalletCache resets cache
func (wm *WalletManager) ResetWalletCache() {
	wm.walletCache = make(map[string]*Wallet)
}
