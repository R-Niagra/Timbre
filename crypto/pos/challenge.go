package pos

import (
	"fmt"
	"math/rand"

	"github.com/Nik-U/pbc"
	pb "github.com/guyu96/go-timbre/protobuf"
	// "github.com/mbilal92/pbc"
)

// Chall is the challenge issued by the verifier
type Chall struct {
	r   *pbc.Element // from Zp
	rho *pbc.Element // from Zp
	I   []int        // random subset of {1...n}, where n is the number of data blocks
}

// Challenge creates the challenge that will be sent to the storage provider. It chooses two random elements from Zp and a l-element subset of {1...n}.
func Challenge(randNum *rand.Rand, n int, l int, pairing *pbc.Pairing) *Chall {
	// randNum := rand.New(rand.NewSource(seed))
	r := pairing.NewZr().Rand()
	rho := pairing.NewZr().Rand()
	indices := make([]int, n)
	for i := 0; i < n; i++ {
		indices[i] = i + 1
	}
	// rand.Seed(time.Now().UTC().UnixNano())
	randNum.Shuffle(n, func(i, j int) {
		indices[i], indices[j] = indices[j], indices[i]
	})
	// If l is much smaller than n, making a new slice can significantly reduce memory usage compared to re-slicing
	I := make([]int, l)
	for j := 0; j < l; j++ {
		I[j] = indices[j]
	}
	return &Chall{
		r:   r,
		rho: rho,
		I:   I,
	}
}

func (ch *Chall) String() string {
	return fmt.Sprintf("Challenge:\n\tr: %s \trho: %s \tI: %v\n", ch.r, ch.rho, ch.I)
}

// ToProto converts Chall struct to Chall protobuf struct.
func (ch *Chall) ToProto() *pb.Chall {
	I := make([]int32, len(ch.I))
	for i := range I {
		I[i] = int32(ch.I[i])
	}
	return &pb.Chall{
		R:   ch.r.Bytes(),
		Rho: ch.rho.Bytes(),
		I:   I,
	}
}

// FromProto sets Chall based on Chall protobuf struct.
func (ch *Chall) FromProto(pbChall *pb.Chall, pairing *pbc.Pairing) {
	ch.r = pairing.NewZr().SetBytes(pbChall.R)
	ch.rho = pairing.NewZr().SetBytes(pbChall.Rho)
	ch.I = make([]int, len(pbChall.I))
	for i := range ch.I {
		ch.I[i] = int(pbChall.I[i])
	}
}

// Equals checks if two challenges are the same.
func (ch *Chall) Equals(anotherCh *Chall) bool {
	if !(ch.r.Equals(anotherCh.r) && ch.rho.Equals(anotherCh.rho)) {
		return false
	}
	if len(ch.I) != len(anotherCh.I) {
		return false
	}
	for i := range ch.I {
		if ch.I[i] != anotherCh.I[i] {
			return false
		}
	}
	return true
}
