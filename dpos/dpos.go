package dpos

// package dpos
import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	dposstate "github.com/guyu96/go-timbre/dpos/state"
	"github.com/guyu96/go-timbre/log"
	pb "github.com/guyu96/go-timbre/protobuf"
	"github.com/guyu96/go-timbre/wallet"
)

// DPOS constants
const (
	miningTickInterval          = 5 * time.Second
	candidatesFilePath          = "bootstrap_committee.txt"
	minerSlotTime               = 10 * time.Second
	blockPrepTime               = 2 * time.Second //Time before tick when miner should start to prepare block
	defaultCommitteeSize        = 2
	minComSize           uint32 = 1
	maxComSize                  = 15
	InitComSize                 = 2
)

// Dpos structure
type Dpos struct {
	committeeSize      uint32
	HasStarted         bool
	committee          []*dposstate.Candidate //Committee from second last round
	nextRoundCommittee []*dposstate.Candidate //Committee members of the next round from the last round
	quitChT            chan int
	quitChL            chan int
	roundNumber        int64            //Current round number
	State              *dposstate.State //Current state which keeps updated based on vote tx in the current round
	startRound         chan uint32
	roundEnded         chan struct{}

	miningSignal    chan struct{}
	listeningSignal chan struct{}
	prepBlockSig    chan struct{}
	createBlock     chan struct{}

	curMinerAdd                string
	nodeAddress                string //Only required for testing purpose
	TimeOffset                 time.Duration
	roundTimeOffset            float64
	firstBlockProd             bool
	MinerCache                 []*MinersInRound
	wallets                    wallet.WalletAccess
	bcHeight                   int64
	roundStartTime             int64 //Unix value of the round start time
	NumberOfToMineBlockPerSlot int
	bc                         BlockchainAccess
}

//MinersInRound is the data required in every round
type MinersInRound struct {
	Miners      []string
	Round       int64
	SlotsPassed int64
}

//GetMinerListeningChannel returns the listening recieve-channel for the miner
func (d *Dpos) GetMinerListeningChannel() <-chan struct{} {
	return d.listeningSignal
}

//GetMinerPreparingChannel returns the prepare recieve-channel for the miner
func (d *Dpos) GetMinerPreparingChannel() <-chan struct{} {
	return d.prepBlockSig
}

//GetMinerMiningChannel returns the mining recieve-channel for the miner
func (d *Dpos) GetMinerMiningChannel() <-chan struct{} {
	return d.miningSignal
}

//GetMinerCreateBlockChannel gets the create block channel for the miner
func (d *Dpos) GetMinerCreateBlockChannel() <-chan struct{} {
	return d.createBlock
}

//AddOrUpdateMinerEntry either add or update existing miner entry per round
func (d *Dpos) AddOrUpdateMinerEntry(miners []string, round int64) {

	//if it is the first entry
	if len(d.MinerCache) == 0 {
		entry := &MinersInRound{
			Miners:      miners,
			Round:       round,
			SlotsPassed: int64(len(miners)),
		}
		d.MinerCache = append(d.MinerCache, entry)
		return
	}

	lastEntry := d.MinerCache[len(d.MinerCache)-1]
	//New entry round should always be greater than the previous
	if lastEntry.Round >= (round) {
		d.DeleteEntryInMinerCache(round)
		if len(d.MinerCache) == 0 {
			entry := &MinersInRound{
				Miners:      miners,
				Round:       round,
				SlotsPassed: int64(len(miners)),
			}
			d.MinerCache = append(d.MinerCache, entry)
			return
		}
		lastEntry = d.MinerCache[len(d.MinerCache)-1]
	} else if lastEntry.Round < (round - 1) {
		panic("Some entry is missing")
	}
	//Compare the current miners from last. if same then update the last entry
	same := false
	if len(miners) == len(lastEntry.Miners) {
		same = true
		for i := 0; i < len(miners); i++ {
			if miners[i] != lastEntry.Miners[i] {
				same = false
				break
			}
		}
	}

	if same {
		lastEntry.Round++
		lastEntry.SlotsPassed += int64(len(miners))
	} else {
		entry := &MinersInRound{
			Miners:      miners,
			Round:       round,
			SlotsPassed: lastEntry.SlotsPassed + int64(len(miners)),
		}
		d.MinerCache = append(d.MinerCache, entry)
	}

	return
}

//DeleteEntryInMinerCache deletes the entry from the tail
func (d *Dpos) DeleteEntryInMinerCache(round int64) error {

	if len(d.MinerCache) == 0 {
		return errors.New("No entry")
	}

	tail := d.MinerCache[len(d.MinerCache)-1]
	if tail.Round < round {
		return errors.New("Tail round smaller")
	}

	last := len(d.MinerCache) - 1
	secondLast := last - 1

	for {
		if secondLast < 0 {
			//make changes to the last one
			if (round - 1) == 0 {
				d.MinerCache = []*MinersInRound{} //Emptying cache if last 1st entry is to be removed
				break
			}
			fmt.Println("second < 0")
			numRounds := d.MinerCache[last].Round - (round - 1)
			fmt.Println("NUM of rounds: ", numRounds)
			numSlots := int64(len(d.MinerCache[last].Miners)) * numRounds
			fmt.Println("num of slots: ", numSlots)
			d.MinerCache[last].Round = round - 1
			d.MinerCache[last].SlotsPassed -= numSlots
			break
		}

		lastRound := d.MinerCache[last].Round
		secondLastRound := d.MinerCache[secondLast].Round

		if round > secondLastRound && round <= lastRound {
			if secondLastRound == (round - 1) {
				d.MinerCache = d.MinerCache[:secondLast+1]
				break
			}

			numRounds := d.MinerCache[last].Round - (round - 1)
			fmt.Println("NUM of rounds to del: ", numRounds)
			numSlots := int64(len(d.MinerCache[last].Miners)) * numRounds
			fmt.Println("num of slots: ", numSlots)
			d.MinerCache[last].Round = round - 1
			d.MinerCache[last].SlotsPassed -= numSlots
			break

		} else if round < lastRound && round <= secondLastRound {
			fmt.Println("len before del: ", len(d.MinerCache), last, secondLast)
			d.MinerCache = d.MinerCache[:secondLast+1] //deleting the last entry
			fmt.Println("len after del: ", len(d.MinerCache), last, secondLast)
		}

		last--
		secondLast--
	}

	return nil
}

