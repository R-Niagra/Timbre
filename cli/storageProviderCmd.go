package cli

import (
	"context"
	"fmt"

	rpcpb "github.com/guyu96/go-timbre/rpc/pb"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
)

var spCmd = &cli.Command{ //storage provider command
	Name:        "storageprovider",
	Usage:       "Manages the storage provider role",
	Category:    "Role Commands",
	Aliases:     []string{"sp"},
	Description: "timbre user entails all the commands associated with the storage provider role...",
	Subcommands: []*cli.Command{
		spStart,
	},
}

var spStart = &cli.Command{
	Name:  "Start",
	Usage: "Creates new storage provider(sp) instance and start it",
	Action: func(c *cli.Context) error {

		client, closer, err := getRPCClient(c)
		if err != nil {
			return err
		}
		defer closer()

		_, err = client.StartStorageProvider(context.Background(), &rpcpb.NonParamRequest{}, grpc.EmptyCallOption{})
		if err != nil {
			return err
		}

		return nil
	},
}

var sendStorageOffer = &cli.Command{
	Name:  "sendStorageOffer",
	Usage: "Sends storage offer to the network",
	Flags: []cli.Flag{
		&cli.Uint64Flag{
			Name:    "price",
			Usage:   "Sets the minimum price for the storage",
			Aliases: []string{"mp"},
			Value:   100, //setting the default price
		},
		&cli.Uint64Flag{
			Name:    "duration",
			Usage:   "Sets the duration of storage in Seconds",
			Aliases: []string{"d"},
			Value:   300000000,
		},
		&cli.Uint64Flag{
			Name:    "storageSize",
			Usage:   "Sets the maximum size of the storage in Bytes",
			Aliases: []string{"sz"},
			Value:   4294967295,
		},
	},
	Action: func(c *cli.Context) error {
		minPrice := c.Uint64("price")
		duration := c.Uint64("duration")
		storageSize := c.Uint64("storageSize")

		client, closer, err := getRPCClient(c)
		if err != nil {
			return err
		}
		defer closer()

		res, err := client.SendStorageOffer(context.Background(), &rpcpb.SendStorageOfferRequest{
			MinPrice:    uint32(minPrice),
			MaxDuration: uint32(duration),
			Size:        uint32(storageSize),
		}, grpc.EmptyCallOption{})
		if err != nil {
			return err
		}

		fmt.Println(res)

		return nil
	},
}
