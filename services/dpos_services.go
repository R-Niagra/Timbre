package services

import (

	// "math"

	"errors"
	"fmt"
	"math"
	"time"

	"github.com/guyu96/go-timbre/core"
	"github.com/guyu96/go-timbre/dpos"
	dposstate "github.com/guyu96/go-timbre/dpos/state"
	"github.com/guyu96/go-timbre/log"
	"github.com/guyu96/go-timbre/net"
	"github.com/guyu96/go-timbre/wallet"
	// pb "github.com/guyu96/go-timbre/protobuf"
)

var (
	// pbRound int64 = 0
	nextTs       int64 = 0
	ErrStaleTail       = errors.New("Stale Main-Tail")
)

//StartDpos starts the DPOS consensus protocol.
func (s *Services) StartDpos() error {
	node := s.node

	if node.Dpos.HasStarted == true {
		log.Info().Msgf("Dpos has already started can't start now")
		return nil
	}

	revoltTip := false //If block is the tip of revolt
	if core.DefaultConfig.Revolt {
		tailBlock := s.node.Bc.GetMainTail()
		if tailBlock.Height() == core.DefaultConfig.RevoltHeight {
			revoltTip = true
			fmt.Println("Got revolt tip. Startin dpos normally")
		}
	}

	log.Info().Msgf("Starting DPos...")

	if s.node.Bc.IsEmpty() != true && !revoltTip { //That means that this node has synced before
		log.Info().Msgf("starting DPOS after the state catch-up")

		//Getting the latest block
		s.node.Dpos.SetRoundTimeOffset(0)
		tailBlock := s.node.Bc.GetMainTail()
		tailBlockTs := tailBlock.GetTimestamp() / (1e9) //in seconds
		tailRound := tailBlock.GetRoundNum()

		curTime := time.Now().Unix()           //Assuming ur UNIX time is correct.
		timeSinceTail := curTime - tailBlockTs //in seconds
		fmt.Println("TimeSinceTail: ", timeSinceTail)
		if timeSinceTail > 2*int64(s.node.Dpos.GetTickInterval().Seconds()) {
			log.Warn().Err(errors.New("Curtime and last block time has big difference. SYNC again"))
			go s.node.Syncer.SyncBlockchain() //Should initiate resync chain protocol
			//review this change
			log.Info().Msgf("Current-time is way ahead tail. Need to sync again")
			return ErrStaleTail

		}
		timeLeft := s.TimeLeftInRoundBySlots(tailBlock) //Gives the timeLeft form time.Now()

		if timeLeft <= 2 { //To increase tolerance
			if timeLeft > 0 {
				time.Sleep(2 * time.Second)
			}
			node.Dpos.AdvanceRound(time.Now().UnixNano(), int64(s.node.Bc.Length()))
			s.node.Dpos.SetRoundStartTime(s.RoundStartTime(s.node.Dpos.GetRoundNum())) //in seconds

			if s.node.Dpos.GetRoundNum() != tailRound+1 {
				fmt.Println("DPos round- Expected: ", s.node.Dpos.GetRoundNum(), tailRound+1)
				negComTime := -int64(s.node.Dpos.GetCommitteeTime().Seconds())
				fmt.Println("negComTime", negComTime)
				if timeLeft < negComTime {
					return errors.New("Need to sync again. Sync time was bigger than round")
				} else {
					panic("Tail round isn't +1. Weird situation")
				}
			}
		} else {
			log.Info().Msgf("Continuing the current round for %v", timeLeft)
			s.node.Dpos.SetRoundStartTime(s.RoundStartTime(s.node.Dpos.GetRoundNum())) //in seconds
			fmt.Println("ROUND START:- ", s.node.Dpos.GetRoundStartTime(), "RN: ", s.node.Dpos.GetRoundNum())

			if s.node.Dpos.GetRoundNum() != tailRound {
				//THis condition shouldn't hit
				panic("Not in correct round")
			}
		}

		log.Info().Msgf("TIME HAS BEE SET. ")

		if tailRound != s.node.Dpos.GetRoundNum() {
			log.Info().Err(errors.New("Round nums of tail block and DPOS doesn't match"))
		}

		node.Dpos.Start()
		log.Info().Msgf("Node started and running")

		return nil
	}

	err := node.Dpos.Setup()
	if err != nil {
		return err
	}

	log.Info().Msgf("STARTING DPOS AT: %v", time.Now().Unix())
	node.Dpos.Start()
	log.Info().Msgf("Node started and running")

	return nil
}

