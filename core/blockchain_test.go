package core

// import (
// 	"encoding/hex"
// 	"fmt"
// 	"os"
// 	"testing"

// 	pb "github.com/guyu96/go-timbre/protobuf"
// 	"github.com/guyu96/go-timbre/storage"
// 	"github.com/guyu96/noise/crypto/edwards25519"
// )

// func TestNewBlockchain(t *testing.T) {
// 	// Delete existing test database file if there is one
// 	os.Remove(storage.DefaultTestDbPath)
// 	publicKey, privateKey, err := edwards25519.GenerateKey(nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	db, err := storage.OpenDatabase(storage.DefaultTestDbPath)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer db.Close()
// 	// Test NewBlockchain
// 	bc, err := CreateBlockchain(db, publicKey, privateKey)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	// Test LoadBlockchain on the new blockchain with a genesis block only
// 	loadedBc, err := LoadBlockchain(db)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if !SameBlockchains(bc, loadedBc) {
// 		t.Fatal("LoadBlockchain failed to reconstruct the original blockchain.")
// 	}
// 	// Test Append
// 	bc = loadedBc
// 	genesisBlock := bc.Genesis()
// 	fakeGenesisBlock, _ := NewGenesisBlock(publicKey, 0, nil)
// 	fakeGenesisBlock.Sign(privateKey)
// 	blocks1 := MakeTestBlocks(genesisBlock, 10, publicKey, privateKey) // first fork with length 10
// 	// blocks2 := MakeTestBlocks(blocks1[1], 15, publicKey, privateKey)       // second fork starting from block 2 and with length 15
// 	// blocks3 := MakeTestBlocks(fakeGenesisBlock, 16, publicKey, privateKey) // third fork starting from a fake genesis block
// 	// blocks4 := MakeTestBlocks(blocks2[3], 3, publicKey, privateKey)        // fourth fork starting from a the 6th block of 2nd fork and length 9
// 	// blocks5 := MakeTestBlocks(blocks2[3], 20, publicKey, privateKey)       // starting from block 6 length 26
// 	blocks6 := MakeTestBlocks(blocks1[1], 11, publicKey, privateKey)

// 	fmt.Println("&&&&&&&&&&&&&&&&&&&&&&&&&")
// 	for _, it := range blocks1 {
// 		fmt.Println("ParentHash:", hex.EncodeToString(it.ParentHash()))
// 		fmt.Println(it.String())
// 	}
// 	fmt.Println("&&&&&&&&&&&&&&&&&&&&&&&&&")
// 	fmt.Println("&&&&&&&&&&&&&&&&&&&&&&&&&")
// 	for _, it := range blocks6 {
// 		fmt.Println("ParentHash:", hex.EncodeToString(it.ParentHash()))
// 		fmt.Println(it.String())
// 	}
// 	fmt.Println("&&&&&&&&&&&&&&&&&&&&&&&&&")

// 	if err := bc.Append(blocks1); err != nil {
// 		t.Fatal(err)
// 	}
// 	bc.UpdateMainTailIndex()

// 	if err := bc.Append(blocks6); err != nil {
// 		t.Fatal(err)
// 	}

// 	blocksToRollBack, blocksToProcess, _, err := bc.UpdateMainTailIndex()
// 	// Should be from block 7 as block 6 is the common parent
// 	// these are in decreasing order,
// 	fmt.Println("To Revert:")
// 	for _, b := range blocksToRollBack {
// 		fmt.Println(b)
// 	}
// 	fmt.Println("--------------------------------")
// 	fmt.Println("To Process:")
// 	for i, _ := range blocksToProcess {
// 		fmt.Println(blocksToProcess[len(blocksToProcess)-1-i])
// 	}

// 	// if err := bc.Append(blocks1); err != nil {
// 	// 	t.Fatal(err)
// 	// }
// 	// bc.UpdateMainTailIndex()

// 	// if err := bc.Append(blocks2); err != nil {
// 	// 	t.Fatal(err)
// 	// }
// 	// bc.UpdateMainTailIndex()

// 	// if err := bc.Append(blocks3); err == nil {
// 	// 	t.Fatal("Failed to reject illegal append.")
// 	// }
// 	// bc.UpdateMainTailIndex()

// 	// // Check that the main tail remains the same after addition of shorter fork
// 	// if err := bc.Append(blocks4); err != nil {
// 	// 	t.Fatal(err)
// 	// }
// 	// bc.UpdateMainTailIndex()

// 	// if bc.Length() != 17 {
// 	// 	t.Fatal("Shorter fork added became the main tail incorrectly")
// 	// }

