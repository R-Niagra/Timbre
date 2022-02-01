package dpos

//RoundMiners contains a round miners
type RoundMiners struct {
	Miners []string
	Round  int64
}

//GetMinersByRound returns miners in every round
func (d *Dpos) GetMinersByRound() ([]*RoundMiners, error) {

	var roundMiners []*RoundMiners
	var i int64 = 1
	for ; i <= d.roundNumber; i++ {
		val, err := d.MinersInRound(i)
		if err != nil {
			return nil, err
		}
		roundMiners = append(roundMiners, &RoundMiners{
			Round:  i,
			Miners: val,
		})

	}

	return roundMiners, nil
}