//MinersInRound returns the miner in the round
func (d *Dpos) MinersInRound(round int64) ([]string, error) {

	if round <= 0 || len(d.MinerCache) == 0 || d.MinerCache[len(d.MinerCache)-1].Round < round {
		return []string{}, errors.New("No entry in the cache")
	}

	ind := len(d.MinerCache) - 1
	for ; ind >= 0; ind-- {
		if ind > 0 && (round > d.MinerCache[ind-1].Round && round <= d.MinerCache[ind].Round) {
			return d.MinerCache[ind].Miners, nil
		} else if ind == 0 && round <= d.MinerCache[ind].Round {
			return d.MinerCache[ind].Miners, nil
		}
	}
	return []string{}, errors.New("Round doesn't exists")
}

//SlotsPassedByRound returns the total slots passed till the round
func (d *Dpos) SlotsPassedByRound(round int64) (int64, error) {

	if round <= 0 {
		return 0, nil
	} else if d.MinerCache[len(d.MinerCache)-1].Round < round {
		return 0, errors.New("No entry in the cache")
	}

	ind := len(d.MinerCache) - 1
	for ; ind >= 0; ind-- {
		if ind > 0 && (round > d.MinerCache[ind-1].Round && round <= d.MinerCache[ind].Round) {
			toSub := (d.MinerCache[ind].Round - round) * int64(len(d.MinerCache[ind].Miners))
			return d.MinerCache[ind].SlotsPassed - toSub, nil
		} else if ind == 0 && round <= d.MinerCache[ind].Round {
			toSub := (d.MinerCache[ind].Round - round) * int64(len(d.MinerCache[ind].Miners))
			return d.MinerCache[ind].SlotsPassed - toSub, nil
		}
	}
	return 0, errors.New("Round doesn't exists")
}

//GetTailSlotsPassed return the slot passed in the tail round
func (d *Dpos) GetTailSlotsPassed() int64 {
	s, _ := d.SlotsPassedByRound(d.GetRoundNum())
	return s
}

// New returns dpos consensus.
func New(comSize uint32, address string, timeOffset time.Duration, walletsAccess wallet.WalletAccess, bcAccess BlockchainAccess) *Dpos {

	newState := dposstate.NewDposState()
	return &Dpos{
		committeeSize:              comSize,
		HasStarted:                 false,
		quitChT:                    make(chan int),
		quitChL:                    make(chan int),
		roundNumber:                0, //Round number starts with zero
		State:                      newState,
		startRound:                 make(chan uint32),
		roundEnded:                 make(chan struct{}),
		miningSignal:               make(chan struct{}),
		listeningSignal:            make(chan struct{}),
		prepBlockSig:               make(chan struct{}),
		createBlock:                make(chan struct{}),
		nodeAddress:                address,
		TimeOffset:                 timeOffset,
		roundTimeOffset:            0,
		bcHeight:                   0,
		roundStartTime:             0,
		wallets:                    walletsAccess,
		NumberOfToMineBlockPerSlot: int(minerSlotTime / miningTickInterval),
		bc:                         bcAccess,
	}

}

//SetBlockchain sets the blockchain in the DPOS
func (d *Dpos) SetBlockchain(bcAccess BlockchainAccess) {
	d.bc = bcAccess
}

//BlockPrepTime returns time before tick when miner should start preparing the block
func (d *Dpos) BlockPrepTime() time.Duration {
	return blockPrepTime
}

//TotalRoundTime returns the total time of the round
func (d *Dpos) TotalRoundTime(round int64) float64 {
	// if miners, ok := d.MinersByRound[round]; ok {
	miners, err := d.MinersInRound(round)
	if err != nil {
		return 0
	}

	roundMiners := len(miners)
	return minerSlotTime.Seconds() * float64(roundMiners)

}

//TotalCurRoundDuration returns the total CurRoundDuration
func (d *Dpos) TotalCurRoundDuration() float64 {
	return minerSlotTime.Seconds() * float64(len(d.committee))
}

//SetRoundStartTime sets the current round start time
func (d *Dpos) SetRoundStartTime(ts int64) {
	d.roundStartTime = ts
}

//GetRoundStartTime returns the round start time
func (d *Dpos) GetRoundStartTime() int64 {
	return d.roundStartTime
}

//GetCurMinerAdd returns the miner at the moment
func (d *Dpos) GetCurMinerAdd() string {
	return d.curMinerAdd
}

func (d *Dpos) GetComitteeSize() uint32 {
	return d.committeeSize
}

//SetBcHeight the height of the blockchain in dpos
func (d *Dpos) SetBcHeight(height int64) {
	d.bcHeight = height
}

//GetBcHeight return the height of the blockchain in dpos
func (d *Dpos) GetBcHeight() int64 {
	return d.bcHeight
}

