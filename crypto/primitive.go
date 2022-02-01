package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"

	ed "golang.org/x/crypto/ed25519"
)

// NewPrivateKey generates a new ed25519 keypair
func GenerateKeys() (ed.PublicKey, ed.PrivateKey) {
	pubKey, prvKey, err := ed.GenerateKey(rand.Reader)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate ed25519 key: %s", err))
	}
	return pubKey, prvKey
}

func structToByteSlice(s interface{}) []byte {
	return []byte(fmt.Sprintf("%v", s))
}

func StructsToByteSlice(structs ...interface{}) []byte {
	bytes := []byte{}
	for _, s := range structs {
		bytes = append(bytes, structToByteSlice(s)...)
	}
	return bytes
}

// Sha256 returns the SHA-256 digest of the data
func Sha256(args ...[]byte) []byte {
	hasher := sha256.New()
	for _, bytes := range args {
		hasher.Write(bytes)
	}
	return hasher.Sum(nil)
}

// HashStructs returns a Sha256 hash of the given structs
func HashStructs(structs ...interface{}) []byte {
	return Sha256(StructsToByteSlice(structs))
}

// Sign wraps ed.Sign
func Sign(prvKey ed.PrivateKey, msg []byte) []byte {
	return ed.Sign(prvKey, msg)
}

// Verify wraps ed.Verify
func Verify(pubKey ed.PublicKey, msg []byte, sig []byte) bool {
	return ed.Verify(pubKey, msg, sig)
}
