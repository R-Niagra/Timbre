package storage

import (
	"errors"
	"fmt"
	"sync"
)

//TransactionPool contains the pool for all the transaction types
type TransactionPool struct {
	tMutex   sync.Mutex
	tPool    []interface{} //FIFO-It should entertain all the transaction types including votes,retrival tx, deals,verification
	capacity uint
}

//NewTransactionPool returns the TransactionPool object
func NewTransactionPool(cacheSize uint) *TransactionPool {
	return &TransactionPool{
		tPool:    make([]interface{}, 0, cacheSize),
		capacity: cacheSize,
	}
}

//Push pushes the transaction to the end of the tPool
func (tp *TransactionPool) Push(tx interface{}) error {
	tp.tMutex.Lock()
	defer tp.tMutex.Unlock()

	if len(tp.tPool) >= int(tp.capacity) {
		return errors.New("Transaction pool is already full. Can't add more")
	}
	// fmt.Println("Pushing type!!!: ", reflect.TypeOf(tx).Elem())
	tp.tPool = append(tp.tPool, tx)
	return nil
}

//Pop gives the first element of the transaction Pool
func (tp *TransactionPool) Pop() (interface{}, error) {
	tp.tMutex.Lock()
	defer tp.tMutex.Unlock()

	if len(tp.tPool) == 0 {
		return nil, errors.New("Transaction pool is empty")
	}
	tx := tp.tPool[0]
	// fmt.Println("Poping type!!!: ", reflect.TypeOf(tx).Elem())
	tp.tPool = tp.tPool[1:]
	return tx, nil
}

//GetHead returns the head of the pool without popping the transaction
func (tp *TransactionPool) GetHead() (interface{}, error) {
	tp.tMutex.Lock()
	defer tp.tMutex.Unlock()

	if len(tp.tPool) == 0 {
		return nil, errors.New("Transaction pool is empty")
	}
	tx := tp.tPool[0]
	return tx, nil
}

//LengthOfPool returns the length of the transaction Pool
func (tp *TransactionPool) LengthOfPool() int {
	tp.tMutex.Lock()
	defer tp.tMutex.Unlock()
	return len(tp.tPool)
}

//IsEmpty returns true in case pool is empty
func (tp *TransactionPool) IsEmpty() bool {
	if len(tp.tPool) == 0 {
		return true
	}
	return false
}

//EmptyPool empties the pool
func (tp *TransactionPool) EmptyPool() {
	tp.tMutex.Lock()
	defer tp.tMutex.Unlock()
	tp.tPool = make([]interface{}, 0, tp.capacity)
}

//GetTransactions simply returns the transactions
func (tp *TransactionPool) GetTransactions() []interface{} {
	return tp.tPool
}

//RemoveRedundantTx removes the transactions which have already been added in the block
func (tp *TransactionPool) RemoveRedundantTx(added []interface{}) error {
	if tp.LengthOfPool() == 0 {
		return nil
	}

	for x := range added {
		deleted := 0
		tp.tMutex.Lock()
		str1 := fmt.Sprintf("%v", added[x])
		for y := range tp.tPool {
			i := y - deleted
			str2 := fmt.Sprintf("%v", tp.tPool[i])

			if str1 == str2 {
				fmt.Println("Evicting stale tx from cache!!!!!!!!!!")
				tp.tPool = tp.tPool[:i+copy(tp.tPool[i:], tp.tPool[i+1:])]
				deleted++
			}
		}
		tp.tMutex.Unlock()

	}

	return nil
}
