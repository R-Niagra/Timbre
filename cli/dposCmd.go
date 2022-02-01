package cli

import (
	"context"
	"fmt"

	rpcpb "github.com/guyu96/go-timbre/rpc/pb"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
)

var dposCmd = &cli.Command{
	Name:  "dpos",
	Usage: "Handle api calls linked with the consensus protocol(Dpos)",
	// Category:    "Basic",
	Description: "dpos takes care of all the useful api-calls majorly for debugging",
	Subcommands: []*cli.Command{
		curRound,
		curMiners,
		listMinersByRound,
		// listSlotsByRound,
		// printMinersCache,
	},
}

var curRound = &cli.Command{
	Name:    "currentRound",
	Usage:   "lists the current round number in network",
	Aliases: []string{"cr"},

	Action: func(c *cli.Context) error {

		client, closer, err := getRPCClient(c)
		if err != nil {
			return err
		}
		defer closer()

		res, err := client.GetCurrentRound(context.Background(), &rpcpb.NonParamRequest{}, grpc.EmptyCallOption{})
		if err != nil {
			return err
		}

		fmt.Println("Current round number is: ", res.RoundNum)

		return nil
	},
}

var curMiners = &cli.Command{
	Name:    "currentMiners",
	Usage:   "lists current miners in current round",
	Aliases: []string{"cm"},
	Action: func(c *cli.Context) error {

		client, closer, err := getRPCClient(c)
		if err != nil {
			return err
		}
		defer closer()

		res, err := client.GetCurrentMiners(context.Background(), &rpcpb.NonParamRequest{}, grpc.EmptyCallOption{})
		if err != nil {
			return err
		}

		fmt.Println("Current miners are: ", res.Miners)

		return nil
	},
}

var listMinersByRound = &cli.Command{
	Name:    "listMinersByRound",
	Usage:   "lists all miners by round. For debugging",
	Aliases: []string{"lmr"},
	Action: func(c *cli.Context) error {

		client, closer, err := getRPCClient(c)
		if err != nil {
			return err
		}
		defer closer()

		res, err := client.GetMinersByRound(context.Background(), &rpcpb.NonParamRequest{}, grpc.EmptyCallOption{})
		if err != nil {
			return err
		}

		for _, m := range res.AllRoundMiners {
			fmt.Println(m.Round, " has ", m.Miners)
		}

		return nil
	},
}
