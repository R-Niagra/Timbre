package cli

import (
	"errors"
	"fmt"

	"github.com/guyu96/go-timbre/rpc"
	rpcpb "github.com/guyu96/go-timbre/rpc/pb"

	"github.com/urfave/cli/v2"
)

//getRpcClient gets a new client for the rpc call
func getRPCClient(ctx *cli.Context) (rpcpb.ApiClient, rpc.Closer, error) {
	servPort, ok := ctx.App.Metadata["rpcPort"]
	if !ok {
		fmt.Println("server port unknown")
		return nil, nil, errors.New("unknown port of the rpc server")
	}
	serverPort := ":" + servPort.(string)
	fmt.Println("server port: ", serverPort)

	client, closer, err := rpc.NewRPCClient(serverPort)
	if err != nil {
		return nil, nil, err
	}
	return client, closer, nil
}

//roleStarter is for starting different roles
type roleStarter func() error

//Commands are all the commands associated with a node
var Commands = []*cli.Command{
	minerCmd,
	spCmd,
	userCmd,
	accountCmd,
	dposCmd,
	txCmd,
}