//GetRoundTimeOffset returns the time offset in round
func (d *Dpos) GetRoundTimeOffset() float64 {
	return d.roundTimeOffset
}

//SetRoundTimeOffset sets the roundTimeoffset
func (d *Dpos) SetRoundTimeOffset(val float64) {
	d.roundTimeOffset = val
}

//ResetRoundTimeOffset resets the roundTimeOffset to zero
func (d *Dpos) ResetRoundTimeOffset() {
	d.roundTimeOffset = 0
}

//GetMinComSize returns the minimum committe size as set in the const
func (d *Dpos) GetMinComSize() uint32 {
	return minComSize
}

//GetMinerSlotTime returns the slot time of the miners
func (d *Dpos) GetMinerSlotTime() time.Duration {
	return minerSlotTime
}

//GetCommitteeTime returns the committe duration
func (d *Dpos) GetCommitteeTime() time.Duration {
	return minerSlotTime * time.Duration(d.committeeSize)
}

//GetRoundNum returns the current round number
func (d *Dpos) GetRoundNum() int64 {
	return d.roundNumber
}

//SetRoundNum sets the round number
func (d *Dpos) SetRoundNum(num int64) {
	d.roundNumber = num
}

//GetTickInterval returns the tick interval b/w blocks
func (d *Dpos) GetTickInterval() time.Duration {
	return miningTickInterval
}

//SetCommitteeSize sets the size of the committee size
func (d *Dpos) SetCommitteeSize(size uint32) {
	d.committeeSize = size
}

//GetCommitteeSize returns the committee size of the committee
func (d *Dpos) GetCommitteeSize() uint32 {
	return d.committeeSize
}

// getCandidatesFromFile read candidates for bootstrapping
func (d *Dpos) getCandidatesFromFile(filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	id := 1
	r := bufio.NewReader(f)

	for {

		l, err := r.ReadString('\n')
		l = strings.TrimSuffix(l, "\n")

		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		d.committee = append(d.committee, &dposstate.Candidate{
			ID:        strconv.Itoa(id),
			Address:   l,
			VotePower: 0,
			TimeStamp: time.Now().Unix(),
		})
		id++
	}
	return nil
}

//GetMinerIndex returns the position of miner in the round commitee
func (d *Dpos) GetMinerIndex(round int64, pk string) (int, bool) {

	miners, err := d.MinersInRound(round)
	if err != nil {
		return 0, false
	}
	for i, miner := range miners {
		if miner == pk {
			return i, true
		}
	}

	return 0, false
}

//TotalMinersInRound returns the number of miners in the round
func (d *Dpos) TotalMinersInRound(round int64) int {

	miners, err := d.MinersInRound(round)
	if err != nil {
		return 0
	}
	return len(miners)
}

//StartDummyCommittee just put dummy candidates with dummy addresses to the current committee
func (d *Dpos) StartDummyCommittee(memCount int) {

	log.Info().Msgf("Creating dummy bootstrap committee")
	startPort := 7000
	l := "127.0.0.1:"
	for i := 0; i < memCount; i++ {
		add := l + strconv.Itoa(startPort)

		d.committee = append(d.committee, &dposstate.Candidate{
			ID:        strconv.Itoa(i),
			Address:   add,
			VotePower: 0,
			TimeStamp: time.Now().Unix(),
		})
		startPort++
	}

}

//BootstrapCommittee empties the committee and then bootstraps the committee with the set of miners given
func (d *Dpos) BootstrapCommittee(members []string) {
	newCommittee := []*dposstate.Candidate{} //Empties the committee

	for _, member := range members {
		d.committee = append(d.committee, &dposstate.Candidate{
			ID:        member,
			Address:   member,
			VotePower: 0,
			TimeStamp: time.Now().Unix(),
		})
	}
	d.State.InitializeStateWithMembers(newCommittee)
}

// Start DPOS
func (d *Dpos) Start() {
	if d.HasStarted == true {
		log.Info().Msgf("Dpos has already started")
		return
	}
	d.HasStarted = true
	go d.roundTracker()
	go d.loop()
	d.startRound <- uint32(len(d.committee)) //First light weight signal to start the round
}

// Stop DPOS
func (d *Dpos) Stop() {
	if !d.HasStarted {
		return
	}
	d.quitChL <- 1
	d.quitChT <- 1
	d.HasStarted = false
}

//StopAndClean stops the dpos and clean the object
func (d *Dpos) StopAndClean() {
	d.Stop()
	//closing all the channels
	close(d.roundEnded)
	close(d.miningSignal)
	close(d.listeningSignal)
	close(d.prepBlockSig)
	close(d.quitChT)
	close(d.quitChL)
	close(d.createBlock)
}

// Setup sets up up dpos from file
func (d *Dpos) Setup() error {

	comSize := InitComSize
	d.StartDummyCommittee(comSize)

	return nil
}

//roundTracker ensures the
func (d *Dpos) roundTracker() {

	for {
		select {

		case <-d.roundEnded:
			fmt.Printf("round no: %v has successfully ended \n", d.roundNumber)
			d.TransitionRound()

			d.roundNumber++
			log.Info().Msgf("Starting next round %v", d.roundNumber)
			d.startRound <- d.committeeSize

		case <-d.quitChT:
			fmt.Println("Stopped Dpos RT.")
			return

		}
	}

}

