package limiter

import (
	"fmt"
	"sync"
	"time"
)

//AccessLimiter limits the fund transfer to addresses
type AccessLimiter struct {
	mu            sync.Mutex
	fundedAddr    map[string]int64
	blockDuration time.Duration
}

// NewLimiter returns a new instance of the AccessLimiter
func NewLimiter(blockTime time.Duration) *AccessLimiter {
	l := &AccessLimiter{
		fundedAddr:    make(map[string]int64),
		blockDuration: blockTime,
	}

	return l
}

// Add limits value till a given time
func (l *AccessLimiter) Add(address string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fundedAddr[address] = time.Now().Unix()
}

//AllowedToGetFunds returns true if sufficient time has passed
func (l *AccessLimiter) AllowedToGetFunds(address string) (time.Duration, bool) {

	if t, ok := l.fundedAddr[address]; ok {
		if t < (time.Now().Unix() - int64(l.blockDuration.Seconds())) {
			return time.Duration((t+int64(l.blockDuration.Seconds()))-time.Now().Unix()) * time.Second, true
		}

		return time.Duration((t+int64(l.blockDuration.Seconds()))-time.Now().Unix()) * time.Second, false
	}

	return time.Duration(0), true
}

//Clear deletes the address from the funded addresses
func (l *AccessLimiter) Clear(add string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.clear(add)
}

func (l *AccessLimiter) clear(add string) {
	delete(l.fundedAddr, add)
}

//RemoveAllowedEntries removes allowed entries
func (l *AccessLimiter) RemoveAllowedEntries() {
	l.mu.Lock()
	defer l.mu.Unlock()

	for addr := range l.fundedAddr {
		if _, ok := l.AllowedToGetFunds(addr); ok {
			fmt.Println(addr, " was cleaned")
			l.clear(addr)
		}
	}
}