// 	// // Test Addition of longer fork
// 	// if err := bc.Append(blocks5); err != nil {
// 	// 	t.Fatal(err)
// 	// }
// 	// blocksToRollBack, blocksToProcess, _, err := bc.UpdateMainTailIndex()
// 	// // Should be from block 7 as block 6 is the common parent
// 	// // these are in decreasing order,
// 	// fmt.Println("To Revert:")
// 	// for _, b := range blocksToRollBack {
// 	// 	fmt.Println(b)
// 	// }
// 	// fmt.Println("--------------------------------")
// 	// fmt.Println("To Process:")
// 	// for i, _ := range blocksToProcess {
// 	// 	fmt.Println(blocksToProcess[len(blocksToProcess)-1-i])
// 	// }

// 	// if bc.Length() != 26 {
// 	// 	t.Fatal("Shorter fork added became the main tail incorrectly")
// 	// }

// 	// // Test Fork deletion
// 	// err = bc.DeleteFork(bc.tails[0])
// 	// if bc.NumForks() != 3 || err != nil {
// 	// 	t.Fatal("Fork deletion error")
// 	// }

// 	// // Common ancestor b/w main tail and main tail
// 	// b, err := bc.findCommonAncestor(bc.tails[2])
// 	// if b.HeightInt() != bc.GetMainTail().HeightInt() {
// 	// 	t.Fatal("Incorrect Common Parent")
// 	// }

// 	// // Common Ancestor b/w maintail and shorter tail
// 	// b, err = bc.findCommonAncestor(bc.tails[0])
// 	// if err != nil || !bytes.Equal(b.Hash(), blocks2[3].Hash()) {
// 	// 	t.Fatal("Common Ancestor Error")
// 	// }

// 	// // Check if main tail can be removed
// 	// err = bc.RemoveBlock(bc.GetMainTail())
// 	// if err == nil {
// 	// 	t.Fatal(err)
// 	// }

// 	// fmt.Println(bc.String())
// 	// if bc.NumForks() != 3 || bc.mainTailIndex != 2 || bc.Length() != 26 { // two forks, the second one should be longer with length 7
// 	// 	t.Fatal("Blockchain forks bookkeeping error.")
// 	// }
// 	// // Test LoadBlockchain after Append
// 	// loadedBc, err = LoadBlockchain(db)
// 	// if err != nil {
// 	// 	t.Fatal(err)
// 	// }
// 	// t.Logf("Before Load:\n%s\nAfter Load:\n%s\n", bc, loadedBc)
// 	// if !SameBlockchains(bc, loadedBc) {
// 	// 	t.Fatal("LoadBlockchain failed to reconstruct the original blockchain.")
// 	// }
// }

// func TestBlockchain_BinaryDealSearch(t *testing.T) {
// 	os.Remove(storage.DefaultTestDbPath)
// 	publicKey, privateKey, err := edwards25519.GenerateKey(nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	db, err := storage.OpenDatabase(storage.DefaultTestDbPath)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer db.Close()
// 	// Test NewBlockchain

// 	// bc,err := InitBlockchain(db,)
// 	genesis, _ := NewGenesisBlock(publicKey, 0, nil)
// 	genesis.ResetTimestamp(0)
// 	genesis.ResetHash()
// 	genesis.Sign(privateKey)

// 	bc, err := InitBlockchain(db, genesis)

// 	// bc, err := CreateBlockchain(db, publicKey, privateKey) //Also creates the genesis block
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	// genesis, _ := NewGenesisBlock(publicKey, 0)
// 	// genesis.Sign(privateKey)

// 	// bc.Append([]*Block{block})
// 	parent := genesis
// 	fmt.Println("Main fork heoght0:", bc.GetMainForkHeight())

// 	for i := int64(0); i < 100; i += 5 { //adding 100 deals in the blockchain
// 		testDeal := make([]*pb.Deal, 5)
// 		for j := int64(1); j <= 5; j++ {
// 			d := MakeTestDeal(i + j)
// 			testDeal[j-1] = d
// 		}

// 		block := CreateNewBlock(parent, publicKey, privateKey, nil, nil, testDeal, nil, 0, 0) //Extra transaction variable as comp to MakeTestBlock

// 		block.ResetTimestamp(i + 5)
// 		block.ResetHash()
// 		block.Sign(privateKey)

// 		err = bc.Append([]*Block{block})
// 		if err != nil {
// 			t.Log(err)
// 		}
// 		// fmt.Println("block height: ", block.Height())
// 		parent = block
// 		// fmt.Println(block.GetTimestamp())
// 	}