//InitializeComMembers resets the current committe members to the given miners
func (d *Dpos) InitializeComMembers(miners, nextRoundMiners []string) {
	//First check if the member is in the DPOSState
	d.committee = []*dposstate.Candidate{}
	for _, miner := range miners {
		cand, err := d.State.GetCandidate(miner)
		if err != nil {
			log.Info().Err(errors.New("Com cannot be initialized with the said member"))
			continue
		}
		candidate := *cand
		d.committee = append(d.committee, &candidate)
	}

	d.nextRoundCommittee = []*dposstate.Candidate{}

	for _, miner := range nextRoundMiners {
		cand, err := d.State.GetCandidate(miner)
		if err != nil {
			log.Info().Err(errors.New("Next round com cannot be initialized with the said member"))
			continue
		}
		candidate := *cand
		d.nextRoundCommittee = append(d.nextRoundCommittee, &candidate)
	}

}

//EmptyCurrentCommittee empties the current miner committee
func (d *Dpos) EmptyCurrentCommittee() {
	d.committee = []*dposstate.Candidate{}
}

//RecomputeLastRoundCommittee reomputes the last round committee
func (d *Dpos) RecomputeLastRoundCommittee() {
	d.State.UpdateVotePower(d.wallets)
	committeeCand := d.State.SortByVotes(int64(d.bc.Length()))
	nextComSize := d.ComputeNextCommitteeSize(committeeCand)
	d.copyCandToNextRoundCommittee(committeeCand, nextComSize) //Copies cand to last round committe
}

//TransitionRound makes preparation for the transition to the next round
func (d *Dpos) TransitionRound() {

	d.State.UpdateVotePower(d.wallets)
	committeeCand := d.State.SortByVotes(int64(d.bc.Length()))
	nextComSize := d.ComputeNextCommitteeSize(committeeCand)

	if d.roundNumber == 0 {
		d.copyCandToNextRoundCommittee(committeeCand, nextComSize) //Copies cand to last round committe
	}

	d.populateFromNextRoundCom() //will copy the members of next round committee to current round committee

	d.copyCandToNextRoundCommittee(committeeCand, nextComSize) //Copies cand to last round committe
	d.committeeSize = uint32(len(d.committee))
	if d.committeeTime() == 0 {
		panic("No committee member to support network!!")
	}

	for _, cand := range d.committee {
		log.Info().Msgf("Cand: %v , votePower: %v", cand.Address, cand.VotePower)
	}

	if d.MiningEligibility(d.nodeAddress) == true {
		d.listeningSignal <- struct{}{}

	}

	d.roundStartTime = lastMiningSlot(time.Now().Unix() + 1).Unix() //adding two to be on safe side

	d.AddOrUpdateMinerEntry(d.GetMinersAddresses(), d.roundNumber+1)

}

//AdvanceRound advances to next round for catching up fast to most updated Dpos state
func (d *Dpos) AdvanceRound(startTime int64, bcHeight int64) int64 {
	d.roundNumber++

	d.State.UpdateVotePower(d.wallets)
	committeeCand := []*dposstate.Candidate{}
	if d.bc != nil {
		committeeCand = d.State.SortByVotes(int64(d.bc.Length()))
	} else {
		committeeCand = d.State.SortByVotes(bcHeight)
	}

	nextComSize := d.ComputeNextCommitteeSize(committeeCand)

	if d.roundNumber == 1 { //In Genesis initialize the last round committee with the current one
		d.copyCandToNextRoundCommittee(committeeCand, nextComSize) //Copies cand to last round committe
	}

	d.populateFromNextRoundCom() //will copy the members of last round to the second last round

	d.copyCandToNextRoundCommittee(committeeCand, nextComSize)
	d.committeeSize = uint32(len(d.committee))
	if d.committeeSize == 0 {
		log.Info().Err(errors.New("Zero committee size"))
		return 0
	}

	comDur := d.committeeTime()
	fmt.Println("committee duration: ", comDur)

	nextRoundTs := startTime + comDur.Nanoseconds()

	d.AddOrUpdateMinerEntry(d.GetMinersAddresses(), d.roundNumber)

	d.roundStartTime = startTime / 1e9
	fmt.Println("NEXT ROUND TS:- ", (nextRoundTs / (int64(time.Second) / int64(time.Nanosecond))), "roundStart", d.roundStartTime)

	return nextRoundTs
}

//ComputeNextCommitteeSize computes the committe of the sorted child candidates
func (d *Dpos) ComputeNextCommitteeSize(sortedCand []*dposstate.Candidate) uint32 {

	if d.State.GetTotalVoteStaked() == 0 || len(sortedCand) == 0 { //If no vote is casted then default size is 2
		comLen := uint32(len(sortedCand))
		d.State.SetChildCommitteeSize(comLen)

		if comLen <= maxComSize {
			return comLen
		} else if comLen > maxComSize {
			return maxComSize
		}

		return minComSize
	}

	var size uint32 = 0
	//Candidate must get atleast 10% of the total votepower to get entry in the committee
	for _, cand := range sortedCand {
		if float64(cand.VotePower) >= (float64(0.1) * float64(d.State.GetTotalVoteStaked())) {
			size++
		}
		if cand.VotePower == 0 { //Since slice is sorted by votepower no need to look further
			break
		}
	}

	if size > maxComSize {
		d.State.SetChildCommitteeSize(size)
		return maxComSize
	}

	d.State.SetChildCommitteeSize(size)
	return size
}

//MiningEligibility checks the eligibility of the candidate to mine by comparing current committee members
func (d *Dpos) MiningEligibility(add string) bool {
	for _, cand := range d.committee {
		if cand.Address == add {
			return true
		}
	}
	return false
}

