package cli

import (
	"context"

	rpcpb "github.com/guyu96/go-timbre/rpc/pb"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
)

var userCmd = &cli.Command{
	Name:        "user",
	Usage:       "Manages the Miner role",
	Category:    "Role Commands",
	Description: "timbre user entails all the commands associated with the user role...",
	Subcommands: []*cli.Command{
		userStart,
	},
}

var userStart = &cli.Command{
	Name:  "Start",
	Usage: "Creates new user instance and start it",
	Action: func(c *cli.Context) error {

		client, closer, err := getRPCClient(c)
		if err != nil {
			return err
		}
		defer closer()

		_, err = client.StartUser(context.Background(), &rpcpb.NonParamRequest{}, grpc.EmptyCallOption{})
		if err != nil {
			return err
		}

		return nil
	},
}
