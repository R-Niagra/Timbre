package wallet

import (
	"fmt"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

func TestNewWallet(t *testing.T) {
	myWal := NewWallet("14555004962417d54bc623a18396ba0bcaf0a6694fcb7eb52f0681f65f4ecc7a")
	//Make sure the lenght of the podfHeghtts is zero to begin with
	assert.Equal(t, 0, len(myWal.podfHeights))
	//adding entries in the wallet to estimate the bytes
	myWal.podfHeights = append(myWal.podfHeights, 100)

	fmt.Println(int(unsafe.Sizeof(*myWal)))

	myWal.Voted["14555004962417d54bc623a18396ba0bcaf0a6694fcb7eb52f0681f65f4ecc7a"] = 30
	myWal.Voted["14555004962417d54bc623a18396ba0bcaf0a6694fcb7eb52f0681f65f4ecc7a"] = 20
	myWal.Voted["14555004962417d54bc623a18396ba0bcaf0a6694fcb7eb52f0681f65f4ecc7a"] = 20
	myWal.Voted["14555004962417d54bc623a18396ba0bcaf0a6694fcb7eb52f0681f65f4ecc7a"] = 20
	fmt.Println(int(unsafe.Sizeof(*myWal)))

}

func TestWallet_DeepCopy(t *testing.T) {
	//Creating new wallet
	myWallet := NewWallet("test")
	myWallet.podfHeights = append(myWallet.podfHeights, 1)
	myWallet.podfHeights = append(myWallet.podfHeights, 2)
	myWallet.podfHeights = append(myWallet.podfHeights, 3)
	myWallet.podfHeights = append(myWallet.podfHeights, 4)

	myWallet.Voted["a"] = 10
	myWallet.Voted["b"] = 10
	myWallet.Voted["c"] = 10

	copyWallet := myWallet.DeepCopy()

	myWallet.podfHeights = append(myWallet.podfHeights, 5) //Making changes afterwards to see if it gets reflected in the copied wallet
	myWallet.Voted["d"] = 10
	myWallet.Voted["e"] = 10
	myWallet.Voted["a"] = 20

	assert.Equal(t, 4, len(copyWallet.podfHeights))
	assert.Equal(t, 3, len(copyWallet.Voted))
	assert.Equal(t, 10, copyWallet.Voted["a"])
}
