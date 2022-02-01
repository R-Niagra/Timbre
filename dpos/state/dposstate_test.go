package dposstate

import (
	"reflect"
	"testing"
)

func TestState_SortByVotes(t *testing.T) {
	type fields struct {
		childCommitteeSize uint32
		candidateState     map[string]*Candidate
		totalVoteStaked    uint64
	}

	difName := map[string]*Candidate{
		"abc": &Candidate{
			Address:   "abc",
			ID:        "abc",
			VotePower: 0,
		},
		"abd": &Candidate{
			Address:   "abd",
			ID:        "abd",
			VotePower: 0,
		},
		"abe": &Candidate{
			Address:   "abe",
			ID:        "abe",
			VotePower: 0,
		},
	}
	difVP := map[string]*Candidate{
		"abc": &Candidate{
			Address:   "abc",
			ID:        "abc",
			VotePower: 0,
		},
		"abd": &Candidate{
			Address:   "abd",
			ID:        "abd",
			VotePower: 1,
		},
		"abe": &Candidate{
			Address:   "abe",
			ID:        "abe",
			VotePower: 2,
		},
	}

	tests := []struct {
		name   string
		fields fields
		want   []*Candidate
	}{
		// TODO: Add test cases.
		{
			name: "Testing different names",
			fields: fields{
				childCommitteeSize: 3,
				candidateState:     difName,
				totalVoteStaked:    0,
			},
			want: []*Candidate{&Candidate{
				Address:   "abc",
				ID:        "abc",
				VotePower: 0,
			}, &Candidate{
				Address:   "abd",
				ID:        "abd",
				VotePower: 0,
			}, &Candidate{
				Address:   "abe",
				ID:        "abe",
				VotePower: 0,
			}},
		},

		{
			name: "Testing different names",
			fields: fields{
				childCommitteeSize: 3,
				candidateState:     difVP,
				totalVoteStaked:    0,
			},
			want: []*Candidate{
				&Candidate{
					Address:   "abe",
					ID:        "abe",
					VotePower: 2,
				}, &Candidate{
					Address:   "abd",
					ID:        "abd",
					VotePower: 1,
				},
				&Candidate{
					Address:   "abc",
					ID:        "abc",
					VotePower: 0,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &State{
				childCommitteeSize: tt.fields.childCommitteeSize,
				candidateState:     tt.fields.candidateState,
				totalVoteStaked:    tt.fields.totalVoteStaked,
			}
			if got := s.SortByVotes(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("State.SortByVotes() = %v, want %v", got, tt.want)
			}
		})
	}
}
