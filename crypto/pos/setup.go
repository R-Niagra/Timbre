package pos

import (
	"fmt"
	"math/big"

	"github.com/Nik-U/pbc"
	"github.com/guyu96/go-timbre/crypto"
	// "github.com/mbilal92/pbc"
)

// AuthTags is the authentication tags generated for a message.
type AuthTags []*pbc.Element

// Setup computes and returns the authentication tags sigma_i for the message M given the message name.
func Setup(M []byte, name []byte, pairing *pbc.Pairing, pk *PublicKey, sk *SecretKey, cfg *Config) AuthTags {
	blocks := Process(M, cfg)
	tags := make(AuthTags, len(blocks))
	for i := range blocks {
		// Compute h = hash(name||i) and u^h
		h := new(big.Int).SetBytes(crypto.Sha256(name, []byte(fmt.Sprintf("%d", i+1)))) // i is required to be 1-indexed here, hence the +1
		uh := pairing.NewG1().Set(pk.u).ThenPowBig(h)
		// Compute product of powers (g^(a^j))^mij
		prod := pairing.NewG1().Set1()
		for j := range blocks[i] {
			mij := new(big.Int).SetBytes(blocks[i][j])
			gaj := pk.gaPow[j]
			term := pairing.NewG1().Set(gaj).ThenPowBig(mij) // term = (g^(a^j))^mij
			prod.ThenMul(term)
		}
		// Finally, compute tag
		tags[i] = pairing.NewG1().Set(uh).ThenMul(prod).ThenPowZn(sk.e)
	}
	return tags
}

// Process divides the message M into blocks based on the given configuration
func Process(M []byte, cfg *Config) [][][]byte {
	s := cfg.s         // number of elements in each data block
	N := cfg.N         // size of an element in bytes
	size := s * N      // size of a data block in bytes
	n := len(M) / size // number of blocks
	r := len(M) % size // size of remainder block in bytes
	pad := 0           // padding required in bytes
	if r != 0 {
		n++
		pad = size - r
	}

	// Make a copy of M to avoid changing the original message and pad the end if necessary
	MCopy := make([]byte, len(M)+pad)
	copy(MCopy, M)
	// Divide MCopy into blocks
	blocks := make([][][]byte, n)
	for i := 0; i < n; i++ { // for each data block
		blocks[i] = make([][]byte, s)
		for j := 0; j < s; j++ { // for each element
			blocks[i][j] = MCopy[i*size+j*N : i*size+(j+1)*N]
		}
	}
	return blocks
}

func GetNumberofBlocksforTag(msgSize int, cfg *Config) int {
	s := cfg.s    // number of elements in each data block
	N := cfg.N    // size of an element in bytes
	size := s * N // size of a data block in bytes
	return msgSize / size
}

func (tags *AuthTags) String() string {
	tagsAsSlice := ([]*pbc.Element)(*tags)
	return fmt.Sprintf("Authentication Tags:\n\t%v", tagsAsSlice)
}

func (tags *AuthTags) ToByteArray() [][]byte {
	tagsAsSlice := ([]*pbc.Element)(*tags)
	numberOfTags := len(tagsAsSlice)
	tagBytesList := make([][]byte, numberOfTags)
	for i, tag := range tagsAsSlice {
		tagBytes := tag.CompressedBytes()
		tagBytesList[i] = tagBytes
	}

	return tagBytesList
}

func TagsFromByteArray(databytes [][]byte, pairing *pbc.Pairing) AuthTags {
	tags := make([]*pbc.Element, len(databytes))
	for i, tagBytes := range databytes {
		tags[i] = pairing.NewG1().SetCompressedBytes(tagBytes)
	}

	return (AuthTags)(tags)
}
