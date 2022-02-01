package core

import (
	pb "github.com/guyu96/go-timbre/protobuf"
	"github.com/mbilal92/noise"
)

//RegisterCandidateTx returns the candidate registration transaction
func RegisterCandidateTx(payerID string, payerPrivateKey noise.PrivateKey, nonce uint64) (*Transaction, error) {
	tBody := &pb.Transaction_Body{
		SenderId: payerID,
		Nonce:    nonce,
	}
	payerSign, err := SignTransaction(tBody, payerPrivateKey)
	if err != nil {
		return nil, err
	}
	newPbTransaction := &pb.Transaction{
		Body:     tBody,
		Type:     TxBecomeCandidate,
		PayerSig: payerSign,
	}

	newTransaction := &Transaction{
		pbTransaction: newPbTransaction,
	}
	return newTransaction, nil
}

//QuitCandidateTx returns the quit candidate transaction
func QuitCandidateTx(payerID string, payerPrivateKey noise.PrivateKey, nonce uint64) (*Transaction, error) {
	tBody := &pb.Transaction_Body{
		SenderId: payerID,
		Nonce:    nonce,
	}
	payerSign, err := SignTransaction(tBody, payerPrivateKey)
	if err != nil {
		return nil, err
	}
	newPbTransaction := &pb.Transaction{
		Body:     tBody,
		Type:     TxQuitCandidate,
		PayerSig: payerSign,
	}

	newTransaction := &Transaction{
		pbTransaction: newPbTransaction,
	}
	return newTransaction, nil
}
