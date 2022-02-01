package core

import (
	"github.com/golang/protobuf/proto"
	pb "github.com/guyu96/go-timbre/protobuf"
	"github.com/mbilal92/noise"
)

//Podf is the struct against proof of double forgery
type Podf struct {
	pbPodf *pb.Transaction
}

//NewPodf returns the new proof of double forgery instance
func NewPodf(h1, h2 *pb.BlockHeader, nonce uint64, pk string, sk noise.PrivateKey) (*Transaction, error) {

	tx := GetTransactionTemplate(pk, nonce)
	podfPayload, err := PodfPayloadToByte(h1, h2)
	if err != nil {
		return nil, err
	}
	tx.pbTransaction.Body.Payload = podfPayload
	tx.pbTransaction.Type = TxPodf
	payerSign, err := SignTransaction(tx.pbTransaction.Body, sk)
	if err != nil {
		return nil, err
	}
	tx.pbTransaction.PayerSig = payerSign

	return tx, nil
}

//PodfPayloadToByte converts the payload to byte
func PodfPayloadToByte(h1, h2 *pb.BlockHeader) ([]byte, error) {
	podfPayload := &pb.PodfPayload{
		Header1: h1,
		Header2: h2,
	}
	return proto.Marshal(podfPayload)
}

//PodfPayloadFromByte returns the payload header from byte
func PodfPayloadFromByte(payload []byte) (*pb.BlockHeader, *pb.BlockHeader, error) {
	podfPayload := new(pb.PodfPayload)
	if err := proto.Unmarshal(payload, podfPayload); err != nil {
		return nil, nil, err
	}

	return podfPayload.Header1, podfPayload.Header2, nil
}

//GetProtoPodf return the proto podf
func (p *Podf) GetProtoPodf() *pb.Transaction {
	return p.pbPodf
}

//ToBytes conver to byte
func (p *Podf) ToBytes() ([]byte, error) {
	podfBytes, err := proto.Marshal(p.pbPodf)
	if err != nil {
		return []byte{}, err
	}
	return podfBytes, nil
}

//PodfFromBytes returns the podf instance from the bytes
func PodfFromBytes(pBytes []byte) (*Podf, error) {
	var newPodf = new(Podf)
	newPodf.pbPodf = new(pb.Transaction)

	if err := proto.Unmarshal(pBytes, newPodf.pbPodf); err != nil {
		return nil, err
	}
	return newPodf, nil
}
