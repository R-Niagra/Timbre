package pos

import (
	"fmt"
	"os"

	"github.com/Nik-U/pbc"
	pb "github.com/guyu96/go-timbre/protobuf"
	// "github.com/mbilal92/pbc"
)

// PublicKey contains the public keys for our PoS scheme
type PublicKey struct {
	v     *pbc.Element
	k     *pbc.Element // kappa
	u     *pbc.Element
	gaPow []*pbc.Element
}

// SecretKey contains the secret keys for our PoS scheme
type SecretKey struct {
	e *pbc.Element // epsilon
	a *pbc.Element // alpha
}

func NewpublicKey(pairing *pbc.Pairing, cfg *Config) *PublicKey {
	g := pairing.NewG1().Rand()      // generator of G
	a := pairing.NewZr().Rand()      // alpha, generator of Zr
	e := pairing.NewZr().Rand()      // epsilon, generator of Zr
	u := pairing.NewG1().Rand()      // generator of G
	v := pairing.NewG1().PowZn(g, e) // g^e
	k := pairing.NewG1().PowZn(v, a) // kappa, g^(ae)

	// We also need g^(a^i) for i = 0...s-1
	gaPow := make([]*pbc.Element, cfg.s)
	gPower := g.PreparePower()
	ai := pairing.NewZr().Set(a) // initialize ai to a^1
	for i := 0; i < cfg.s; i++ {
		gaPow[i] = pairing.NewG1()
		if i == 0 {
			gaPow[i].Set(g) // g^(a^0) = g
		} else if i == 1 {
			gPower.PowZn(gaPow[i], a) // g^(a^1) = g^a
		} else {
			ai.Mul(ai, a) // compute a^(i+1)
			gPower.PowZn(gaPow[i], ai)
		}
	}

	// Generate public and secret keys
	pk := &PublicKey{
		v:     v,
		k:     k,
		u:     u,
		gaPow: gaPow,
	}

	return pk
}

// KeyGen generates a bilinear map (pairing) and the public and secret keys required by our PoS scheme
// Generic discrete log must be infeasible in groups of order r
// Finite field discrete log must be infeasible in groups of order q^2
// cfg contains s, the number of elements in a data block
func KeyGen(rbits uint32, qbits uint32, cfg *Config) (*pbc.Pairing, *PublicKey, *SecretKey, *pbc.Params) {
	// Generate symmetric pairing on Type A curve
	f, _ := os.Open("../PosParam.config")
	params, _ := pbc.NewParams(f)
	// params := pbc.GenerateA(rbits, qbits)
	pairing := params.NewPairing()
	// pairing, _ := pbc.NewPairingFromString(deal.PosParam)

	// Generate elements
	g := pairing.NewG1().Rand()      // generator of G
	a := pairing.NewZr().Rand()      // alpha, generator of Zr
	e := pairing.NewZr().Rand()      // epsilon, generator of Zr
	u := pairing.NewG1().Rand()      // generator of G
	v := pairing.NewG1().PowZn(g, e) // g^e
	k := pairing.NewG1().PowZn(v, a) // kappa, g^(ae)

	// We also need g^(a^i) for i = 0...s-1
	gaPow := make([]*pbc.Element, cfg.s)
	gPower := g.PreparePower()
	ai := pairing.NewZr().Set(a) // initialize ai to a^1
	for i := 0; i < cfg.s; i++ {
		gaPow[i] = pairing.NewG1()
		if i == 0 {
			gaPow[i].Set(g) // g^(a^0) = g
		} else if i == 1 {
			gPower.PowZn(gaPow[i], a) // g^(a^1) = g^a
		} else {
			ai.Mul(ai, a) // compute a^(i+1)
			gPower.PowZn(gaPow[i], ai)
		}
	}

	// Generate public and secret keys
	pk := &PublicKey{
		v:     v,
		k:     k,
		u:     u,
		gaPow: gaPow,
	}
	sk := &SecretKey{
		e: e,
		a: a,
	}

	return pairing, pk, sk, params
}

// KeyGenDefault generates a bilinear map with rbits = 160 and qbits = 512 and the public and secret keys required by our PoS scheme
func KeyGenDefault(cfg *Config) (*pbc.Pairing, *PublicKey, *SecretKey, *pbc.Params) {
	return KeyGen(160, 512, cfg)
}

// KeyGenTest generates a bilinear map with rbits = 8 and qbits = 16 and the public and secret keys for testing purposes
func KeyGenTest(cfg *Config) (*pbc.Pairing, *PublicKey, *SecretKey, *pbc.Params) {
	return KeyGen(8, 16, cfg)
}

func (pk *PublicKey) String() string {
	s := "Public Key:\n"
	s += fmt.Sprintf("\tv: %s\n\tk: %s\n\tu: %s\n", pk.v, pk.k, pk.u)
	s += "\t{g^(a^i)}_i:\n"
	for _, el := range pk.gaPow {
		s += fmt.Sprintf("\t\t%s\n", el)
	}
	return s
}

// ToProto converts PublicKey struct to PublicKey protobuf struct.
func (pk *PublicKey) ToProto() *pb.PublicKey {
	v := pk.v.CompressedBytes()
	k := pk.k.CompressedBytes()
	u := pk.u.CompressedBytes()
	gaPow := make([][]byte, len(pk.gaPow))
	for i, gai := range pk.gaPow {
		gaPow[i] = gai.CompressedBytes()
	}
	return &pb.PublicKey{
		V:     v,
		K:     k,
		U:     u,
		GaPow: gaPow,
	}
}

// FromProto sets PublicKey based on PublicKey protobuf struct.
func (pk *PublicKey) FromProto(pbPK *pb.PublicKey, pairing *pbc.Pairing) {
	pk.v = pairing.NewG1().SetCompressedBytes(pbPK.V)
	pk.k = pairing.NewG1().SetCompressedBytes(pbPK.K)
	pk.u = pairing.NewG1().SetCompressedBytes(pbPK.U)
	pk.gaPow = make([]*pbc.Element, len(pbPK.GaPow))
	for i, gaiBytes := range pbPK.GaPow {
		pk.gaPow[i] = pairing.NewG1().SetCompressedBytes(gaiBytes)
	}
}

// Equals checks if two public keys are the same.
func (pk *PublicKey) Equals(anotherPK *PublicKey) bool {
	if !(pk.v.Equals(anotherPK.v) && pk.k.Equals(anotherPK.k)) {
		return false
	}
	if len(pk.gaPow) != len(anotherPK.gaPow) {
		return false
	}
	for i := range pk.gaPow {
		if !pk.gaPow[i].Equals(anotherPK.gaPow[i]) {
			return false
		}
	}
	return true
}

func (sk *SecretKey) String() string {
	s := "Secret Key:\n"
	s += fmt.Sprintf("\te: %s\n\ta: %s\n", sk.e, sk.a)
	return s
}