// 	fmt.Println("Main fork heoght2:", bc.GetMainForkHeight())

// 	for i := int64(1); i < 100; i++ {
// 		tDeal, _, err := bc.BinaryDealSearch(i)

// 		if tDeal == nil {
// 			t.Fatal("Deal not found")
// 		}
// 		t.Log("deal found")
// 		if err != nil {
// 			t.Error(err)
// 		}
// 	}

// 	// type fields struct {
// 	// 	db            *storage.Database
// 	// 	genesis       *Block
// 	// 	tails         []*Block
// 	// 	forkLengths   []int
// 	// 	mainTailIndex int
// 	// 	mainForkMap   map[int64][]byte
// 	// }
// 	// type args struct {
// 	// 	ts int64
// 	// }
// 	// tests := []struct {
// 	// 	name    string
// 	// 	fields  fields
// 	// 	args    args
// 	// 	want    *pb.Deal
// 	// 	want1   int64
// 	// 	wantErr bool
// 	// }{
// 	// 	// TODO: Add test cases.
// 	// }
// 	// for _, tt := range tests {
// 	// 	t.Run(tt.name, func(t *testing.T) {
// 	// 		bc := &Blockchain{
// 	// 			db:            tt.fields.db,
// 	// 			genesis:       tt.fields.genesis,
// 	// 			tails:         tt.fields.tails,
// 	// 			forkLengths:   tt.fields.forkLengths,
// 	// 			mainTailIndex: tt.fields.mainTailIndex,
// 	// 			mainForkMap:   tt.fields.mainForkMap,
// 	// 		}
// 	// 		got, got1, err := bc.DealSearch(tt.args.ts)
// 	// 		if (err != nil) != tt.wantErr {
// 	// 			t.Errorf("Blockchain.DealSearch() error = %v, wantErr %v", err, tt.wantErr)
// 	// 			return
// 	// 		}
// 	// 		if !reflect.DeepEqual(got, tt.want) {
// 	// 			t.Errorf("Blockchain.DealSearch() got = %v, want %v", got, tt.want)
// 	// 		}
// 	// 		if got1 != tt.want1 {
// 	// 			t.Errorf("Blockchain.DealSearch() got1 = %v, want %v", got1, tt.want1)
// 	// 		}
// 	// 	})
// 	// }
// }

// func TestBlockchain_LinearDealSearch(t *testing.T) {
// 	os.Remove(storage.DefaultTestDbPath)
// 	publicKey, privateKey, err := edwards25519.GenerateKey(nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	db, err := storage.OpenDatabase(storage.DefaultTestDbPath)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer db.Close()
// 	// Test NewBlockchain

// 	// bc,err := InitBlockchain(db,)
// 	genesis, _ := NewGenesisBlock(publicKey, 0, nil)
// 	genesis.ResetTimestamp(0)
// 	genesis.ResetHash()
// 	genesis.Sign(privateKey)

// 	bc, err := InitBlockchain(db, genesis)

// 	// bc, err := CreateBlockchain(db, publicKey, privateKey) //Also creates the genesis block
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	// genesis, _ := NewGenesisBlock(publicKey, 0)
// 	// genesis.Sign(privateKey)

// 	// bc.Append([]*Block{block})
// 	parent := genesis

// 	for i := int64(0); i < 100; i += 5 { //adding 100 deals in the blockchain
// 		testDeal := make([]*pb.Deal, 5)
// 		for j := int64(1); j <= 5; j++ {
// 			d := MakeTestDeal(i + j)
// 			testDeal[j-1] = d
// 		}

// 		block := CreateNewBlock(parent, publicKey, privateKey, nil, nil, testDeal, nil, 0, 0) //Extra transaction variable as comp to MakeTestBlock

// 		block.ResetTimestamp(i + 5)
// 		block.ResetHash()
// 		block.Sign(privateKey)

// 		err = bc.Append([]*Block{block})
// 		if err != nil {
// 			t.Log(err)
// 		}
// 		parent = block
// 		// fmt.Println(block.GetTimestamp())
// 	}

// 	fmt.Println("Main fork heoght2:", bc.GetMainForkHeight())

// 	for i := int64(1); i < 100; i++ {
// 		tDeal, _, err := bc.LinearDealSearch(i)

// 		if tDeal == nil {
// 			t.Fatal("Deal not found")
// 		}
// 		t.Log("deal found")
// 		if err != nil {
// 			t.Error(err)
// 		}
// 	}
// }
