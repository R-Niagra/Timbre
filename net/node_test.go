package net

import (
	"fmt"
	"testing"

	"github.com/guyu96/go-timbre/core"
	pb "github.com/guyu96/go-timbre/protobuf"
	"github.com/guyu96/go-timbre/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDelegateRegAndResig(t *testing.T) {
	tNode, err := NewTestNode(8000, 2, false)
	nodeID := tNode.GetNodeID()
	require.NotNil(t, tNode)
	assert.NoError(t, err)

	u1 := wallet.NewWallet("u1", []byte{}, nil)
	require.NotNil(t, u1)
	err = tNode.Wm.PutWalletByAddress("u1", u1)
	assert.Nil(t, err)
	assert.Equal(t, u1.GetBalance(), uint64(10000))

	u2 := wallet.NewWallet("u2", []byte{}, nil)
	require.NotNil(t, u2)
	err = tNode.Wm.PutWalletByAddress("u2", u2)
	assert.Nil(t, err)
	assert.Equal(t, u2.GetBalance(), uint64(10000))
	//Registering the testnode as a delegate
	t1, err := core.RegisterCandidateTx([]byte(nodeID.Address()), tNode.PrivateKey(), 0)
	assert.NoError(t, err)
	assert.NotNil(t, t1)
	pbTrans := t1.GetProtoTransaction()
	assert.NotNil(t, pbTrans)
	regTxs := []*pb.Transaction{pbTrans}
	txFee := core.GetTxFee(pbTrans)

	err = tNode.Wm.ApplyTransactions(regTxs, u1)
	assert.Nil(t, err)
	myBalance := 10000 - txFee
	assert.Equal(t, myBalance, tNode.Wm.MyWallet.GetBalance())
	assert.True(t, tNode.Dpos.State.CheckCandByAdd("127.0.0.1:8000"))

	//Resigning this testnode as candidate -. A legitimate transaction
	t2, err := core.QuitCandidateTx([]byte(nodeID.Address()), tNode.PrivateKey(), 0)
	assert.NoError(t, err)
	assert.NotNil(t, t2)
	pbTrans = t2.GetProtoTransaction()
	assert.NotNil(t, pbTrans)
	regTxs = []*pb.Transaction{pbTrans}
	txFee = core.GetTxFee(pbTrans)

	err = tNode.Wm.ApplyTransactions(regTxs, u1)
	assert.Nil(t, err)
	myBalance = myBalance - txFee
	assert.Equal(t, myBalance, tNode.Wm.MyWallet.GetBalance())
	assert.False(t, tNode.Dpos.State.CheckCandByAdd("127.0.0.1:8000"))

	//Resigning once again. Illegitimate transaction
	t3, err := core.QuitCandidateTx([]byte(nodeID.Address()), tNode.PrivateKey(), 0)
	assert.NoError(t, err)
	assert.NotNil(t, t3)
	pbTrans = t3.GetProtoTransaction()
	assert.NotNil(t, pbTrans)
	regTxs = []*pb.Transaction{pbTrans}
	txFee = core.GetTxFee(pbTrans)

	err = tNode.Wm.ApplyTransactions(regTxs, u1)
	assert.Error(t, err) //Should give an error. unregistering again
	assert.Equal(t, myBalance, tNode.Wm.MyWallet.GetBalance())

	//Registering the testnode as a delegate once again -> Legitimate transaction- user can register again given that they have balance after resigning
	t4, err := core.RegisterCandidateTx([]byte(nodeID.Address()), tNode.PrivateKey(), 0)
	assert.NoError(t, err)
	assert.NotNil(t, t4)
	pbTrans = t4.GetProtoTransaction()
	assert.NotNil(t, pbTrans)
	regTxs = []*pb.Transaction{pbTrans}
	txFee = core.GetTxFee(pbTrans)

	err = tNode.Wm.ApplyTransactions(regTxs, u1)
	assert.Nil(t, err)
	myBalance = myBalance - txFee
	assert.Equal(t, myBalance, tNode.Wm.MyWallet.GetBalance())
	assert.True(t, tNode.Dpos.State.CheckCandByAdd("127.0.0.1:8000"))

	//Double registering -> illegitimate transaction. Shouldn't be applied
	t5, err := core.RegisterCandidateTx([]byte(nodeID.Address()), tNode.PrivateKey(), 0)
	assert.NoError(t, err)
	assert.NotNil(t, t5)
	pbTrans = t5.GetProtoTransaction()
	assert.NotNil(t, pbTrans)
	regTxs = []*pb.Transaction{pbTrans}
	txFee = core.GetTxFee(pbTrans)

	err = tNode.Wm.ApplyTransactions(regTxs, u1)
	assert.Error(t, err)
	fmt.Println(err.Error())
	assert.Equal(t, myBalance, tNode.Wm.MyWallet.GetBalance()) //Balance should be the same
	assert.True(t, tNode.Dpos.State.CheckCandByAdd("127.0.0.1:8000"))
}
