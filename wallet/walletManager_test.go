package wallet

import (
	"fmt"
	"testing"

	"github.com/guyu96/go-timbre/core"
	pb "github.com/guyu96/go-timbre/protobuf"
	"github.com/mbilal92/noise"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Keypair struct {
	PrivateKey noise.PrivateKey
	PublicKey  noise.PublicKey
}

//Test file require function update
//Comprehensive test for the voting protocol
func TestWalletManager_CheckAndApplyVotes(t *testing.T) {
	//creating new wallet manager
	keys := new(Keypair)
	var err error

	keys.PublicKey, keys.PrivateKey, err = noise.GenerateKeys(nil)
	voterAdd := "TestVoter"
	WM := NewWalletManager(voterAdd)
	require.NotNil(t, WM)
	testVoter := WM.MyWallet
	testVoter.SetBalance(10000)
	assert.Equal(t, testVoter.GetBalance(), int64(10000))

	//Creating test candidate wallets
	cand1 := NewWallet("cand1")
	require.NotNil(t, cand1)
	cand1.SetBalance(10000)
	assert.Equal(t, cand1.GetBalance(), int64(10000))
	err = WM.CheckAndPutWalletByAddress("cand1", cand1)
	// cand1, _ = WM.GetWalletByAddress("cand1")

	assert.Nil(t, err)
	VP, _, err := WM.GetCandVotePower("cand1")
	assert.Equal(t, uint64(0), VP)

	cand2 := NewWallet("cand2")
	require.NotNil(t, cand2)
	cand2.SetBalance(10000)
	assert.Equal(t, cand2.GetBalance(), int64(10000))
	err = WM.CheckAndPutWalletByAddress("cand2", cand2)
	assert.Nil(t, err)
	// cand2, _ = WM.GetWalletByAddress("cand2")

	VP, _, err = WM.GetCandVotePower("cand2")
	assert.Equal(t, uint64(0), VP)

	cand3 := NewWallet("cand3")
	require.NotNil(t, cand3)
	cand3.SetBalance(10000)
	assert.Equal(t, cand3.GetBalance(), int64(10000))
	err = WM.CheckAndPutWalletByAddress("cand3", cand3)
	assert.Nil(t, err)
	// cand3, _ = WM.GetWalletByAddress("cand3")
	VP, _, err = WM.GetCandVotePower("cand3")
	assert.Equal(t, uint64(0), VP)

	testMiner := NewWallet("testMiner")
	require.NotNil(t, testMiner)
	testMiner.SetBalance(10000)
	assert.Equal(t, testMiner.GetBalance(), int64(10000))
	err = WM.CheckAndPutWalletByAddress("testMiner", testMiner)
	assert.Nil(t, err)
	// testMiner, _ = WM.GetWalletByAddress("testMiner")
	VP, _, err = WM.GetCandVotePower("testMiner")
	assert.Equal(t, uint64(0), VP)
	//Vote candidate1 20percent of the power voter owned
	vote1 := "+20|cand1"

	var votes []string
	votes = append(votes, vote1)
	voteTrans, _ := core.NewVote(voterAdd, keys.PrivateKey, votes)
	vBytes, _ := voteTrans.ToBytes()
	// fmt.Println("Single vote bytes are: ", len(vBytes))
	pbVote := voteTrans.GetProtoVote()
	txFee := core.GetTxFee(pbVote)
	assert.NotEqual(t, 0, txFee)
	var voteTxs []*pb.Vote
	voteTxs = append(voteTxs, pbVote)

	block := core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, []*pb.Transaction{}, voteTxs, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0)
	require.NotNil(t, block)

	err = WM.CheckAndApplyVotes(block, "testMiner")
	assert.Nil(t, err)
	newBal := 10000 - txFee
	trueVp := int64(0.2 * float64(newBal)) //Vote should be applied after the transaction fee has been paid

	VP, _, err = WM.GetCandVotePower("cand1")
	fmt.Println("txFee", txFee, " bal: ", newBal, "True VP:", trueVp, "got: ", VP)
	assert.Equal(t, trueVp, VP)
	assert.Equal(t, 80, WM.MyWallet.GetVoteLeft())
	assert.Equal(t, int64(10000)+txFee, testMiner.GetBalance()) //Miner should be able to earn transaciton fee
	WM.PrintWalletBalances()
	//Unvoting candidate 2-> Should result in an error since candidate 2 has never been voted -> Should also result in reward reverse
	vote2 := "-30|cand2"

	votes = []string{vote2}
	voteTrans, _ = core.NewVote(voterAdd, keys.PrivateKey, votes)
	pbVote = voteTrans.GetProtoVote()
	// var voteTxs []*pb.Vote
	voteTxs = []*pb.Vote{pbVote}

	block = core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, []*pb.Transaction{}, voteTxs, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0)
	require.NotNil(t, block)

	err = WM.CheckAndApplyVotes(block, "testMiner")
	assert.Error(t, err) //Should return non nil error
	fmt.Println(err.Error())
	WM.PrintWalletBalances()

	vote3 := "+30|cand2"

	votes = []string{vote3}
	voteTrans, _ = core.NewVote(voterAdd, keys.PrivateKey, votes)
	pbVote = voteTrans.GetProtoVote()
	txFee = core.GetTxFee(pbVote)
	assert.NotEqual(t, 0, txFee)

	voteTxs = []*pb.Vote{pbVote}

	block = core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, []*pb.Transaction{}, voteTxs, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0)
	require.NotNil(t, block)

	err = WM.CheckAndApplyVotes(block, "testMiner")
	assert.Nil(t, err)

	newBal -= txFee
	VP1, _, err := WM.GetCandVotePower("cand1")
	trueVp = int64(0.3 * float64(newBal))
	VP, _, err = WM.GetCandVotePower("cand2")
	fmt.Println("txFee", txFee, " bal: ", newBal, "True VP:", trueVp, "got: ", VP, "c1: ", VP1)
	WM.PrintWalletBalances()
	assert.Equal(t, trueVp, VP)
	assert.Equal(t, 50, WM.MyWallet.GetVoteLeft())

	//Should add votepower to the candidate 3 successfully
	vote4 := "+50|cand3"

	votes = []string{vote4}
	voteTrans, _ = core.NewVote(voterAdd, keys.PrivateKey, votes)
	pbVote = voteTrans.GetProtoVote()
	txFee = core.GetTxFee(pbVote)
	// assert.Equal(t, int64(27), txFee)
	voteTxs = []*pb.Vote{pbVote}

	block = core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, []*pb.Transaction{}, voteTxs, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0)
	require.NotNil(t, block)

	err = WM.CheckAndApplyVotes(block, "testMiner")
	assert.Nil(t, err)

	VP, _, err = WM.GetCandVotePower("cand2")
	assert.NotEqual(t, trueVp, VP) //Candidate shouldn't have the same votepower
	fmt.Println("Weird bal; ", newBal)
	newBal -= txFee
	trueVp = int64(0.5 * float64(newBal))
	VP, _, err = WM.GetCandVotePower("cand3")
	fmt.Println("txFee", txFee, " bal: ", newBal, "True VP:", trueVp, "got: ", VP)
	WM.PrintWalletBalances()

	assert.Equal(t, trueVp, VP)
	assert.Equal(t, 0, WM.MyWallet.GetVoteLeft())

	//This vote shouldn't execute because user has already staked 100 percent of his vote balance
	vote5 := "+10|cand1"

	votes = []string{vote5}
	voteTrans, _ = core.NewVote(voterAdd, keys.PrivateKey, votes)
	pbVote = voteTrans.GetProtoVote()
	// var voteTxs []*pb.Vote
	voteTxs = []*pb.Vote{pbVote}

	block = core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, []*pb.Transaction{}, voteTxs, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0)
	require.NotNil(t, block)

	err = WM.CheckAndApplyVotes(block, "testMiner")
	assert.Error(t, err)
	WM.PrintWalletBalances()

	//This vote shouldn't execute because user has already staked 100 percent of his vote balance
	vote6 := "+1|cand1"

	votes = []string{vote6}
	voteTrans, _ = core.NewVote(voterAdd, keys.PrivateKey, votes)
	pbVote = voteTrans.GetProtoVote()
	// var voteTxs []*pb.Vote
	voteTxs = []*pb.Vote{pbVote}

	block = core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, []*pb.Transaction{}, voteTxs, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0)
	require.NotNil(t, block)

	err = WM.CheckAndApplyVotes(block, "testMiner")
	assert.Error(t, err)
	WM.PrintWalletBalances()
	//Doing multiple votes

	vote7 := "-50|cand3"
	vote8 := "+30|cand2"
	vote9 := "+20|cand1"
	votes = []string{vote7, vote8, vote9}
	voteTrans, _ = core.NewVote(voterAdd, keys.PrivateKey, votes)
	vBytes, _ = voteTrans.ToBytes()
	fmt.Println("Triple vote bytes are: ", len(vBytes))
	pbVote = voteTrans.GetProtoVote()
	txFee = core.GetTxFee(pbVote)
	fmt.Println("triple tx: ", txFee)
	assert.NotEqual(t, 0, txFee)
	voteTxs = []*pb.Vote{pbVote}

	block = core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, []*pb.Transaction{}, voteTxs, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0)
	require.NotNil(t, block)

	err = WM.CheckAndApplyVotes(block, "testMiner")
	assert.Nil(t, err)

	newBal -= txFee
	VP, _, err = WM.GetCandVotePower("cand3")
	assert.Equal(t, int64(0), VP)
	fmt.Println("Cand3 VP: ", VP)
	assert.Equal(t, 0, WM.MyWallet.GetVoteLeft())

	trueVp = int64(0.6 * float64(newBal))
	VP, _, err = WM.GetCandVotePower("cand2")
	assert.Equal(t, trueVp, VP)
	per, err := WM.MyWallet.GetCandVotePercentage("cand2")
	assert.Nil(t, err)
	assert.Equal(t, 60, per)
	assert.Equal(t, 0, WM.MyWallet.GetVoteLeft())

	trueVp = int64(0.4 * float64(newBal))
	// trueVp++ //Because in process of reassigning vote power rounding introduced error
	VP, _, err = WM.GetCandVotePower("cand1")
	fmt.Println("cand-1 true vp:-", trueVp, " got", VP)
	assert.Equal(t, trueVp, VP)
	assert.Equal(t, 0, WM.MyWallet.GetVoteLeft())
	WM.PrintWalletBalances()

	//THis should also be accepted. Testing multiple voting transaction in the block. Some votes are repeated.
	vote10 := "-30|cand2"
	vote11 := "-30|cand2"
	vote12 := "-30|cand1"
	votes = []string{vote10, vote11, vote12}
	voteTrans, _ = core.NewVote(voterAdd, keys.PrivateKey, votes)
	pbVote = voteTrans.GetProtoVote()
	txFee = core.GetTxFee(pbVote)

	voteTxs = []*pb.Vote{pbVote}

	block = core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, []*pb.Transaction{}, voteTxs, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0)
	require.NotNil(t, block)

	err = WM.CheckAndApplyVotes(block, "testMiner")
	assert.Nil(t, err)

	newBal -= txFee
	VP, _, err = WM.GetCandVotePower("cand2")
	fmt.Println("cand2 vp: ", VP)
	assert.Equal(t, int64(0), VP) //Due to error in the previous
	assert.Equal(t, 90, WM.MyWallet.GetVoteLeft())

	trueVp = int64(0.1 * float64(newBal))
	// trueVp += 2
	VP, _, err = WM.GetCandVotePower("cand1")
	assert.Equal(t, trueVp, VP)
	assert.Equal(t, 90, WM.MyWallet.GetVoteLeft())
	fmt.Println("txFee", txFee, " bal: ", newBal, "True VP:", trueVp, "got: ", VP)
	WM.PrintWalletBalances()
	// Finally completely unvote
	vote13 := "-10|cand1"
	votes = []string{vote13}
	voteTrans, _ = core.NewVote(voterAdd, keys.PrivateKey, votes)
	pbVote = voteTrans.GetProtoVote()
	txFee = core.GetTxFee(pbVote)
	voteTxs = []*pb.Vote{pbVote}

	block = core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, []*pb.Transaction{}, voteTxs, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0)
	require.NotNil(t, block)

	err = WM.CheckAndApplyVotes(block, "testMiner")
	assert.Nil(t, err)
	newBal -= txFee
	VP, _, err = WM.GetCandVotePower("cand1")
	fmt.Println(VP)
	assert.Equal(t, int64(0), VP) //Rounding discrepency
	assert.Equal(t, 100, WM.MyWallet.GetVoteLeft())
	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// //Testing the vote revert in case any transaction in the block in invalid

	//Voting back
	vote14 := "+50|cand3"
	vote15 := "+30|cand2"
	vote16 := "+20|cand1"
	votes = []string{vote14, vote15, vote16}
	voteTrans, _ = core.NewVote(voterAdd, keys.PrivateKey, votes)
	pbVote = voteTrans.GetProtoVote()
	txFee = core.GetTxFee(pbVote)
	// var voteTxs []*pb.Vote
	voteTxs = []*pb.Vote{pbVote}

	block = core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, []*pb.Transaction{}, voteTxs, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0)
	require.NotNil(t, block)

	err = WM.CheckAndApplyVotes(block, "testMiner")
	assert.Nil(t, err)

	newBal -= txFee
	// wal, _ := WM.GetWalletByAddress("cand3")
	// bal := wal.GetBalance()
	trueVp = int64(0.5 * float64(newBal))
	VP, _, err = WM.GetCandVotePower("cand3")
	fmt.Println("txFee", txFee, " bal: ", newBal, "True VP:", trueVp, "got: ", VP)
	assert.Equal(t, trueVp, VP)
	assert.Equal(t, 0, WM.MyWallet.GetVoteLeft())
	WM.PrintWalletBalances()

	trueVp = int64(0.3 * float64(newBal))
	VP, _, err = WM.GetCandVotePower("cand2")
	assert.Equal(t, trueVp, VP)
	per, err = WM.MyWallet.GetCandVotePercentage("cand2")
	assert.Nil(t, err)
	assert.Equal(t, 30, per)
	assert.Equal(t, 0, WM.MyWallet.GetVoteLeft())

	trueVp = int64(0.2 * float64(newBal))
	VP, _, err = WM.GetCandVotePower("cand1")
	assert.Equal(t, trueVp, VP)
	assert.Equal(t, 0, WM.MyWallet.GetVoteLeft())

	//Lets unvote with an invalid transaction in the votes. -> These votes shouldn't be applied. Vote revert should be successful
	vote17 := "-50|cand3"
	vote18 := "-30|cand2" //Invalid unvote
	vote19 := "-40|cand1"
	votes = []string{vote17, vote18, vote19}
	voteTrans, _ = core.NewVote(voterAdd, keys.PrivateKey, votes)
	pbVote = voteTrans.GetProtoVote()
	txFee = core.GetTxFee(pbVote)
	voteTxs = []*pb.Vote{pbVote}

	block = core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, []*pb.Transaction{}, voteTxs, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0)
	require.NotNil(t, block)

	err = WM.CheckAndApplyVotes(block, "testMiner")
	assert.Error(t, err)
	// newBal -= txFee   //Since the transaction is undone
	trueVp = int64(0.5 * float64(newBal))
	VP, _, err = WM.GetCandVotePower("cand3")
	assert.Equal(t, trueVp, VP)
	assert.Equal(t, 0, WM.MyWallet.GetVoteLeft())

	trueVp = int64(0.3 * float64(newBal))
	VP, _, err = WM.GetCandVotePower("cand2")
	assert.Equal(t, trueVp, VP)
	per, err = WM.MyWallet.GetCandVotePercentage("cand2")
	assert.Nil(t, err)
	assert.Equal(t, 30, per)
	assert.Equal(t, 0, WM.MyWallet.GetVoteLeft())

	trueVp = int64(0.2 * float64(newBal))
	VP, _, err = WM.GetCandVotePower("cand1")
	assert.Equal(t, trueVp, VP)
	assert.Equal(t, 0, WM.MyWallet.GetVoteLeft())
	WM.PrintWalletBalances()
	//Create a retrieval transaction and vote percentage should change accordingly

	testTrans, err := core.MakeTestTransaction([]byte(voterAdd), []byte("cand1"), int64(5000), 111, keys.PrivateKey)
	assert.Nil(t, err)
	// WM.CheckAndUpdateWalletStates()
	protoTrans := testTrans.GetProtoTransaction()
	txFee = core.GetTxFee(protoTrans)
	retTxs := []*pb.Transaction{protoTrans}
	block = core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, retTxs, []*pb.Vote{}, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0)
	require.NotNil(t, block)
	err = WM.CheckAndUpdateWalletStates(block, "TestMiner")
	assert.Nil(t, err)
	newBal -= (5000 + txFee)
	fmt.Println("balance : ", WM.MyWallet.GetBalance(), "should be: ", newBal)
	assert.Equal(t, newBal, WM.MyWallet.GetBalance())

	trueVp = int64(0.5 * float64(newBal))
	VP, _, err = WM.GetCandVotePower("cand3")
	assert.Equal(t, trueVp, VP)
	assert.Equal(t, 0, WM.MyWallet.GetVoteLeft())

	trueVp = int64(0.3 * float64(newBal))
	VP, _, err = WM.GetCandVotePower("cand2")
	assert.Equal(t, trueVp, VP)
	per, err = WM.MyWallet.GetCandVotePercentage("cand2")
	assert.Nil(t, err)
	assert.Equal(t, 30, per)
	assert.Equal(t, 0, WM.MyWallet.GetVoteLeft())

	trueVp = int64(0.2 * float64(newBal))
	VP, _, err = WM.GetCandVotePower("cand1")
	assert.Equal(t, trueVp, VP)
	assert.Equal(t, 0, WM.MyWallet.GetVoteLeft())
}