//StartDpos2 starts the DPOS consensus protocol.
func (s *Services) StartDpos2(comSize int, minerRole bool) error {
	node := s.node

	if s.node.Bc.IsEmpty() != true { //That means that this node has synced before
		log.Info().Msgf("starting DPOS after the state catch-up")

		//Getting the latest block
		s.node.Dpos.SetRoundTimeOffset(0)
		tailBlock := s.node.Bc.GetMainTail()
		tailBlockTs := tailBlock.GetTimestamp() / (1e9) //in seconds
		tailRound := tailBlock.GetRoundNum()
		log.Info().Msgf("Tail block height: %v", tailBlock.Height(), "tail round: %v", tailRound)
		curTime := time.Now().Unix()           //Assuming ur UNIX time is correct.
		timeSinceTail := curTime - tailBlockTs //in seconds
		if timeSinceTail > 2*int64(s.node.Dpos.GetMinerSlotTime().Seconds()) {
			log.Info().Err(errors.New("Curtime and last block time has big difference. SYNC again"))
			return errors.New("Current-time is way ahead tail. Need to sync again")

		}

		timeLeft := s.TimeLeftInRoundBySlots(tailBlock) //Gives the timeLeft form time.Now()

		if timeLeft <= int64(s.node.Dpos.GetTickInterval().Seconds()-1) { //To increase tolerance (-2)
			count := 0
			for timeLeft < int64(s.node.Dpos.GetTickInterval().Seconds()-1) {
				fmt.Println("Wait for the next round... ", timeLeft)
				time.Sleep(3 * time.Second) //Sleeping for some time
				tailBlock = s.node.Bc.GetMainTail()
				tailBlockTs = tailBlock.GetTimestamp() / (1e9) //in seconds
				tailRound = tailBlock.GetRoundNum()
				timeLeft = s.TimeLeftInRoundBySlots(tailBlock) //Gives the timeLeft form time.Now()
				count++
				if count > 5 && timeLeft > 2 { //In rare condition when only one miner is mining and second one tries to join but gets first slot to mine

					break
				}
			}

		}
		log.Info().Msgf("Continuing the round for ", timeLeft)
		s.node.Dpos.SetRoundStartTime(s.RoundStartTime(s.node.Dpos.GetRoundNum())) //in seconds
		fmt.Println("ROUND START:- ", s.node.Dpos.GetRoundStartTime(), "RN: ", s.node.Dpos.GetRoundNum())

		if s.node.Dpos.GetRoundNum() != tailRound {
			panic("Not in correct round")
			fmt.Println("Resetting the round num")
			node.Dpos.SetRoundNum(tailBlock.GetRoundNum())
		}

		if tailRound != s.node.Dpos.GetRoundNum() {
			log.Info().Err(errors.New("Round nums of tail block and DPOS doesn't match"))
		}

		node.Dpos.Start()

		log.Info().Msgf("Node started and running")
		// log.Info().Msgf("Requesting to register me as candidate")

		return nil
	}

	err := node.Dpos.Setup()
	if err != nil {
		return err
	}

	fmt.Println("STARTING DPOS AT: ", time.Now().Unix())
	node.Dpos.Start()
	log.Info().Msgf("Node started and running")
	time.Sleep(200 * time.Millisecond)
	log.Info().Msgf("Announcing reg-Cand !")

	if s.node.Bc.IsEmpty() && minerRole == true { //Only register without transaction if it hasn't synced up
		go func() {
			for i := 0; i < 5; i++ { //Announcing candidacy 3 times
				s.AnnounceCandidacy()
				time.Sleep(1 * time.Second)
			}
			// err = s.DoDelegateRegisteration() //Also adding the transaction- To establish as a ground truth
		}()
	}
	return nil
}

//TimeLeftInRoundBySlots returns the time left in the round from time.Now() using the sum of all the slots passed
func (s *Services) TimeLeftInRoundBySlots(b *core.Block) int64 {
	bRn := b.GetRoundNum()
	gts := s.node.Bc.Genesis().GetTimestamp() / (1e9) //Genesis timestamp in seconds
	chainStartTime := gts - int64(s.node.Dpos.GetTickInterval().Seconds())
	log.Info().Msgf("Chain start time: ", chainStartTime)
	curTime := time.Now().Unix()
	slotsPassed, err := s.node.Dpos.SlotsPassedByRound(bRn)
	if err != nil {
		panic(err.Error())
	}

	//Round end time in which this block is mined
	endTime := chainStartTime + slotsPassed*int64(s.node.Dpos.GetMinerSlotTime().Seconds())
	return endTime - curTime
}

