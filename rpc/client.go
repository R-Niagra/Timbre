package rpc

import (
	rpcpb "github.com/guyu96/go-timbre/rpc/pb"
	"google.golang.org/grpc"
)

//Closer is the conn closer
type Closer func() error

//NewRPCClient ceates an grpc client
func NewRPCClient(serverPort string) (rpcpb.ApiClient, Closer, error) {
	opts := []grpc.DialOption{grpc.WithInsecure()}
	conn, err := grpc.Dial(serverPort, opts...) //Should close connection later on
	if err != nil {
		return nil, nil, err
	}
	// defer conn.Close()

	client := rpcpb.NewApiClient(conn)
	// ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	// defer cancel()

	// res, err := client.GetAccount(ctx, &rpcpb.NonParamRequest{}, grpc.EmptyCallOption{})
	// if err != nil {
	// 	panic(err.Error())
	// }
	// fmt.Println("Result is: ", res)
	return client, conn.Close, nil
}