//copyCandToNextRoundCommittee copy the pointer candidates into the committee from state candidates passed
func (d *Dpos) copyCandToNextRoundCommittee(committeeCand []*dposstate.Candidate, nextComSize uint32) {
	d.nextRoundCommittee = []*dposstate.Candidate{} //Emptying the current slice
	for index, cand := range committeeCand {
		candidate := *cand
		d.nextRoundCommittee = append(d.nextRoundCommittee, &candidate)
		if uint32(index) >= nextComSize-1 { //Only copy candidates equivalent to committe size
			break
		}
	}
}

//populateFromNextRoundCom copies the committee next round to the current round
func (d *Dpos) populateFromNextRoundCom() {
	d.committee = []*dposstate.Candidate{}

	for _, cand := range d.nextRoundCommittee {
		candidate := *cand
		d.committee = append(d.committee, &candidate)
	}

}

//loop loops over all the miner slots and then transition to next round
func (d *Dpos) loop() {
	log.Info().Msgf("Started Dpos loop")
	nextMinerTurn := false
	if d.roundStartTime == 0 { //For the first time only
		d.roundStartTime = lastMiningSlot(time.Now().Unix()).Unix()
		fmt.Println("Ensure same round-START: ", d.roundStartTime)
	}

	for {
		totalMiners := <-d.startRound //waits for the round start signal
		log.Info().Msgf("round start: %v", d.roundStartTime)
		if totalMiners == 0 {
			panic("No miners")
		}

		mIndex := 0 //Setting the miner index
	ComLoop:
		for mIndex < int(d.committeeSize) {

			blocksMined := 0
			curTime := (time.Now().Unix())
			tickTimer := time.After(d.TimeLeftToTick(curTime))
			blockPrepTimer := time.After(time.Duration(math.MaxInt64)) //Settin it to the infinity

			for d.TimeLeftToMine(curTime) < 0 { //For safety in case time is negative minerTimer will signal right away
				time.Sleep(100 * time.Millisecond)
				curTime = (time.Now().Unix())
			}

			log.Info().Msgf("Miner time left: %v", d.TimeLeftToMine(curTime)+time.Duration(200)*time.Millisecond)
			minerTimer := time.After(d.TimeLeftToMine(curTime) + time.Duration(200)*time.Millisecond)
			log.Info().Msgf("commitee size: %v", len(d.committee))
			mIndex = d.calcMinerIndUsingRoundTs(time.Now().Unix())
			miner := d.committee[mIndex]
			d.curMinerAdd = miner.Address
			if d.curMinerAdd == d.nodeAddress {
				d.miningSignal <- struct{}{} //Current node just has started to mine
				if d.TimeLeftToTick(time.Now().Unix()) >= blockPrepTime {
					blockPrepTimer = time.After(d.TimeLeftToTick(time.Now().Unix()) - 2*time.Second)
				}
			}

			for {
				select {

				case <-tickTimer:
					// fmt.Println("TS is: ", time.Now().Unix())
					log.Info().Msgf("%s to mine now. VotePower: %v", miner.Address, miner.VotePower)
					d.mineBlock(time.Now(), miner, d.roundNumber) //Should be called on miner
					d.bcHeight++                                  //TODO:- make increment in the syncer
					blocksMined++

					if blocksMined < d.BlocksPerMiner() {
						curTime = (time.Now().Unix())
						tickTimer = time.After(d.TimeLeftToTick(curTime))
						if d.curMinerAdd == d.nodeAddress {
							blockPrepTimer = time.After(d.TimeLeftToTick(time.Now().Unix()) - blockPrepTime)
						}
					} else {
						nextMinerTurn = true
					}

				case <-blockPrepTimer:
					fmt.Println("Start preparing block...")
					d.prepBlockSig <- struct{}{}

				case <-minerTimer:
					nextMinerTurn = true

				case <-d.quitChL:
					// fmt.Println("Stopped Dpos loop.")
					return
				}

				if nextMinerTurn == true {
					nextMinerTurn = false
					break
				}
			}

			if mIndex == len(d.committee)-1 { //In case you are syncing up all nodes might not get the turn. So check index
				nextMinerTurn = false

				log.Info().Msgf("Across round tolerance time:- %v", time.Second)
				tolTicker := time.After(time.Second)
				d.curMinerAdd = ""
				for {
					select {
					case <-tolTicker:
						break ComLoop
					case <-d.quitChL:
						return
					}
				}

			}

		}
		d.roundEnded <- struct{}{}
	}
}

//GetCurrentMinerAdd returns the address of the current miner
func (d *Dpos) GetCurrentMinerAdd() string {
	return d.curMinerAdd
}

func (d *Dpos) mineBlock(now time.Time, miner *dposstate.Candidate, roundNum int64) error {

	if miner.Address == d.nodeAddress {

		d.createBlock <- struct{}{} //Block creation will be triggered in miner.go
		return nil
	}
	return errors.New("Not my turn")
}

//TimeLeftToMine returns the time left in next mining slot
func (d *Dpos) TimeLeftToMine(ts int64) time.Duration {
	nextTime := nextMiningSlot(ts)
	timeLeft := nextTime - ts
	return time.Duration(timeLeft) * time.Second
}

//TimeLeftToTick returns the time left in the mining tick in seconds
func (d *Dpos) TimeLeftToTick(ts int64) time.Duration {
	nextTime := nextTickSlot(ts)
	timeLeft := nextTime - ts
	return time.Duration(timeLeft) * time.Second
}

//NextTickTime return returns the time of next tixk
func (d *Dpos) NextTickTime(ts int64) int64 {
	nextTime := nextTickSlot(ts)
	return nextTime
}