//RoundStartTime returns the round start time
func (s *Services) RoundStartTime(round int64) int64 {
	gts := s.node.Bc.Genesis().GetTimestamp() / (1e9) //Genesis timestamp in seconds
	chainStartTime := gts - int64(s.node.Dpos.GetTickInterval().Seconds())
	slotsPassed, err := s.node.Dpos.SlotsPassedByRound(round - 1)
	if err != nil {
		panic(err.Error())
	}
	roundStart := chainStartTime + slotsPassed*int64(s.node.Dpos.GetMinerSlotTime().Seconds())

	return roundStart
}

//RoundNum used to round numbers
func RoundNum(x, unit float64) float64 {
	return math.Round(x/unit) * unit
}

//timeLeftInRound returns the time left(in seconds) in the round based on the blocks on the blockchain
//TimeLeftInRound returns the time left in the round
func (s *Services) timeLeftInRound(block *core.Block) (bool, float64) {
	sameMiner := false
	bTs := block.GetTimestamp()
	bRound := block.GetRoundNum()
	bPk := block.MinerPublicKey()

	minerInd, exists := s.node.Dpos.GetMinerIndex(bRound, bPk.String())
	if exists == false {
		log.Info().Err(errors.New("Miner wasn't allowed to mine in this round"))
	}

	minerSlotsPassed := 1 //Counting this block as the first produced by this miner
	blocksPerMiner := s.node.Dpos.BlocksPerMiner()
	minerSlotTime := s.node.Dpos.GetMinerSlotTime()
	blockInterval := s.node.Dpos.GetTickInterval().Nanoseconds()

	for i := 0; i < blocksPerMiner; i++ { //Looping over blocksPerMiner+1 just to ensure that a miner hasn't produced block more than it was allowed
		phash := block.ParentHash()
		pBlock, err := s.node.Bc.GetBlockByHash(phash)
		if err != nil {
			log.Info().Err(err)
			break
		}
		pPk := pBlock.MinerPublicKey()
		pRound := block.GetRoundNum()
		pTs := pBlock.GetTimestamp()
		bTimeDiff := bTs - pTs

		slotDiff := int(math.Round(float64(bTimeDiff) / float64(s.node.Dpos.GetTickInterval().Nanoseconds()))) //TODO:- Check if round is the right func to use
		if pPk.String() != bPk.String() || pRound != bRound {
			//This is the only block produced by the miner
			if pRound != bRound {
				_, pRoundTimeLeft := s.timeLeftInRound(pBlock)
				slotDiff = int(math.Round((float64(bTimeDiff) - pRoundTimeLeft*1e9) / float64(s.node.Dpos.GetTickInterval().Nanoseconds())))
			}
			if pRound == bRound && pPk.String() != bPk.String() {
				slotDiff = int(math.Round((float64(bTimeDiff) - s.node.Dpos.RoundToleranceTime()*1e9) / float64(s.node.Dpos.GetTickInterval().Nanoseconds())))
			}

			minerSlotsPassed += slotDiff - 1
			break
		}
		if minerSlotsPassed > blocksPerMiner {
			log.Info().Err(errors.New("Miner prodecued more blocks than allowed"))
			break
		}

		//Handles the case if a miner produce block then doesn't produce block and finally produce one
		if bTimeDiff > blockInterval+1*1e9 { //giving 1 secconds of buffer
			i += slotDiff - 1
			minerSlotsPassed += slotDiff - 1
		}
		if bTimeDiff < blockInterval-1*1e9 { //giving 1 secconds of buffer
			log.Info().Err(errors.New("miner is producing blocks before time/spammming"))
		}
		minerSlotsPassed++

		bPk = pPk
		bRound = pRound
		bTs = pTs
		block = pBlock

	}
	if minerSlotsPassed > blocksPerMiner {
		minerSlotsPassed = blocksPerMiner
	}
	minersToCome := s.node.Dpos.TotalMinersInRound(bRound) - minerInd - 1

	nextMinersTime := minerSlotTime.Seconds() * float64(minersToCome)
	if minerSlotsPassed >= blocksPerMiner {
		return sameMiner, nextMinersTime + s.node.Dpos.RoundToleranceTime()
	}

	sameMiner = true
	curMinerTime := (float64(blocksPerMiner-minerSlotsPassed))*s.node.Dpos.GetTickInterval().Seconds() + s.node.Dpos.RoundToleranceTime()
	timeTillRoundEnd := nextMinersTime + curMinerTime
	return sameMiner, timeTillRoundEnd
}

