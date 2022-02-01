package cli

import (
	"context"
	"fmt"

	rpcpb "github.com/guyu96/go-timbre/rpc/pb"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
)

var accountCmd = &cli.Command{
	Name:  "account",
	Usage: "Handle the accounts-api on the blockchain",
	// Category:    "Basic",
	Description: "account takes care of all the api call for back-end accounts. Useful for debugging",
	Subcommands: []*cli.Command{
		myAddress,
		myBalance,
		listBalances,
		listAccounts,
		listVotes,
	},
}

var myAddress = &cli.Command{
	Name:    "myAddress",
	Usage:   "list the adddress associated with the node",
	Aliases: []string{"myAdd"},
	Action: func(c *cli.Context) error {

		client, closer, err := getRPCClient(c)
		if err != nil {
			return err
		}
		defer closer()

		res, err := client.GetAccount(context.Background(), &rpcpb.NonParamRequest{}, grpc.EmptyCallOption{})
		if err != nil {
			return err
		}

		fmt.Println("My address is: ", res.GetAddress())

		return nil
	},
}

var myBalance = &cli.Command{
	Name:  "balance",
	Usage: "Shows the current balance associated with the node",
	Action: func(c *cli.Context) error {

		client, closer, err := getRPCClient(c)
		if err != nil {
			return err
		}
		defer closer()

		res, err := client.GetAccount(context.Background(), &rpcpb.NonParamRequest{}, grpc.EmptyCallOption{})
		if err != nil {
			return err
		}
		fmt.Println(res.GetAddress(), "has ", res.GetBalance(), " decibels")

		return nil
	},
}

var listBalances = &cli.Command{
	Name:  "listBalances",
	Usage: "List current balance of all the accounts on the network",
	Action: func(c *cli.Context) error {

		client, closer, err := getRPCClient(c)
		if err != nil {
			return err
		}
		defer closer()

		res, err := client.ListBalances(context.Background(), &rpcpb.NonParamRequest{}, grpc.EmptyCallOption{})
		if err != nil {
			return err
		}
		fmt.Println(res)

		return nil
	},
	Category: "DebugAccoundCmds",
}

var listAccounts = &cli.Command{
	Name:  "listAccounts",
	Usage: "List all account addresses in blockchain",
	Action: func(c *cli.Context) error {

		client, closer, err := getRPCClient(c)
		if err != nil {
			return err
		}
		defer closer()

		res, err := client.ListBalances(context.Background(), &rpcpb.NonParamRequest{}, grpc.EmptyCallOption{})
		if err != nil {
			return err
		}
		for _, acc := range res.Accounts {
			fmt.Println(acc.Address)
		}
		return nil
	},
	Category: "DebugAccoundCmds",
}

var listVotes = &cli.Command{
	Name:  "listVotes",
	Usage: "List current votes of all the accounts on the network",
	Action: func(c *cli.Context) error {

		client, closer, err := getRPCClient(c)
		if err != nil {
			return err
		}
		defer closer()

		res, err := client.ListVotes(context.Background(), &rpcpb.NonParamRequest{}, grpc.EmptyCallOption{})
		if err != nil {
			return err
		}
		for _, acc := range res.AccountsWithVotes {
			fmt.Println(acc.Acc.Address, " voted: ")
			for _, v := range acc.Votes {
				fmt.Println(" ", v.Address, " with ", v.Percent, "% decibels")
			}
		}
		return nil

	},
	Category: "DebugAccoundCmds",
}
