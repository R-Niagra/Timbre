package cli

import (
	"context"
	"fmt"
	"strings"

	rpcpb "github.com/guyu96/go-timbre/rpc/pb"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
)

var txCmd = &cli.Command{
	Name:        "tx",
	Usage:       "Handles all transaction related services",
	Description: "tx will let you create all types of transactions(retrieval,vote,Del-reg,Del-quit) and relay them to the miners",
	Subcommands: []*cli.Command{
		createTransferTx,
		createVoteTx,
		createUnVoteTx,
		createDelegateRegistrationTx,
		createDelegateQuitTx,
	},
}

var createTransferTx = &cli.Command{
	Name:    "newTransferTx",
	Usage:   "Create a new retrieval transaction",
	Aliases: []string{"rt"},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "recepientAddress",
			Aliases:  []string{"ra"},
			Required: true,
		},
		&cli.Uint64Flag{
			Name:     "amount",
			Aliases:  []string{"a"},
			Required: true,
		},
	},
	Action: func(c *cli.Context) error {

		recAdd := c.String("recepientAddress")
		amount := c.Uint64("amount")
		client, closer, err := getRPCClient(c)
		if err != nil {
			return err
		}
		defer closer()

		res, err := client.TransferMoney(context.Background(), &rpcpb.TransferMoneyRequest{
			Address: strings.TrimSpace(recAdd),
			Amount:  amount,
		}, grpc.EmptyCallOption{})
		if err != nil {
			return err
		}
		fmt.Println(res)

		return nil
	},
}

var createVoteTx = &cli.Command{
	Name:    "newVoteTx",
	Usage:   "Create a new vote transaction",
	Aliases: []string{"vt"},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "recepientAddress",
			Aliases:  []string{"ra"},
			Required: true,
		},
		&cli.StringFlag{
			Name:     "percent",
			Aliases:  []string{"p"},
			Required: true,
		},
	},
	Action: func(c *cli.Context) error {

		recAdd := c.String("recepientAddress")
		percent := c.String("percent")
		client, closer, err := getRPCClient(c)
		if err != nil {
			return err
		}
		defer closer()

		res, err := client.DoVote(context.Background(), &rpcpb.VoteRequest{
			Address:    strings.TrimSpace(recAdd),
			Percentage: strings.TrimSpace(percent),
		}, grpc.EmptyCallOption{})
		if err != nil {
			return err
		}
		fmt.Println(res)

		return nil
	},
}

var createUnVoteTx = &cli.Command{
	Name:    "newUnVoteTx",
	Usage:   "Create a new un-vote transaction",
	Aliases: []string{"uvt"},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "recepientAddress",
			Aliases:  []string{"ra"},
			Required: true,
		},
		&cli.StringFlag{
			Name:     "percent",
			Aliases:  []string{"p"},
			Required: true,
		},
	},
	Action: func(c *cli.Context) error {

		recAdd := c.String("recepientAddress")
		percent := c.String("percent")
		client, closer, err := getRPCClient(c)
		if err != nil {
			return err
		}
		defer closer()

		res, err := client.DoUnVote(context.Background(), &rpcpb.VoteRequest{
			Address:    strings.TrimSpace(recAdd),
			Percentage: strings.TrimSpace(percent),
		}, grpc.EmptyCallOption{})
		if err != nil {
			return err
		}
		fmt.Println(res)

		return nil
	},
}

var createDelegateRegistrationTx = &cli.Command{
	Name:        "newDelRegTx",
	Usage:       "Create a new delegate registration transaction",
	Description: "Delegate registration tx is required to become miner",
	Aliases:     []string{"dr"},

	Action: func(c *cli.Context) error {

		client, closer, err := getRPCClient(c)
		if err != nil {
			return err
		}
		defer closer()

		res, err := client.RegisterDelegate(context.Background(), &rpcpb.NonParamRequest{}, grpc.EmptyCallOption{})
		if err != nil {
			return err
		}
		fmt.Println(res)

		return nil
	},
}

var createDelegateQuitTx = &cli.Command{
	Name:        "newDelQuitTx",
	Usage:       "Create a new delegate quit transaction",
	Description: "Delegate quit tx is required to discharge from being miner",
	Aliases:     []string{"dq"},

	Action: func(c *cli.Context) error {

		client, closer, err := getRPCClient(c)
		if err != nil {
			return err
		}
		defer closer()

		res, err := client.UnRegisterDelegate(context.Background(), &rpcpb.NonParamRequest{}, grpc.EmptyCallOption{})
		if err != nil {
			return err
		}
		fmt.Println(res)
		return nil
	},
}
