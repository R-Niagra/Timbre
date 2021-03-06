package pos

import (
	"fmt"
	"math/big"

	"github.com/guyu96/go-timbre/crypto"
	// "github.com/mbilal92/pbc"
	"github.com/Nik-U/pbc"
)

// Verify verifies the validity of the proof generated by the storage provider given the file name and the challenge. It returns true if the proof is valid and false otherwise.
func Verify(name []byte, pairing *pbc.Pairing, pk *PublicKey, ch *Chall, prf *Proof) bool {
	r, rho, I := ch.r, ch.rho, ch.I
	psi, y, tag := prf.psi, prf.y, prf.tag
	// Compute v[i] = rho^I[i], as in Prove
	v := make([]*pbc.Element, len(I))
	for i := range v {
		exp := big.NewInt(int64(I[i]))
		v[i] = pairing.NewZr().Set(rho).ThenPowBig(exp)
	}
	// Compute eta = product_i(u^(hash(name||i)) * v[i])
	eta := pairing.NewG1().Set1()
	for i, vi := range v {
		idx := I[i] // idx is 1-indexed
		// Compute h = hash(name||i) and the exponent of u, h * v[i]
		h := new(big.Int).SetBytes(crypto.Sha256(name, []byte(fmt.Sprintf("%d", idx))))
		exp := pairing.NewZr().Set(vi).ThenMulBig(h)
		term := pairing.NewG1().Set(pk.u).ThenPowZn(exp)
		eta.ThenMul(term)
	}
	// Check if e(eta, v) * e(psi, k * v^(-r)) == e(tag, g) * e(g^(-y), v), where e denotes the pairing operation
	// Labeling from left to right, we need to check if t1 * t2 == t3 * t4
	g := pk.gaPow[0]
	t1 := pairing.NewGT().Pair(eta, pk.v)
	vr := pairing.NewG1().Set(pk.v).ThenPowZn(pairing.NewZr().Set(r).ThenNeg()) // v^(-r)
	t2 := pairing.NewGT().Pair(psi, pairing.NewG1().Set(pk.k).ThenMul(vr))
	t3 := pairing.NewGT().Pair(tag, g)
	gy := pairing.NewG1().Set(g).ThenPowZn(pairing.NewZr().Set(y).ThenNeg()) // g^(-y)
	t4 := pairing.NewGT().Pair(gy, pk.v)
	lhs := pairing.NewGT().Set(t1).ThenMul(t2)
	rhs := pairing.NewGT().Set(t3).ThenMul(t4)
	return lhs.Equals(rhs)
}