func TestWalletManager_ApplyTransactions(t *testing.T) {

	keys := new(Keypair)
	var err error

	keys.PublicKey, keys.PrivateKey, err = noise.GenerateKeys(nil)
	tPayer := "TestPayer"
	WM := NewWalletManager(tPayer) //Payer for thr transfer transaction
	mWallet := WM.MyWallet
	mWallet.SetBalance(10000)
	require.NotNil(t, WM)

	//Creating test user wallets -> user-1
	u1 := NewWallet("u1")
	require.NotNil(t, u1)
	err = WM.CheckAndPutWalletByAddress("u1", u1)
	u1, err = WM.GetWalletByAddress("u1")
	u1.SetBalance(10000)
	assert.Equal(t, u1.GetBalance(), uint64(10000))

	assert.Nil(t, err)

	//Creating test user wallets -> user-2
	u2 := NewWallet("u2")
	require.NotNil(t, u2)
	err = WM.CheckAndPutWalletByAddress("u2", u2)
	u2, err = WM.GetWalletByAddress("u2")
	u2.SetBalance(10000)
	assert.Equal(t, u2.GetBalance(), uint64(10000))

	assert.Nil(t, err)

	//Creating test user wallets -> user-3
	u3 := NewWallet("u3")
	require.NotNil(t, u3)
	err = WM.CheckAndPutWalletByAddress("u3", u3)
	u3, err = WM.GetWalletByAddress("u3")
	u3.SetBalance(10000)
	assert.Equal(t, u3.GetBalance(), uint64(10000))
	assert.Nil(t, err)

	var retTxs []*pb.Transaction
	var txAmount int64 = 1000
	//User 1 is transfering 1000 to the user 2
	t1, err := core.MakeTestTransaction([]byte("u1"), []byte("u2"), txAmount, 0, keys.PrivateKey)
	assert.NoError(t, err)
	assert.NotNil(t, t1)
	pbTrans := t1.GetProtoTransaction()
	assert.NotNil(t, pbTrans)
	retTxs = append(retTxs, pbTrans)

	txFee := core.GetTxFee(pbTrans)
	assert.NotEqual(t, 0, txFee)
	// fmt.Println("Tx fee: ", txFee)
	_, err = WM.ApplyTransactions(retTxs, mWallet, 100)
	assert.Nil(t, err)
	fmt.Println("u1 balance: ", u1.GetBalance())
	u1Balance := uint64(10000 - txAmount - txFee)
	u2Balance := uint64(10000 + txAmount)
	mBalance := int64(10000 + txFee)
	assert.Equal(t, u1Balance, u1.GetBalance())
	assert.Equal(t, u2Balance, u2.GetBalance())
	assert.Equal(t, mBalance, mWallet.GetBalance())
	WM.PrintWalletBalances()
	//User 1 is transfering 1000 to the user 2
	t2, err := core.MakeTestTransaction([]byte(tPayer), []byte("u3"), txAmount, 0, keys.PrivateKey)
	assert.NoError(t, err)
	assert.NotNil(t, t2)
	pbTrans = t2.GetProtoTransaction()
	assert.NotNil(t, pbTrans)
	retTxs = []*pb.Transaction{pbTrans}

	txFee = core.GetTxFee(pbTrans)
	assert.NotEqual(t, 0, txFee)
	_, err = WM.ApplyTransactions(retTxs, mWallet, 100)
	assert.Nil(t, err)
	u3Balance := uint64(10000 + txAmount)
	mBalance = mBalance - txAmount - txFee + txFee

	assert.Equal(t, mBalance, mWallet.GetBalance())
	assert.Equal(t, u3Balance, u3.GetBalance())
	WM.PrintWalletBalances()
}

