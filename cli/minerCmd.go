package cli

import (
	"context"

	rpcpb "github.com/guyu96/go-timbre/rpc/pb"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
)

var minerCmd = &cli.Command{
	Name:        "miner",
	Usage:       "Manages the Miner role",
	Category:    "Role Commands",
	Description: "timbre miner entails all the commands associated with the miner role...",
	Subcommands: []*cli.Command{
		minerStart,
	},
}

var minerStart = &cli.Command{
	Name:  "Start",
	Usage: "Creates new miner instance and start the miner",
	Action: func(c *cli.Context) error {

		client, closer, err := getRPCClient(c)
		if err != nil {
			return err
		}
		defer closer()

		_, err = client.StartMiner(context.Background(), &rpcpb.NonParamRequest{}, grpc.EmptyCallOption{})
		if err != nil {
			return err
		}

		return nil
	},
}