//NextMiningSlotTime return returns the time of next mining slot
func (d *Dpos) NextMiningSlotTime(ts int64) time.Time {
	return time.Unix(nextMiningSlot(ts), 0)
}

//Basically it rounds down to the start of the round and gives the timestamp
func lastMiningSlot(ts int64) time.Time {
	now := time.Duration(ts) * time.Second
	last := ((now - time.Second) / minerSlotTime) * minerSlotTime
	return time.Unix(int64(last/time.Second), 0)
}

func nextTickSlot(ts int64) int64 {
	now := time.Duration(ts) * time.Second
	next := ((now + miningTickInterval) / miningTickInterval) * miningTickInterval
	return int64(next / time.Second)
}

func nextMiningSlot(ts int64) int64 {
	now := time.Duration(ts) * time.Second
	next := ((now + minerSlotTime) / minerSlotTime) * minerSlotTime
	return int64(next / time.Second)
}

func (d *Dpos) calcMinerIndex(ts int64) int {
	return (int(ts) / int(minerSlotTime.Seconds())) % int(d.committeeSize)
}

//Uses the rouding. (Not true)Because this function is only called when signalling based on absolute slot happens.(Will not be called in the middle of the slot time; could be when syncing)
func (d *Dpos) calcMinerIndUsingRoundTs(ts int64) int {
	timeDif := ts - d.roundStartTime
	return int(timeDif) / int(minerSlotTime.Seconds()) % int(d.committeeSize)
}

//RoundNum rounds the number to the nearest unit
func RoundNum(x, unit float64) float64 {
	return math.Round(x/unit) * unit
}

func (d *Dpos) nextCommitteeTime(curComInd int) int64 {
	return int64(curComInd) * int64(d.committeeTime().Seconds())
}

func (d *Dpos) currentCommitteeIndex(ts int64) int {
	return int(ts / int64(d.committeeTime().Seconds()))
}

func (d *Dpos) committeeTime() time.Duration {
	return minerSlotTime * time.Duration(d.committeeSize)
}

//BlocksPerMiner returns the number of block a miner can produce
func (d *Dpos) BlocksPerMiner() int {
	numOfBlocks := minerSlotTime.Seconds() / miningTickInterval.Seconds()
	return int(numOfBlocks)
}

//RoundBlockTime returns the time in seconds for creating the blocks in a round
func (d *Dpos) RoundBlockTime() float64 {
	return float64(d.BlocksPerMiner()) * miningTickInterval.Seconds()
}

//RoundToleranceTime returns the time buffer(in seconds) at the end of the round
func (d *Dpos) RoundToleranceTime() float64 {
	return d.GetMinerSlotTime().Seconds() - d.RoundBlockTime()
}

//GetMinersAddresses returns the addresses of the miners currently mining in this round
func (d *Dpos) GetMinersAddresses() []string {
	var minerAddresses []string
	for _, cand := range d.committee {
		minerAddresses = append(minerAddresses, cand.Address)
	}
	return minerAddresses
}

//GetMinersForNextRound returns the miners computed using last round -> stored in nextRoundCommittee. It may not be final and could change in case there are delayed blocks from last round
func (d *Dpos) GetMinersForNextRound() []string {
	var minerAddresses []string
	for _, cand := range d.nextRoundCommittee {
		minerAddresses = append(minerAddresses, cand.Address)
	}
	return minerAddresses
}

//GetMinersByID get the miners ID
func (d *Dpos) GetMinersByID() []string {
	var minerIDs []string
	for _, cand := range d.committee {
		minerIDs = append(minerIDs, cand.ID)
	}
	return minerIDs
}

//PrintMiners print miners in the current round of the miners
func (d *Dpos) PrintMiners() {
	miners := d.GetMinersAddresses()
	for _, miner := range miners {
		fmt.Println(miner)
	}
}

//MinersToBytes makes a proto message of set of miners if current node is running dpos
func (d *Dpos) MinersToBytes() ([]byte, error) {

	if d.HasStarted == true {
		minerAdds := d.GetMinersAddresses()
		miners := &pb.CurrentMiners{
			MinerAddresses: minerAdds,
			NumMiners:      uint32(len(minerAdds)),
		}
		return proto.Marshal(miners)
	}

	return []byte{}, errors.New("Node doesn't runs dpos")
}

//MinersFromBytes returns the CurrentMiner instance form the bytes
func (d *Dpos) MinersFromBytes(minerBytes []byte) (*pb.CurrentMiners, error) {
	miners := new(pb.CurrentMiners)
	if err := proto.Unmarshal(minerBytes, miners); err != nil {
		return nil, err
	}
	return miners, nil
}

