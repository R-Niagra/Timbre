package dposstate

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/guyu96/go-timbre/log"
	pb "github.com/guyu96/go-timbre/protobuf"
	"github.com/guyu96/go-timbre/wallet"
)

const (
	defaultCommitteeSize = 3
)

//BlockchainAccess to access the hight of the chain
type BlockchainAccess interface {
	Length() int //To get the length of the blockchain
}

//State maintains the states of the candidates to be elected eventually for the next round
type State struct {
	childCommitteeSize uint32
	candidateState     map[string]*Candidate //For Candidate voting in current round to elect committee for next round
	totalVoteStaked    uint64
	// committeeState []*Candidate
}

//NewDposState returns the state object
func NewDposState() *State {
	// Todo : connect with storage for persistence
	return &State{
		candidateState:     make(map[string]*Candidate),
		childCommitteeSize: defaultCommitteeSize,
		totalVoteStaked:    0,
	}
}

//CloneState clone the current state
func (s *State) CloneState() *State {

	copyState := &State{
		candidateState:     make(map[string]*Candidate),
		childCommitteeSize: s.childCommitteeSize,
		totalVoteStaked:    s.totalVoteStaked,
	}

	for k, v := range s.candidateState {
		copyState.candidateState[k] = v
	}

	return copyState

}

//GetStateFromPb generate state from protobuf state struct
func GetStateFromPb(pbState *pb.CommitteeState) *State {
	newState := &State{
		childCommitteeSize: pbState.ChildCommitteeSize,
		totalVoteStaked:    pbState.TotalVoteStaked,
		candidateState:     make(map[string]*Candidate),
	}

	for _, cand := range pbState.CandidatesState {
		newState.candidateState[cand.Address] = FromProto(cand)
	}

	return newState
}

//CloneStateToPb clone the current state
func (s *State) CloneStateToPb() *pb.CommitteeState {
	copyState := &pb.CommitteeState{
		ChildCommitteeSize: s.childCommitteeSize,
		TotalVoteStaked:    s.totalVoteStaked,
		CandidatesState:    make([]*pb.Candidate, len(s.candidateState)),
	}
	for _, v := range s.candidateState {
		copyState.CandidatesState = append(copyState.CandidatesState, v.ToProto())
	}
	return copyState
}

//ResetState resets the votepower of candidates in map to zero
func (s *State) ResetState() {

	for key, candidate := range s.candidateState {
		candidate.VotePower = 0
		s.candidateState[key] = candidate
	}
	s.totalVoteStaked = 0

}

//InitializeStateWithMembers put the members in the state
func (s *State) InitializeStateWithMembers(candidates []*Candidate) {
	//Emptying the state map
	s.candidateState = make(map[string]*Candidate)

	for _, cand := range candidates {
		s.candidateState[cand.Address] = cand
	}
	s.totalVoteStaked = 0

}

//TotalCandidates returns the length of the candidate state
func (s *State) TotalCandidates() int {
	return len(s.candidateState)
}

//CheckCandExistance checks if candidate exists in the candidate state
func (s *State) CheckCandExistance(id string) bool {
	if _, ok := s.candidateState[id]; ok {
		return true
	}
	return false
}

//GetTotalVoteStaked returns the total vote power being casted in a round
func (s *State) GetTotalVoteStaked() uint64 {
	return s.totalVoteStaked
}

//GetChildCommitteeSize returns the ChildCommitteeSize
func (s *State) GetChildCommitteeSize() uint32 {
	return s.childCommitteeSize
}

//SetChildCommitteeSize sets the committee size
func (s *State) SetChildCommitteeSize(size uint32) {
	s.childCommitteeSize = size
}

// GetCandidates returns candidate list from candidate state.
func (s *State) GetCandidates() []*Candidate {

	candidates := make([]*Candidate, 0, len(s.candidateState))
	for _, value := range s.candidateState {
		candidates = append(candidates, value)
	}
	return candidates
}

// PutCandidate add candidate to candidate array
func (s *State) PutCandidate(id string, candidate *Candidate) error {
	if _, ok := s.candidateState[id]; ok {
		return errors.New("Candidate with id already exists")
	}
	s.candidateState[id] = candidate
	return nil
}

//GetCandidate returns the candidate by id
func (s *State) GetCandidate(id string) (*Candidate, error) {
	if _, ok := s.candidateState[id]; ok {
		return s.candidateState[id], nil
	}
	return nil, errors.New("Candidate is not in store")
}