func TestWalletManager_ValidateBlockTx(t *testing.T) {
	type fields struct {
		walletByAddress  map[string]*Wallet
		walletByUsername map[string]*Wallet
		MyWallet         *Wallet
		Bc               blockChain
		Executor         TxExecutor
	}
	type args struct {
		block *core.Block
	}
	//--------------------------------------------------------------//

	keys := new(Keypair)
	var err error

	keys.PublicKey, keys.PrivateKey, err = noise.GenerateKeys(nil)

	voterAdd := "TestVoter"
	WM := NewWalletManager(voterAdd)
	require.NotNil(t, WM)
	testVoter := WM.MyWallet
	testVoter.SetBalance(10000)

	//Creating test candidate wallets
	cand1 := NewWallet("cand1")
	require.NotNil(t, cand1)
	cand1.SetBalance(10000)
	assert.Equal(t, cand1.GetBalance(), uint64(10000))
	err = WM.CheckAndPutWalletByAddress("cand1", cand1)
	// cand1, _ = WM.GetWalletByAddress("cand1")

	assert.Nil(t, err)
	VP, _, err := WM.GetCandVotePower("cand1")
	assert.Equal(t, uint64(0), VP)

	cand2 := NewWallet("cand2")
	require.NotNil(t, cand2)
	cand2.SetBalance(10000)
	assert.Equal(t, cand2.GetBalance(), uint64(10000))
	err = WM.CheckAndPutWalletByAddress("cand2", cand2)
	assert.Nil(t, err)
	VP, _, err = WM.GetCandVotePower("cand2")
	assert.Equal(t, uint64(0), VP)

	testMiner := NewWallet("testMiner")
	require.NotNil(t, testMiner)
	testMiner.SetBalance(10000)
	assert.Equal(t, testMiner.GetBalance(), uint64(10000))
	err = WM.CheckAndPutWalletByAddress("testMiner", testMiner)
	assert.Nil(t, err)
	// testMiner, _ = WM.GetWalletByAddress("testMiner")
	VP, _, err = WM.GetCandVotePower("testMiner")
	assert.Equal(t, uint64(0), VP)
	//Vote transaction

	vote1 := "+20|cand1"
	var votes []string
	votes = append(votes, vote1)
	voteTrans, _ := core.NewVote(voterAdd, keys.PrivateKey, votes)
	// vBytes, _ := voteTrans.ToBytes()
	// fmt.Println("Single vote bytes are: ", len(vBytes))
	pbVote := voteTrans.GetProtoVote()
	txFee := core.GetTxFee(pbVote)
	assert.NotEqual(t, 0, txFee)
	var voteTxs1 []*pb.Vote
	voteTxs1 = append(voteTxs1, pbVote)

	vote2 := "-30|cand2"
	votes = []string{vote2}
	voteTrans, _ = core.NewVote(voterAdd, keys.PrivateKey, votes)
	pbVote = voteTrans.GetProtoVote()
	voteTxs2 := []*pb.Vote{pbVote}

	vote10 := "-30|cand2"
	vote11 := "-30|cand2"
	vote12 := "-30|cand1"
	votes = []string{vote10, vote11, vote12}
	voteTrans, _ = core.NewVote(voterAdd, keys.PrivateKey, votes)
	pbVote = voteTrans.GetProtoVote()
	txFee = core.GetTxFee(pbVote)

	voteTxs3 := []*pb.Vote{pbVote}

	vote13 := "30|cand1"
	votes = []string{vote13}
	voteTrans, _ = core.NewVote(voterAdd, keys.PrivateKey, votes)
	pbVote = voteTrans.GetProtoVote()
	txFee = core.GetTxFee(pbVote)

	voteTxs4 := []*pb.Vote{pbVote}
	//----------------------------------------------------------------------------------//
	//Now testing transaction
	//Creating test user wallets -> user-1
	u1 := NewWallet("u1")
	require.NotNil(t, u1)
	err = WM.CheckAndPutWalletByAddress("u1", u1)
	u1, err = WM.GetWalletByAddress("u1")
	u1.SetBalance(10000)
	assert.Equal(t, u1.GetBalance(), uint64(10000))
	assert.Nil(t, err)

	//Creating test user wallets -> user-2
	u2 := NewWallet("u2")
	require.NotNil(t, u2)
	err = WM.CheckAndPutWalletByAddress("u2", u2)
	u2, err = WM.GetWalletByAddress("u2")
	u2.SetBalance(10000)
	assert.Equal(t, u2.GetBalance(), uint64(10000))
	assert.Nil(t, err)

	//First transaction //
	var txAmount int64 = 1000
	//User 1 is transfering 1000 to the user 2
	t1, err := core.MakeTestTransaction([]byte("u1"), []byte("u2"), txAmount, 0, keys.PrivateKey)
	assert.NoError(t, err)
	assert.NotNil(t, t1)
	pbTrans := t1.GetProtoTransaction()
	assert.NotNil(t, pbTrans)
	retTxs := []*pb.Transaction{pbTrans}

	//Second invalid transaction
	txAmount = 200000
	t2, err := core.MakeTestTransaction([]byte("u1"), []byte("u2"), txAmount, 0, keys.PrivateKey)
	assert.NoError(t, err)
	assert.NotNil(t, t2)
	pbTrans = t2.GetProtoTransaction()
	assert.NotNil(t, pbTrans)
	retTxs2 := []*pb.Transaction{pbTrans}

	//Third tranaction where one is valid and the other one is also valid
	txAmount = 2000
	t3, err := core.MakeTestTransaction([]byte("u1"), []byte("u2"), txAmount, 0, keys.PrivateKey)
	assert.NoError(t, err)
	assert.NotNil(t, t3)
	pbTrans = t3.GetProtoTransaction()
	assert.NotNil(t, pbTrans)
	retTxs3 := []*pb.Transaction{t1.GetProtoTransaction(), pbTrans}

	//Fourth transaction where one is valid and the other is not
	retTxs4 := []*pb.Transaction{t1.GetProtoTransaction(), t2.GetProtoTransaction()}

	//------------------------------------------------------------------------------------//
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bool
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "Testing correct vote validation",
			fields: fields{
				walletByAddress: WM.walletByAddress,
				MyWallet:        WM.MyWallet,
				Bc:              nil,
				Executor:        nil,
			},
			args: args{
				block: core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, []*pb.Transaction{}, voteTxs1, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0),
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Testing incorrect vote validation",
			fields: fields{
				walletByAddress: WM.walletByAddress,
				MyWallet:        WM.MyWallet,
				Bc:              nil,
				Executor:        nil,
			},
			args: args{
				block: core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, []*pb.Transaction{}, voteTxs2, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0),
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "Testing incorrect vote validation",
			fields: fields{
				walletByAddress: WM.walletByAddress,
				MyWallet:        WM.MyWallet,
				Bc:              nil,
				Executor:        nil,
			},
			args: args{
				block: core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, []*pb.Transaction{}, voteTxs3, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0),
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "Testing incorrect vote structure validation",
			fields: fields{
				walletByAddress: WM.walletByAddress,
				MyWallet:        WM.MyWallet,
				Bc:              nil,
				Executor:        nil,
			},
			args: args{
				block: core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, []*pb.Transaction{}, voteTxs4, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0),
			},
			want:    false,
			wantErr: true,
		},

		{
			name: "Testing correct ret-Tx structure validation",
			fields: fields{
				walletByAddress: WM.walletByAddress,

				MyWallet: WM.MyWallet,
				Bc:       nil,
				Executor: nil,
			},
			args: args{
				block: core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, retTxs, []*pb.Vote{}, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0),
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Testing incorrect(exceeding balance) ret-Tx validation",
			fields: fields{
				walletByAddress: WM.walletByAddress,

				MyWallet: WM.MyWallet,
				Bc:       nil,
				Executor: nil,
			},
			args: args{
				block: core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, retTxs2, []*pb.Vote{}, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0),
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "Testing multiple correct ret-Tx validation",
			fields: fields{
				walletByAddress: WM.walletByAddress,

				MyWallet: WM.MyWallet,
				Bc:       nil,
				Executor: nil,
			},
			args: args{
				block: core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, retTxs3, []*pb.Vote{}, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0),
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Testing multiple ret-Tx(one correct and one false) validation",
			fields: fields{
				walletByAddress: WM.walletByAddress,

				MyWallet: WM.MyWallet,
				Bc:       nil,
				Executor: nil,
			},
			args: args{
				block: core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, retTxs4, []*pb.Vote{}, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0),
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wm := &WalletManager{
				walletByAddress: tt.fields.walletByAddress,
				MyWallet:        tt.fields.MyWallet,
				Executor:        tt.fields.Executor,
			}
			got, err := wm.ValidateBlockTx(tt.args.block)
			if (err != nil) != tt.wantErr {
				t.Errorf("WalletManager.ValidateBlockTx() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("WalletManager.ValidateBlockTx() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWalletManager_RevertWalletStates(t *testing.T) {

	keys := new(Keypair)
	var err error

	keys.PublicKey, keys.PrivateKey, err = noise.GenerateKeys(nil)

	voterAdd := "TestVoter"
	WM := NewWalletManager(voterAdd)
	// mWallet := WM.MyWallet

	require.NotNil(t, WM)
	testVoter := WM.MyWallet
	testVoter.SetBalance(10000)
	assert.Equal(t, testVoter.GetBalance(), uint64(10000))

	//Creating test candidate wallets
	cand1 := NewWallet("cand1")
	require.NotNil(t, cand1)
	cand1.SetBalance(10000)
	assert.Equal(t, cand1.GetBalance(), uint64(10000))
	err = WM.CheckAndPutWalletByAddress("cand1", cand1)
	// cand1, _ = WM.GetWalletByAddress("cand1")

	assert.Nil(t, err)
	VP, _, err := WM.GetCandVotePower("cand1")
	assert.Equal(t, uint64(0), VP)

	cand2 := NewWallet("cand2")
	require.NotNil(t, cand2)
	cand2.SetBalance(10000)
	assert.Equal(t, cand2.GetBalance(), uint64(10000))
	err = WM.CheckAndPutWalletByAddress("cand2", cand2)
	assert.Nil(t, err)
	// cand2, _ = WM.GetWalletByAddress("cand2")

	VP, _, err = WM.GetCandVotePower("cand2")
	assert.Equal(t, uint64(0), VP)

	testMiner := NewWallet("testMiner")
	require.NotNil(t, testMiner)
	testMiner.SetBalance(10000)
	assert.Equal(t, testMiner.GetBalance(), uint64(10000))
	err = WM.CheckAndPutWalletByAddress("testMiner", testMiner)
	assert.Nil(t, err)
	// testMiner, _ = WM.GetWalletByAddress("testMiner")
	VP, _, err = WM.GetCandVotePower("testMiner")
	assert.Equal(t, uint64(0), VP)
	//Vote1 candidate1 20percent of the power voter owned
	vote1 := "+20|cand1"

	var votes []string
	votes = append(votes, vote1)
	voteTrans, _ := core.NewVote(voterAdd, keys.PrivateKey, votes)
	pbVote := voteTrans.GetProtoVote()
	txFee := core.GetTxFee(pbVote)
	assert.NotEqual(t, 0, txFee)
	var voteTxs []*pb.Vote
	voteTxs = append(voteTxs, pbVote)

	block := core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, []*pb.Transaction{}, voteTxs, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0)
	require.NotNil(t, block)

	err = WM.CheckAndApplyVotes(block, "testMiner")
	assert.Nil(t, err)
	newBal := 10000 - txFee
	trueVp := uint64(0.2 * float64(newBal)) //Vote should be applied after the transaction fee has been paid
	VP, _, err = WM.GetCandVotePower("cand1")
	fmt.Println("txFee", txFee, " bal: ", newBal, "True VP:", trueVp, "got: ", VP)
	assert.Equal(t, trueVp, VP)
	assert.Equal(t, 80, WM.MyWallet.GetVoteLeft())
	assert.Equal(t, int64(10000)+txFee, testMiner.GetBalance()) //Miner should be able to earn transaciton fee
	//Reverting vote-1
	WM.RevertVotes(block.GetVotes(), "testMiner", block.GetVotes(), 100)
	VP, _, err = WM.GetCandVotePower("cand1")
	assert.Equal(t, uint64(0), VP)
	assert.Equal(t, 100, WM.MyWallet.GetVoteLeft())
	assert.Equal(t, uint64(10000), testMiner.GetBalance()) //Miner should be able to earn transaciton fee

	//Second vote
	vote8 := "+30|cand2"
	vote9 := "+20|cand1"
	votes = []string{vote8, vote9}
	voteTrans, _ = core.NewVote(voterAdd, keys.PrivateKey, votes)
	pbVote = voteTrans.GetProtoVote()
	txFee = core.GetTxFee(pbVote)
	assert.NotEqual(t, 0, txFee)
	voteTxs = []*pb.Vote{pbVote}

	block = core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, []*pb.Transaction{}, voteTxs, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0)
	require.NotNil(t, block)

	err = WM.CheckAndApplyVotes(block, "testMiner")
	assert.Nil(t, err)

	newBal = 10000 - txFee

	trueVp = uint64(0.3 * float64(newBal))
	VP, _, err = WM.GetCandVotePower("cand2")
	assert.Equal(t, trueVp, VP)
	per, err := WM.MyWallet.GetCandVotePercentage("cand2")
	assert.Nil(t, err)
	assert.Equal(t, 30, per)
	assert.Equal(t, 50, WM.MyWallet.GetVoteLeft())

	trueVp = uint64(0.2 * float64(newBal))
	// trueVp++ //Because in process of reassigning vote power rounding introduced error
	VP, _, err = WM.GetCandVotePower("cand1")
	fmt.Println("cand-1 true vp:-", trueVp, " got", VP)
	assert.Equal(t, trueVp, VP)
	assert.Equal(t, 50, WM.MyWallet.GetVoteLeft())
	//Reverting the second vote
	WM.RevertVotes(block.GetVotes(), "testMiner", block.GetVotes(), 100)
	VP, _, err = WM.GetCandVotePower("cand1")
	assert.Equal(t, uint64(0), VP)
	assert.Equal(t, 100, WM.MyWallet.GetVoteLeft())
	assert.Equal(t, uint64(10000), testMiner.GetBalance()) //Miner should be able to earn transaciton fee
	VP, _, err = WM.GetCandVotePower("cand2")
	assert.Equal(t, uint64(0), VP)
	////////////////////////////////////////////////---------------------/////////////////////////////

	u1 := NewWallet("u1")
	require.NotNil(t, u1)
	err = WM.CheckAndPutWalletByAddress("u1", u1)
	u1, err = WM.GetWalletByAddress("u1")
	u1.SetBalance(10000)
	assert.Equal(t, u1.GetBalance(), uint64(10000))
	assert.Nil(t, err)

	//Creating test user wallets -> user-2
	u2 := NewWallet("u2")
	require.NotNil(t, u2)
	err = WM.CheckAndPutWalletByAddress("u2", u2)
	u2, err = WM.GetWalletByAddress("u2")
	u2.SetBalance(10000)
	assert.Equal(t, u2.GetBalance(), uint64(10000))
	assert.Nil(t, err)

	//First transaction //
	var txAmount int64 = 1000
	//User 1 is transfering 1000 to the user 2
	t1, err := core.MakeTestTransaction([]byte("u1"), []byte("u2"), txAmount, 0, keys.PrivateKey)
	assert.NoError(t, err)
	assert.NotNil(t, t1)
	pbTrans := t1.GetProtoTransaction()
	assert.NotNil(t, pbTrans)
	retTxs := []*pb.Transaction{pbTrans}
	block = core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, retTxs, []*pb.Vote{}, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0)
	require.NotNil(t, block)

	err = WM.CheckAndUpdateWalletStates(block, "testMiner")

	require.Nil(t, err)
	//Asserting if the
	txFee1 := core.GetTxFee(pbTrans)
	assert.NotEqual(t, 0, txFee1)
	// fmt.Println("Tx fee: ", txFee1)

	// fmt.Println("u1 balance: ", u1.GetBalance())
	u1Balance := uint64(10000 - txAmount - txFee1)
	u2Balance := uint64(10000 + txAmount)
	mBalance := uint64(10000 + txFee1 + WM.RewardForBlockCreation())
	assert.Equal(t, u1Balance, u1.GetBalance())
	assert.Equal(t, u2Balance, u2.GetBalance())
	assert.Equal(t, mBalance, testMiner.GetBalance())

	//Reverting first transaction
	err = WM.RevertWalletStates(block, "testMiner")
	require.Nil(t, err)

	fmt.Println("u2 balance: ", u2.GetBalance())
	u1Balance = uint64(10000)
	u2Balance = uint64(10000)
	mBalance = uint64(10000)
	assert.Equal(t, u1Balance, u1.GetBalance())
	assert.Equal(t, u2Balance, u2.GetBalance())
	assert.Equal(t, mBalance, testMiner.GetBalance())

	//////////////////////////////////////////////////////////////////////////////
	//Second valid transaction
	txAmount = 7000
	//User 1 is transfering 1000 to the user 2
	t2, err := core.MakeTestTransaction([]byte("u1"), []byte("u2"), txAmount, 0, keys.PrivateKey)
	assert.NoError(t, err)
	assert.NotNil(t, t2)
	pbTrans = t2.GetProtoTransaction()
	assert.NotNil(t, pbTrans)
	retTxs = []*pb.Transaction{pbTrans}
	block = core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, retTxs, []*pb.Vote{}, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0)
	require.NotNil(t, block)

	err = WM.CheckAndUpdateWalletStates(block, "testMiner")

	require.Nil(t, err)
	//Asserting if the
	txFee2 := core.GetTxFee(pbTrans)
	assert.NotEqual(t, 0, txFee2)
	// fmt.Println("Tx fee: ", txFee2)

	// fmt.Println("u1 balance: ", u1.GetBalance())
	u1Balance = uint64(10000 - txAmount - txFee2)
	u2Balance = uint64(10000 + txAmount)
	mBalance = uint64(10000 + txFee2 + WM.RewardForBlockCreation())
	assert.Equal(t, u1Balance, u1.GetBalance())
	assert.Equal(t, u2Balance, u2.GetBalance())
	assert.Equal(t, mBalance, testMiner.GetBalance())

	//Reverting second transaction
	err = WM.RevertWalletStates(block, "testMiner")
	require.Nil(t, err)

	fmt.Println("u2 balance: ", u2.GetBalance())
	u1Balance = uint64(10000)
	u2Balance = uint64(10000)
	mBalance = uint64(10000)
	assert.Equal(t, u1Balance, u1.GetBalance())
	assert.Equal(t, u2Balance, u2.GetBalance())
	assert.Equal(t, mBalance, testMiner.GetBalance())

	//3rd transaction -> do multiple transaction
	txAmount = 7000 + 1000
	retTxs = []*pb.Transaction{t1.GetProtoTransaction(), pbTrans}
	block = core.CreateTestBlock(nil, keys.PublicKey, keys.PrivateKey, retTxs, []*pb.Vote{}, []*pb.Deal{}, []*pb.ChPrPair{}, 0, 0)
	require.NotNil(t, block)

	err = WM.CheckAndUpdateWalletStates(block, "testMiner")

	require.Nil(t, err)
	//Asserting if the
	txFee = txFee1 + txFee2
	assert.NotEqual(t, 0, txFee)

	// fmt.Println("u1 balance: ", u1.GetBalance())
	u1Balance = uint64(10000 - txAmount - txFee)
	u2Balance = uint64(10000 + txAmount)
	mBalance = uint64(10000 + txFee + WM.RewardForBlockCreation())
	assert.Equal(t, u1Balance, u1.GetBalance())
	assert.Equal(t, u2Balance, u2.GetBalance())
	assert.Equal(t, mBalance, testMiner.GetBalance())

	//Reverting transactions in third step
	err = WM.RevertWalletStates(block, "testMiner")
	require.Nil(t, err)

	// fmt.Println("u2 balance: ", u2.GetBalance())
	u1Balance = uint64(10000)
	u2Balance = uint64(10000)
	mBalance = uint64(10000)
	assert.Equal(t, u1Balance, u1.GetBalance())
	assert.Equal(t, u2Balance, u2.GetBalance())
	assert.Equal(t, mBalance, testMiner.GetBalance())
}