//ValidateMiningOrder validates the order of miner is the round map
func (d *Dpos) ValidateMiningOrder(parentMiner, childMiner string, pmRound, CRound int64, timeDiff, bTs, genesisTime int64) error {
	fmt.Println("Validating mining order...", parentMiner, childMiner, pmRound, CRound)
	slotDif := 0
	pmPosition := 0
	parentPresent, childPresent := false, false
	var slotsSinceGenesis int64 = 0
	chainStartTime := genesisTime - d.GetTickInterval().Nanoseconds()

	for i := pmRound; i < CRound+1; i++ {
		if i == pmRound {
			if pMiners, err := d.MinersInRound(pmRound); err == nil {

				for j, pMiner := range pMiners {
					if pMiner == parentMiner {
						parentPresent = true
						slotDif += len(pMiners) - j - 1
						pmPosition = j
						break
					}
				}
			} else {
				return errors.New("Blocks parent round doesn't exist")
			}
		}
		if i == CRound {
			if CMiners, err := d.MinersInRound(CRound); err == nil {

				for k, CMiner := range CMiners {
					if CMiner == childMiner {
						childPresent = true
						slotsPassed, err := d.SlotsPassedByRound(CRound - 1)
						if err != nil {
							return err
						}
						slotsSinceGenesis = slotsPassed + int64(k)
						if pmRound == CRound {
							slotDif = k - pmPosition

							if k < pmPosition {
								log.Info().Err(errors.New(("Child mining position is less than parent")))
								return errors.New("Child mining position is less than parent")
							}
							break
						}
						slotDif += k + 1
						break
					}
				}
			} else {
				return errors.New("Miner doesn't exist in the round")
			}
		}
		if i != pmRound && i != CRound {
			miners, err := d.MinersInRound(i)
			if err != nil {
				return err
			}
			slotDif += len(miners)
		}
	}

	if parentPresent == false {
		log.Warn().Msgf("Parent miner not found")
		return errors.New("Parent miner not found")
	} else if childPresent == false {
		log.Warn().Msgf("Child miner not found")
		return errors.New("Child miner not found")
	}

	slotStartTime := (slotsSinceGenesis * d.GetMinerSlotTime().Nanoseconds()) + chainStartTime
	if (bTs < (slotStartTime)) || (bTs > (slotStartTime + d.GetMinerSlotTime().Nanoseconds() + 1e9)) { //Will check the absolute time slot
		log.Info().Msgf("bts--MINING START TIME(sec): ", bTs/(1e9), slotStartTime/(1e9))
		return errors.New("In wrong slot")
	}

	return nil
}

//ValidateDigestMiners validates the digest block miner presence and their positions
func (d *Dpos) ValidateDigestMiners(cMiners, nextRoundMiners []string) error {
	curMiners := d.GetMinersAddresses()
	nextMiners := d.GetMinersForNextRound()
	if len(curMiners) != len(cMiners) {
		return errors.New("Wrong curMiner len")
	}

	for i, miner := range cMiners {
		if miner != curMiners[i] {
			return errors.New("Wrong curMiner entry")
		}
	}

	if len(nextMiners) != len(nextRoundMiners) {
		return errors.New("Wrong nextMiners len")
	}

	for i, miner := range nextMiners {
		if miner != nextRoundMiners[i] {
			return errors.New("Wrong curMiner entry")
		}
	}

	return nil
}

//PrintMinersByRound print all the miners by the round
func (d *Dpos) PrintMinersByRound() {

	var i int64 = 1
	for ; i <= d.roundNumber; i++ {
		if val, err := d.MinersInRound(i); err == nil {
			log.Info().Msgf("Round: ", i, " ", val)
		}
	}

}

//PrintMinersCache prints the miner cache
func (d *Dpos) PrintMinersCache() {

	for i, v := range d.MinerCache {
		fmt.Println(i, " ", v)
	}

}

//PrintSlotsPassedByRound prints the slots passed by round
func (d *Dpos) PrintSlotsPassedByRound() {

	var i, roundStart int64 = 0, 0
	for i = roundStart; i <= d.roundNumber; i++ {
		if val, err := d.SlotsPassedByRound(i); err == nil {
			fmt.Println("Round: ", i, " ", val)
		}
	}
}

//TimeLeftInRound returns the timeleft in round according to the DPOS
func (d *Dpos) TimeLeftInRound() int64 {
	curTime := time.Now().Unix()
	timeLeft := int64(d.committeeTime().Seconds()) - (curTime - d.roundStartTime)
	return timeLeft
}

//DposSnapshotUsingPb takes the snapshot of the Dpos state in pb struct
func (d *Dpos) DposSnapshotUsingPb() *PbStateSnapshot {

	snap := &PbStateSnapshot{
		SnapShot: pb.DposSnapShot{
			RoundNumber:   d.roundNumber,
			MinersByRound: make([]*pb.RoundMinersEntry, len(d.MinerCache)),
		},
	}
	snap.AddNextRoundComPb(d.nextRoundCommittee)

	for i, e := range d.MinerCache { //adding the miners by round in the array    1->r
		minerEntry := &pb.RoundMinersEntry{
			Miners:      e.Miners,
			Round:       e.Round,
			SlotsPassed: e.SlotsPassed,
		}
		snap.SnapShot.MinersByRound[i] = minerEntry
	}

	return snap
}

// NewDposUsingSnapShot creates a new lightwaight dpos instance from snapshot
func NewDposUsingSnapShot(snapshot *pb.DposSnapShot, address string, walletsAccess wallet.WalletAccess) (*Dpos, error) {

	newState := dposstate.GenerateStateFromWallets(walletsAccess) //Generating state from wallet access

	dpos := &Dpos{

		HasStarted: false,

		roundNumber: snapshot.RoundNumber, //Round number starts with zero
		State:       newState,

		wallets: walletsAccess,
	}

	for _, entry := range snapshot.MinersByRound {
		newEntry := &MinersInRound{
			Miners:      entry.Miners,
			Round:       entry.Round,
			SlotsPassed: entry.SlotsPassed,
		}
		dpos.MinerCache = append(dpos.MinerCache, newEntry)
	}

	dpos.committee = []*dposstate.Candidate{}
	for _, id := range dpos.MinerCache[len(dpos.MinerCache)-1].Miners {
		cand, err := dpos.State.GetCandidate(id)
		if err != nil {
			return nil, err
		}
		dpos.committee = append(dpos.committee, cand)
	}

	dpos.nextRoundCommittee = []*dposstate.Candidate{} //setting the current round committee
	for _, v := range snapshot.NextRoundCom {
		dpos.nextRoundCommittee = append(dpos.nextRoundCommittee, dposstate.FromProto(v))
	}

	return dpos, nil
}