// AddVotePowerToCandidate adds vote to the candidate
func (s *State) AddVotePowerToCandidate(id string, amount int64) error {

	candidate, err := s.GetCandidate(id)
	if err != nil {
		return err
	}
	candidate.VotePower += amount
	s.totalVoteStaked += uint64(amount)
	log.Info().Msgf(id, "Gained vote power by: %d", amount)
	return nil
}

//SubVotePowerToCandidate subtracts the vote power from candidates
func (s *State) SubVotePowerToCandidate(id string, amount int64) error {

	candidate, err := s.GetCandidate(id)
	if err != nil {
		return err
	}
	candidate.VotePower -= amount
	return nil
}

// SortByVotes Sort candidates for committee for votes
func (s *State) SortByVotes(bcHeight int64) []*Candidate {

	candidates := s.GetCandidates()

	if bcHeight != 0 {

		for i, cand := range candidates { //Removing the banned candidates from the sorting list
			if cand.bannedTill >= bcHeight {
				candidates = append(candidates[0:i], candidates[i+1:]...)
				fmt.Println("BANNED: ", cand)
			}
		}
	} else {
		fmt.Println("BC height zero in sortByvotes")
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Address < (candidates[j].Address)
	})

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].VotePower > (candidates[j].VotePower)
	})

	return append([]*Candidate(nil), candidates...)
}

//DelCandidate deletes the candidate id
func (s *State) DelCandidate(id string) error {
	if _, ok := s.candidateState[id]; ok {
		delete(s.candidateState, id)
		return nil
	}
	return errors.New("No entry against this id")
}

//RegisterCand registers the candidate if wallet balance is positive
func (s *State) RegisterCand(id string) (*Candidate, error) {
	//Set of checks to ensure eligibility of the node to become candidate

	newCandidate := NewCandidate(id, id, 0, time.Now().Unix(), -1)

	if _, alreadyPresent := s.candidateState[id]; alreadyPresent {
		return nil, errors.New("Entry already in record. Can't register again")
	}
	s.candidateState[id] = newCandidate
	return newCandidate, nil

}

//QuitCandidate remove candidate from the state
func (s *State) QuitCandidate(id string) error {
	if _, present := s.candidateState[id]; !present {
		return errors.New("Entry not in record. Can't unregister")
	}
	delete(s.candidateState, id)
	return nil
}

//AddGenesisRoundCand adds the cand in the first round
func (s *State) AddGenesisRoundCand(address string, ID string) {
	newCandidate := NewCandidate(address, address, 0, time.Now().Unix(), -1)
	s.PutCandidate(address, newCandidate)
}

//UpdateVotePower updates the votepower in the state
func (s *State) UpdateVotePower(wallets wallet.WalletAccess) {
	for ID, cand := range s.candidateState {
		newVP, bannedHeight, err := wallets.GetCandVotePower(ID) //Gives the votepower VP
		if err != nil {
			continue
		}

		oldVP := cand.VotePower
		var DeltaVP int64 = int64(newVP) - int64(oldVP)
		cand.SetVotePower(int64(newVP))
		cand.SetBannedHeight(bannedHeight)
		s.totalVoteStaked += uint64(DeltaVP)
	}
}

//GenerateStateFromCandAdd generates the state from the wallets
func GenerateStateFromWallets(wallets wallet.WalletAccess) *State {

	cands := wallets.GetRegisteredCandidates()
	state := make(map[string]*Candidate)
	var totalVotes uint64 = 0

	for _, cand := range cands {
		newVP, bannedHeight, err := wallets.GetCandVotePower(cand) //Gives the votepower VP
		if err != nil {
			continue
		}

		c := NewCandidate(cand, cand, int64(newVP), time.Now().Unix(), bannedHeight)
		totalVotes += newVP
		state[cand] = c
	}

	return &State{
		candidateState:     state,
		childCommitteeSize: defaultCommitteeSize,
		totalVoteStaked:    totalVotes,
	}
}

//PrintCandidates print candidates in the state
func (s *State) PrintCandidates() {
	for _, cand := range s.candidateState {
		fmt.Println(cand.Address, " banned till: ", cand.bannedTill)
	}
}

//CheckCandByAdd checks if a candidate with address exists in the dpos state
func (s *State) CheckCandByAdd(add string) bool {
	if _, ok := s.candidateState[add]; ok {
		return true
	}
	return false
}
