package wallet

import (
	// "sync"
	"errors"
	"fmt"
	"strconv"

	"github.com/guyu96/go-timbre/core"
	"github.com/guyu96/go-timbre/log"
	pb "github.com/guyu96/go-timbre/protobuf"
)

var (
	//BaseBalance is the initial balance of every wallet entry
	BaseBalance int64 = 10000000
)

//Wallet maintains the balance of the balance of the account
type Wallet struct {
	isCandidate    bool
	podfHeights    []int64
	bannedTill     int64 //Height till the miner is banned
	votePower      uint64
	address        string
	nonce          uint64
	Voted          map[string]int
	voteLeft       int   //Vote balance left in the wallet of the user
	balance        int64 //Total amount in the account
	blockReward    int64 //Fixed block reward for mining the block
	proofReward    int64 //This is the reward upon successful deal verification
	rollBackAmount int64 //Rollback amount is the number of decibels being received by the user from previous epoch
	transactionFee int64
}

//NewWallet creates the wallet instance
func NewWallet(userAddress string) *Wallet {

	return &Wallet{
		isCandidate:    false,
		bannedTill:     -1,
		votePower:      0, //Vote power I am currently having in the network- Got by voting
		address:        userAddress,
		Voted:          make(map[string]int),
		balance:        BaseBalance,
		voteLeft:       100, //Percentage of vote left in the wallet
		blockReward:    0,
		proofReward:    0,
		transactionFee: 0,
		nonce:          0,
		rollBackAmount: 0,
	}
}

//NewWalletFromDigestEntry returns the new instance of the wallet from disgest entry
func NewWalletFromDigestEntry(entry *pb.AccState) (*Wallet, error) {

	w := &Wallet{
		isCandidate:    entry.IsCandidate,
		address:        entry.Pk,
		votePower:      0,
		Voted:          make(map[string]int),
		balance:        entry.Balance,
		nonce:          entry.Nonce,
		voteLeft:       100,
		blockReward:    0,
		proofReward:    0,
		transactionFee: 0,
		rollBackAmount: 0,
		bannedTill:     -1,
	}
	for _, h := range entry.PodfHeights {
		w.podfHeights = append(w.podfHeights, h)
	}
	if len(w.podfHeights) > 0 && len(w.podfHeights)%blockThresholdToBan == 0 {
		w.bannedTill = w.podfHeights[len(w.podfHeights)-1] + banDuration //update the banned till in case it crosses the threshold
	}

	votes := entry.GetVotes()
	for _, vote := range votes {

		action, percent, candID, err := ParseVote(vote)
		if err != nil {
			return nil, err
		}

		if action == "-" { //Corrosponding to unvote
			return nil, errors.New("Wrong vote action in digest block")
		}

		err = w.CheckVoteValidity(percent, candID)
		if err != nil { //Will never hit coz it has already been verified
			return nil, err
		}
		w.Voted[candID] = percent
		w.SubVoteLeft(percent)
	}

	return w, nil
}

//DeepCopy deep copies the wallet
func (w *Wallet) DeepCopy() *Wallet {
	copyWallet := &Wallet{
		isCandidate:    w.isCandidate,
		bannedTill:     w.bannedTill,
		votePower:      w.votePower,
		address:        w.address,
		balance:        w.balance,
		voteLeft:       w.voteLeft,
		blockReward:    w.blockReward,
		transactionFee: w.transactionFee,
		nonce:          w.nonce,
		rollBackAmount: w.rollBackAmount,
		podfHeights:    make([]int64, len(w.podfHeights)),
		Voted:          make(map[string]int),
	}
	copy(copyWallet.podfHeights, w.podfHeights) //Copies the podf heights
	for k, v := range w.Voted {
		copyWallet.Voted[k] = v
	}

	return copyWallet
}

//UnusedWallet marks wallet unused
func (w *Wallet) UnusedWallet() bool {
	if w.balance == BaseBalance && w.voteLeft == 100 && w.blockReward == 0 && w.transactionFee == 0 && w.isCandidate == false {
		return true
	}
	return false
}

