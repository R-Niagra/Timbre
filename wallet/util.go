package wallet

import (
	"errors"
	"strconv"
	"strings"

	"github.com/guyu96/go-timbre/log"
)

//ParseVote parses the vote
func ParseVote(vote string) (string, int, string, error) {

	sepExist := strings.Contains(vote, "|")
	if !sepExist {
		return "", 0, "", errors.New("Invalid vote. Separater doesn't exist")
	}
	s := strings.Split(vote, "|")
	action := string(s[0][0])
	// fmt.Println("parsed vote is: ", s, "action: ", action)

	if action != "-" && action != "+" {
		log.Info().Msgf("Invalid action sign found in vote: %s", vote)
		return "", 0, "", errors.New("Invalid action sign found in vote: ")
	}
	var num int = 0
	var percentage string = ""
	percentage = s[0][1:]
	if percentage == "" {
		return "", 0, "", errors.New("Invalid action sign found in vote: ")
	}
	num, _ = strconv.Atoi(percentage)
	if num < 0 || num > 100 {
		return "", 0, "", errors.New("Invalid vote number found")
	}
	// fmt.Println("action: ", action, "amountPer", percentage)

	candidate := s[1]
	if candidate == "" {
		return "", 0, "", errors.New("No address provided. Vote invalid")
	}

	return action, num, candidate, nil
}

//Account is used for sending info for a wallet over rpc
type Account struct {
	Address string
	Balance int64
}

//Vote is an intance of vote to one particular candidate
type Vote struct {
	Address string
	Percent int
}

//AccountVotes is all the votesPercentage given to candidates by one account
type AccountVotes struct {
	Account
	Votes []*Vote
}

//GetAccounts returns the balances of all the accounts
func (wm *WalletManager) GetAccounts() []*Account {

	var accounts []*Account

	for add, wallet := range wm.walletByAddress {
		newAcc := &Account{
			Address: add,
			Balance: wallet.GetBalance(),
		}
		accounts = append(accounts, newAcc)
	}
	return accounts

}

//GetAccountsWithVotes return all account with votes
func (wm *WalletManager) GetAccountsWithVotes() []*AccountVotes {

	var accounts []*AccountVotes

	for add, wallet := range wm.walletByAddress {

		var userVotes []*Vote
		for cand, percent := range wallet.Voted {
			userVotes = append(userVotes, &Vote{
				Address: cand,
				Percent: percent,
			})
		}
		accounts = append(accounts, &AccountVotes{
			Account{
				Address: add,
				Balance: wallet.GetBalance(),
			},
			userVotes,
		})
	}

	return accounts
}
