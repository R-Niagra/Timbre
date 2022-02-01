package pos

import (
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/Nik-U/pbc"
	// "github.com/mbilal92/pbc"
)

func TestKeyGen(t *testing.T) {
	cfg := MakeTestConfig()
	pairing, pk, sk := KeyGenTest(cfg)
	u := pairing.NewG1().Rand()
	v := pairing.NewG1().Rand()
	euv := pairing.NewGT().Pair(u, v)
	evu := pairing.NewGT().Pair(v, u)
	if !(pairing.IsSymmetric() || euv.Equals(evu)) {
		t.Fatal("Pairing is not symmetric.")
	}
	g := pk.gaPow[0]
	ae := pairing.NewZr().Set(sk.a).ThenMul(sk.e)
	if !pk.k.Equals(pairing.NewG1().Set(g).ThenPowZn(ae)) {
		t.Fatal("Error generating kappa in secret key.")
	}
	for i := 1; i < len(pk.gaPow); i++ {
		ai := pairing.NewZr().Set(sk.a).ThenPowBig(big.NewInt(int64(i)))
		gai := pairing.NewG1().Set(g).ThenPowZn(ai)
		if !gai.Equals(pk.gaPow[i]) {
			t.Fatal("Error generating the powers g^(a^i) in public key.")
		}
	}
}

func TestProcess(t *testing.T) {
	rand.Seed(time.Now().UTC().UnixNano())
	cfg := MakeTestConfig()
	// Generate random message that should be split into 3 blocks
	size := cfg.s * cfg.N
	remainder := rand.Intn(size-1) + 1 // ensure remainder is non-zero modular size
	M := make([]byte, 2*size+remainder)
	for i := range M {
		M[i] = byte(rand.Int())
	}
	// Process message and check dimension and content
	blocks := Process(M, cfg)
	if len(blocks) != 3 {
		t.Fatal("Process split the message into a wrong number of blocks.")
	}
	for _, block := range blocks {
		if len(block) != cfg.s {
			t.Fatal("The number of elements contained in each block is incorrect.")
		}
	}
	for i := range M {
		blockI := i / size
		elementI := (i % size) / cfg.N
		byteI := (i % size) % cfg.N
		if M[i] != blocks[blockI][elementI][byteI] {
			t.Fatal("Process failed to replicate the message data.")
		}
	}
}

func TestPoly(t *testing.T) {
	cfg := MakeTestConfig()
	pairing, _, _ := KeyGenTest(cfg)
	makeZp := getZpMaker(pairing)
	// Create the polynomial 5 + 3x + 2x^2 and the point 10
	poly := []*pbc.Element{makeZp(5), makeZp(3), makeZp(2)}
	point := makeZp(10)
	evalResult := polyEval(poly, point, pairing)
	quotient, remainder := polyDiv(poly, point, pairing)
	// Evaluation result should be 235
	if !evalResult.Equals(makeZp(235)) {
		t.Fatal("polyEval returned incorrect result.")
	}
	// Quotient should be 23 + 2x and remainder should be 235
	if !(quotient[0].Equals(makeZp(23)) || quotient[1].Equals(makeZp(2))) {
		t.Fatal("polyDiv returned incorrect quotient.")
	}
	if !remainder.Equals(makeZp(235)) {
		t.Fatal("polyDiv returned incorrect remainder.")
	}
}

func TestProto(t *testing.T) {
	cfg := MakeTestConfig()
	M := []byte("I am a message used for testing protobuf marshaling and unmarshaling.")
	name := []byte("I am just a file name")
	pairing, pk, sk := KeyGenTest(cfg)
	// pairing1, _, _ := KeyGenTest(cfg)
	pbPK := new(PublicKey)
	pbPK.FromProto(pk.ToProto(), pairing)
	if !pk.Equals(pbPK) {
		t.Fatal("Protobuf conversion for PublicKey failed.")
	}
	tags := Setup(M, name, pairing, pk, sk, cfg)
	ch := Challenge(len(tags), 2, pairing)
	pbCh := new(Chall)
	pbCh.FromProto(ch.ToProto(), pairing)
	if !ch.Equals(pbCh) {
		t.Fatal("Protobuf conversion for Chall failed.")
	}
	prf := Prove(M, cfg, pairing, pk, tags, ch)
	pbPrf := new(Proof)
	pbPrf.FromProto(prf.ToProto(), pairing)
	if !prf.Equals(pbPrf) {
		t.Fatal("Protobuf conversion for Proof failed.")
	}
}

func TestPos(t *testing.T) {
	cfg := MakeTestConfig()
	M := []byte("I am a message used for testing the PoS scheme. Testing the PoS scheme is my only purpose. My entire existence depends upon the validity of the PoS scheme.")
	name := []byte("I am just a file name")
	pairing, pk, sk := KeyGenTest(cfg)
	tags := Setup(M, name, pairing, pk, sk, cfg)
	ch := Challenge(len(tags), 3, pairing)
	prf := Prove(M, cfg, pairing, pk, tags, ch)
	randomPrf := &Proof{
		psi: pairing.NewG1().Rand(),
		y:   pairing.NewZr().Rand(),
		tag: pairing.NewG1().Rand(),
	}
	valid := Verify(name, pairing, pk, ch, prf)
	randomValid := Verify(name, pairing, pk, ch, randomPrf)
	if !valid {
		t.Fatal("Verify failed on a valid proof.")
	}
	if randomValid {
		t.Fatal("Verify succeeded on a random proof.")
	}
}

// getZpMaker constructs a maker that generates the residue of integer x in Zp
func getZpMaker(pairing *pbc.Pairing) func(x int) *pbc.Element {
	return func(x int) *pbc.Element {
		return pairing.NewZr().Set1().ThenMulBig(big.NewInt(int64(x)))
	}
}