//DeepCopyToPbWallet gives the protobuf struct of the wallet
func (w *Wallet) DeepCopyToPbWallet() *pb.Wallet {

	pbWallet := &pb.Wallet{
		IsCandidate:    w.isCandidate,
		BannedTill:     w.bannedTill,
		VotePower:      w.votePower,
		Address:        w.address,
		Balance:        w.balance,
		VoteLeft:       int32(w.voteLeft),
		BlockReward:    w.blockReward,
		TransactionFee: w.transactionFee,
		Nonce:          w.nonce,
		RollBackAmount: w.rollBackAmount,
		PodfHeights:    make([]int64, len(w.podfHeights)),
		Voted:          make(map[string]int32),
	}

	copy(pbWallet.PodfHeights, w.podfHeights) //Copies the podf heights
	for k, v := range w.Voted {
		pbWallet.Voted[k] = int32(v)
	}
	return pbWallet
}

//FormProtoWallet returns wallet from proto wallet
func FormProtoWallet(w *pb.Wallet) *Wallet {

	wal := &Wallet{
		isCandidate:    w.IsCandidate,
		bannedTill:     w.BannedTill,
		votePower:      w.VotePower,
		address:        w.Address,
		balance:        w.Balance,
		voteLeft:       int(w.VoteLeft),
		blockReward:    w.BlockReward,
		transactionFee: w.TransactionFee,
		nonce:          w.Nonce,
		rollBackAmount: w.RollBackAmount,
		podfHeights:    make([]int64, len(w.PodfHeights)),
		Voted:          make(map[string]int),
	}

	copy(wal.podfHeights, w.PodfHeights) //Copies the podf heights

	for k, v := range w.Voted {
		wal.Voted[k] = int(v)
	}

	return wal
}

//SetCandidate sets the value of isCandidate field
func (w *Wallet) SetCandidate(flag bool) {
	w.isCandidate = flag
}

//IsCandidate return the candidacy status of the user
func (w *Wallet) IsCandidate() bool {
	return w.isCandidate
}

//GetBannedTill returns the banned till value
func (w *Wallet) GetBannedTill() int64 {
	return w.bannedTill
}

//GetRollBackAmount returns the rollback amount in the wallet
func (w *Wallet) GetRollBackAmount() int64 {
	return w.rollBackAmount
}

//SetRollBackAmount sets the rollback amount
func (w *Wallet) SetRollBackAmount(num int64) {
	w.rollBackAmount = num
}

//SetBlockReward sets the block reward in the wallet
func (w *Wallet) SetBlockReward(num int64) {
	w.blockReward = num
}

//GetBlockReward return the block reward
func (w *Wallet) GetBlockReward() int64 {
	return w.blockReward
}

//SetProofReward set the proof reward
func (w *Wallet) SetProofReward(num int64) {
	w.proofReward = num
}

//SetTransactionFee sets the transaction fee in the wallet
func (w *Wallet) SetTransactionFee(num int64) {
	w.transactionFee = num
}

//GetBalance returns the current balance
func (w *Wallet) GetBalance() int64 {
	return w.balance
}

//GetAddress returns the address of the wallet
func (w *Wallet) GetAddress() string {
	return w.address
}

//GetNonce returns the nonce of the wallet
func (w *Wallet) GetNonce() uint64 {
	return w.nonce
}

//SetNonce updates the nonce of the wallet
func (w *Wallet) SetNonce(updated uint64) {
	w.nonce = updated
}

//SetBalance sets the balance of the wallet to the provided balance
func (w *Wallet) SetBalance(amount int64) {
	w.balance = amount
}

//ValidateTransactionAmount validates if transaction amount is valid
func (w *Wallet) ValidateTransactionAmount(transaction *core.Transaction) error {

	if transaction.GetTxDecibels() == 0 {
		return errors.New("Invalid transaction. Can't be zero")
	}
	totalAmount := transaction.GetTxDecibels() + core.GetTxFee(transaction.GetProtoTransaction())
	if w.balance < totalAmount {

		return errors.New("Balance is low")
	}
	fmt.Println("amount validated from wallet")
	return nil
}

//ValidateTransactionFee validates the amount of fee in the wallet
func (w *Wallet) ValidateTransactionFee(tx *pb.Transaction) error {

	txFee := core.GetTxFee(tx)
	if w.balance < txFee {
		return errors.New("Insufficient balance")
	}
	return nil
}

