package core

import (
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang/protobuf/proto"
	pb "github.com/guyu96/go-timbre/protobuf"
	"github.com/mbilal92/noise"
)

//Transaction is the protopuf transaction wrapper
type Transaction struct {
	pbTransaction *pb.Transaction
}

//MakeTestTransaction creates transaction for testing pupose
func MakeTestTransaction(payerID, recipientID string, amount int64, nonce uint64, payerPrivateKey noise.PrivateKey) (*Transaction, error) {

	tBody := &pb.Transaction_Body{ //No payload for the transfer transaction
		RecipientId: recipientID,
		Decibels:    amount,
		Nonce:       nonce,
		TimeStamp:   time.Now().UnixNano(),
		SenderId:    payerID,
	}
	payerSign, err := SignTransaction(tBody, payerPrivateKey)
	if err != nil {
		return nil, err
	}

	newPbTransaction := &pb.Transaction{
		Body:     tBody,
		PayerSig: payerSign,
		Type:     TxTransfer, //Corrosponding to the transfer type transaction
	}

	newTransaction := &Transaction{
		pbTransaction: newPbTransaction,
	}

	// fmt.Println("Test transaction has been created")
	return newTransaction, nil
}

//GetTransactionTemplate returns the generic transaction template
func GetTransactionTemplate(payerID string, nonce uint64) *Transaction {
	tx := &Transaction{
		pbTransaction: &pb.Transaction{
			Body: &pb.Transaction_Body{
				SenderId:  payerID,
				Nonce:     nonce,
				TimeStamp: time.Now().UnixNano(),
			},
		},
	}
	return tx
}

//SignTransaction signs the transaction body using private key of the payer
func SignTransaction(transactionBody *pb.Transaction_Body, payerPrivateKey noise.PrivateKey) ([]byte, error) {
	tBodyBytes, err := proto.Marshal(transactionBody) //This marshals the transaction
	if err != nil {
		return nil, err
	}
	//Appending chain-id so that users on same network are able to identify it
	tBodyBytes = append(tBodyBytes, []byte(DefaultConfig.ChainID)...)

	tBodySign := payerPrivateKey.SignB(tBodyBytes)

	return tBodySign, nil
}

//HasValidSign returns if the signature is valid or not
func HasValidSign(tx *pb.Transaction) (bool, error) {
	pk, err := hex.DecodeString(tx.Body.GetSenderId())
	if err != nil {
		return false, err
	}
	publicKey := ByteToPublicKey(pk)
	m, err := proto.Marshal(tx.Body) //This marshals the transaction
	if err != nil {
		return false, err
	}
	//Checking if the transaction is from same network as mine
	m = append(m, []byte(DefaultConfig.ChainID)...)

	valid := publicKey.VerifyB(m, tx.GetPayerSig())
	if !valid {
		return false, errors.New("Failed validation")
	}

	return true, nil
}

//GetTxDecibels returns the decibles in the transaction
func (t *Transaction) GetTxDecibels() int64 {
	return t.pbTransaction.GetBody().GetDecibels()
}

//GetProtoTransaction returns proto transaction
func (t *Transaction) GetProtoTransaction() *pb.Transaction {
	return t.pbTransaction
}

//ToBytes convert it into bytes
func (t *Transaction) ToBytes() ([]byte, error) {
	return proto.Marshal(t.pbTransaction)
}

//TransactionFromBytes converts back to the transaction type from bytes
func TransactionFromBytes(transBytes []byte) (*Transaction, error) {
	trans := new(Transaction)
	trans.pbTransaction = new(pb.Transaction)
	if err := proto.Unmarshal(transBytes, trans.pbTransaction); err != nil {
		return nil, err
	}
	return trans, nil
}

//ResourceUtilized returns the resource used object
func (t *Transaction) ResourceUtilized() *ResourceUsage {
	txSize, _ := t.ToBytes()
	gasBurnt := &ResourceUsage{
		cpuUsage: 10,
		netUsage: int64(len(txSize)),
	}
	return gasBurnt
}