//AnnounceCandidacy broadcasts the candidate registration announcement
func (s *Services) AnnounceCandidacy() error {
	node := s.node
	nodeAdd := node.PublicKey().String()
	myCand, err := node.Dpos.State.RegisterCand(nodeAdd)

	if err != nil {

		myCand, _ = s.node.Dpos.State.GetCandidate(nodeAdd)
	}

	err = s.node.Wm.SetCandidacy(nodeAdd)
	if err != nil {
		fmt.Println(err.Error())
	}

	candBytes, err := myCand.ToBytes()
	if err != nil {
		return err
	}
	msg := net.NewMessage(net.MsgCandidateRegisteration, candBytes)
	node.Broadcast(msg)
	return nil
}

//HandleRegisterCandidate registers the candidate
func (s *Services) HandleRegisterCandidate(msg net.Message) {
	if s.node.Bc.IsEmpty() != true { //Check if the blockchain has already started
		log.Info().Err(errors.New("Cannot register without Tx"))
		return
	}

	newCand, err := dposstate.FromBytes(msg.Data)
	if err != nil {
		log.Info().Msgf(err.Error())
	}
	err = s.node.Dpos.State.PutCandidate(newCand.ID, newCand)
	if err != nil {
		return
	}
	s.node.Wm.CheckAndPutWalletByAddress(newCand.Address, wallet.NewWallet(newCand.Address))
	cWallet, err := s.node.Wm.GetWalletByAddress(newCand.Address)
	if err != nil {
		fmt.Errorf("Cand wallet doesn't exists")
		return
	}
	cWallet.SetCandidate(true) //Setting the isCandidate field of wallet as true

	log.Info().Msgf("A candidate has been successfully added to the state!!")
}

//UpdateDpos updates the dpos
func (s *Services) UpdateDpos(block *core.Block, d *dpos.Dpos) error { //Take round by round jump

	//Every block shouldn't update the state
	nrTs := new(int64) //next Round timestamp(start)
	var prevMiner string = ""
	var pbRound int64 = 0
	var pHeight int64 = 0

	if block.IsGenesis() == false { //For the non-genesis block
		parentHash := block.ParentHash()
		pBlock, err := s.node.Bc.GetBlockByHash(parentHash)
		if err != nil {
			log.Info().Err(err)
			log.Info().Err(errors.New("Not syncing in order!!!!!"))
			return errors.New("Can't fetch parent")
		}
		pbRound = pBlock.GetRoundNum()
		pHeight = pBlock.Height()
		prevMiner = pBlock.MinerPublicKey().String()
	}

	cbRound := block.GetRoundNum() //Current block round
	//Since it is updating so it might not receive the register candidate announcement thus for every vote it registers the candidates in the store
	minerAdd := block.MinerPublicKey().String()

	if cbRound == 1 && minerAdd != prevMiner { //THis is corrosponding to the first round where genesis block along with other blocks will be created
		if block.IsGenesis() == true {
			d.SetCommitteeSize(0) //Should start with zero
		}
		d.State.AddGenesisRoundCand(minerAdd, minerAdd)                           //For now using add and ID as the same TODO: change second argument to the ID
		d.SetRoundNum(0)                                                          //Advance round will increment the number since it is called in the start of the round
		nextTs = block.GetTimestamp() - d.GetTickInterval().Nanoseconds() + 2*1e9 //DPOS start time with 2 second of the gap

		log.Info().Msgf("Advancing GENESIS ROUND")
		d.ResetRoundTimeOffset() //Time offset should be set to zero coz if might have been called before
		*nrTs = d.AdvanceRound(nextTs, pHeight)
		if *nrTs == 0 {
			log.Info().Err(errors.New("Invalid round. Committee size is 0"))
			return errors.New("Invalid round. Committee size is 0")
		}
		pbRound++
		nextTs = *nrTs
		return nil
	}

	//Check if DPOS roundnumber is same as the block parent, if not then reset it
	if pbRound != 0 && d.GetRoundNum() != pbRound {
		parentHash := block.ParentHash()
		pBlock, err := s.node.Bc.GetBlockByHash(parentHash)
		if err != nil {
			log.Error().Msgf(err.Error())
			return err
		}
		d.RevertDposTill(pBlock.GetRoundNum())
	}

	if pbRound == cbRound {
		return nil
	}

	for ; cbRound > pbRound; pbRound++ { //Should only advance round if block roundNum is greater than the previous block round numround

		*nrTs = d.AdvanceRound(nextTs, pHeight)
		if *nrTs == 0 {
			log.Info().Err(errors.New("Invalid round. Committee size is 0"))
		}
		nextTs = *nrTs
	}

	return nil
}