//InjectEpochState incorporate epoch state in the dpos
func (d *Dpos) InjectEpochState(walletsAccess wallet.WalletAccess, cMiners, nextMiners []string, round, slotsPassed int64) error {

	newState := dposstate.GenerateStateFromWallets(walletsAccess)

	newCommittee := []*dposstate.Candidate{}
	for _, id := range cMiners {
		cand, err := newState.GetCandidate(id)
		if err != nil {
			return err
		}
		newCommittee = append(newCommittee, cand)
	}

	newRoundCom := []*dposstate.Candidate{}
	for _, id := range nextMiners {
		cand, err := newState.GetCandidate(id)
		if err != nil {
			return err
		}
		newRoundCom = append(newRoundCom, cand)
	}

	newEntry := &MinersInRound{
		Miners:      cMiners,
		Round:       round,
		SlotsPassed: slotsPassed,
	}

	d.State = newState
	d.roundNumber = round
	d.wallets = walletsAccess
	d.committee = []*dposstate.Candidate{}
	d.committee = newCommittee
	d.nextRoundCommittee = []*dposstate.Candidate{}
	d.nextRoundCommittee = newRoundCom

	d.MinerCache = append(d.MinerCache, newEntry)

	return nil
}

//InjectDposChanges injects the changes of the a Dpos into mainTail dpos
func (d *Dpos) InjectDposChanges(newDpos *Dpos) error {

	if d.HasStarted {
		log.Info().Msgf("Stopping dpos...")
		d.Stop()
	}

	d.State = newDpos.State.CloneState() //resetting the dposstate
	d.roundNumber = newDpos.roundNumber

	d.MinerCache = []*MinersInRound{}
	for _, e := range newDpos.MinerCache {
		newEntry := &MinersInRound{
			Miners:      e.Miners,
			Round:       e.Round,
			SlotsPassed: e.SlotsPassed,
		}
		d.MinerCache = append(d.MinerCache, newEntry)
	}

	d.nextRoundCommittee = []*dposstate.Candidate{}
	for _, cand := range newDpos.nextRoundCommittee {
		candidate := *cand
		d.nextRoundCommittee = append(d.nextRoundCommittee, &candidate)
	}

	curCom, err := d.MinersInRound(d.roundNumber)
	if err != nil {
		return errors.New("Current round committe doesn't exists")
	}
	d.committee = []*dposstate.Candidate{}
	for _, cand := range curCom {
		c, err := d.State.GetCandidate(cand)
		if err != nil {
			return err
		}
		d.committee = append(d.committee, c)
	}
	d.committeeSize = uint32(len(d.committee))

	return nil
}

//RevertDposState reverts the dpos state from block(bRound) to parent block(pbRound)
func (d *Dpos) RevertDposState(bRound, pbRound int64) {
	if bRound < 1 {
		return
	}

	//Ensure that dpos round is the same as this block round
	if d.GetRoundNum() > bRound { //Handle the case where current block is in n-1 round and new round hasn't produced a block as yet

		nextRoundMiners, err := d.MinersInRound(bRound + 1)
		if err != nil {
			panic("No entry")
		}

		for i := bRound + 1; i <= d.GetRoundNum(); i++ {
			d.DeleteEntryInMinerCache(i) //Just deleting entry to be on safe side
		}

		d.SetRoundNum(bRound) //Decreasing the round number-> putting it equal to the parent with diff round
		curMiners, _ := d.MinersInRound(bRound)
		d.InitializeComMembers(curMiners, nextRoundMiners)

	} else if d.GetRoundNum() < bRound {
		panic("DPOS round lower then current blockRound. Handle this")
	}

	for i := bRound; i > pbRound; i-- {
		fmt.Println("WARN: Del entry: ", bRound, pbRound, "DPOS round: ", d.GetRoundNum())
		nextRoundMiners, _ := d.MinersInRound(i)
		d.SetRoundNum(i - 1) //Decreasing the round number-> putting it equal to the parent with diff round
		d.DeleteEntryInMinerCache(i)
		curMiners, err := d.MinersInRound(i - 1)
		if err != nil {
			panic("No entry")
		}

		d.InitializeComMembers(curMiners, nextRoundMiners)
	}

}

//RevertDposTill reverts the Dpos state to the given block. Dpos state does include this block
func (d *Dpos) RevertDposTill(bRound int64) {
	log.Info().Msgf("Reverting DPOS till: %v", bRound)
	fmt.Println("Dpos round number: ", d.GetRoundNum())

	if bRound < 1 { //
		return
	}

	if d.GetRoundNum() > bRound { //Handle the case where current block is in n-1 round and new round hasn't produced a block as yet
		d.SetRoundNum(bRound) //Decreasing the round number-> putting it equal to the parent with diff round
		nextRoundMiners, err := d.MinersInRound(bRound + 1)
		if err != nil {
			panic("No entry")
		}
		curMiners, err := d.MinersInRound(bRound)
		if err != nil {
			panic("no entry")
		}
		d.InitializeComMembers(curMiners, nextRoundMiners)

		for i := bRound + 1; i <= d.GetRoundNum(); i++ {
			d.DeleteEntryInMinerCache(i) //Just deleting entry to be on safe side
		}
	} else if d.GetRoundNum() < bRound {
		panic("DPOS round lower then current blockRound. Block from future??")
	}
}