//CheckAndApplyTrans checks the transactions and see if there is transaction against his ID then apply changes to the wallet balance
func (w *Wallet) CheckAndApplyTrans(block *core.Block) error {
	isValid := block.IsValid()
	if isValid != true {
		return errors.New("Block is not valid")
	}
	transactions := block.GetTransactions()
	for _, transaction := range transactions {
		if string(transaction.GetBody().GetRecipientId()) == w.address { //If transaction's recepient address is in the transaction
			w.balance += transaction.GetBody().GetDecibels()

			log.Info().Msgf("Received a transaction!!!!!.wallet balance updated")
		}
	}
	return nil
}

//CheckUnvoteValidity checks if the unvote transaction of voted before
func (w *Wallet) CheckUnvoteValidity(percent int, candID string) error {

	if votedPercent, ok := w.Voted[candID]; ok {
		if votedPercent < percent {
			return errors.New("Unvote percent is greater and invalid")
		}
		return nil
	}
	return errors.New("Not voted before. Can't unvote")
}

//CheckVoteValidity validates the upvote
func (w *Wallet) CheckVoteValidity(percent int, candID string) error {
	if percent > w.voteLeft {
		return errors.New("Vote exceeds the leftover vote balance")
	}
	return nil
}

//ExecuteVote executes the vote
func (w *Wallet) ExecuteVote(toAdd int, candID string) {
	if votedPercent, ok := w.Voted[candID]; ok {
		w.Voted[candID] = votedPercent + toAdd
		return
	}
	w.AddVotedCand(toAdd, candID)
}

//ExecuteUnvote applies the unvote
func (w *Wallet) ExecuteUnvote(toSub int, candID string) error {
	percent, _ := w.Voted[candID]
	updated := percent - toSub
	if updated == 0 {
		err := w.DelVotedCand(candID)
		if err != nil {
			return err
		}
	}
	w.Voted[candID] = updated
	return nil
}

//AddVotedCand adds the cand entry to the voted
func (w *Wallet) AddVotedCand(percent int, candID string) error {
	if _, ok := w.Voted[candID]; ok {
		errors.New("Entry already exists")
	}
	w.Voted[candID] = percent
	return nil
}

//DelVotedCand deletes the candidate for the voted map
func (w *Wallet) DelVotedCand(candID string) error {
	if _, ok := w.Voted[candID]; ok {
		delete(w.Voted, candID)
		return nil
	}
	return errors.New("Entry doesn't exists")
}

//AddVoteLeft adds the balance to the voteBalance
func (w *Wallet) AddVoteLeft(num int) {
	w.voteLeft += num
}

//SubVoteLeft subtracts the voteBalance
func (w *Wallet) SubVoteLeft(num int) {
	w.voteLeft -= num
}

//GetVoteLeft returns the vote left in the wallet
func (w *Wallet) GetVoteLeft() int {
	return w.voteLeft
}

//AddVotePower add to the vote power
func (w *Wallet) AddVotePower(num uint64) {
	w.votePower += num
}

//SubVotePower subtracts votepower
func (w *Wallet) SubVotePower(num uint64) {
	w.votePower -= num
}

//GetCandVotePercentage returns the percentage of the vote voted to the candidate
func (w *Wallet) GetCandVotePercentage(candID string) (int, error) {

	if _, ok := w.Voted[candID]; ok {
		return w.Voted[candID], nil
	}

	return 0, errors.New("No vote present")
}

//GetVotePower returns votepower of the candidate
func (w *Wallet) GetVotePower() uint64 {
	return w.votePower
}

//PrintVotes print the votes in the wallet
func (w *Wallet) PrintVotes() {
	for cand, percent := range w.Voted {
		fmt.Println("  ->", cand, " PercentVP", percent, " ")
	}
}

//GetVoteArray returns the array of the vote casted by the wallet owner
func (w *Wallet) GetVoteArray() []string {

	votes := []string{}
	for k, v := range w.Voted {
		vote := "+" + strconv.Itoa(v) + "|" + k
		votes = append(votes, vote)
	}
	return votes
}
