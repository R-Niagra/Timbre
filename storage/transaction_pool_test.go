package storage

import (
	"testing"
)

func TestNewTransactionPool(t *testing.T) {
	pool := NewTransactionPool(5)

	if pool == nil {
		t.Errorf("Cannot create pool instance")
	}
}
func TestTransactionPool_Push(t *testing.T) {
	pool := NewTransactionPool(5)

	pool.Push(3)
	pool.Push(4)
	pool.Push(4)
	pool.Push(4)
	pool.Push(4)

	if pool.LengthOfPool() != 5 {
		t.Errorf("Length not equivilant to pushed numbers")
	}

}

func TestTransactionPool_Pop(t *testing.T) {

	pool := NewTransactionPool(10)
	_, err := pool.Pop()
	if err == nil {
		t.Errorf("Poping the empty list")
	}

	pool.Push(3)
	pool.Push(4)
	val, err := pool.Pop()
	if val != 3 {
		t.Errorf("Poping wrong element")
	}
}

func TestTransactionPool_LengthOfPool(t *testing.T) {
	pool := NewTransactionPool(10)
	if pool.LengthOfPool() != 0 {
		t.Errorf("wrong length")
	}
	pool.Push(3)
	pool.Push(4)
	pool.Push(5)
	pool.Push(2)

	if pool.LengthOfPool() != 4 {
		t.Errorf("wrong length")
	}

	pool.Pop()
	if pool.LengthOfPool() != 3 {
		t.Errorf("wrong length")
	}
}

func TestTransactionPool_RemoveRedundantTx(t *testing.T) {
	pool := NewTransactionPool(10)

	pool.Push(2)
	pool.Push(2)
	pool.Push(2)
	pool.Push(1)
	pool.Push(0)

	err := pool.RemoveRedundantTx([]interface{}{1, 2, 3})
	if err != nil {
		t.Error(err)
	}
	if pool.tPool[0] != 0 {
		t.Errorf("Not able to completely remove redundency")
	}

}
