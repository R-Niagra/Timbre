package rpc

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	gonet "net"
	"net/http"
	"os"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/guyu96/go-timbre/log"
	"github.com/guyu96/go-timbre/net"
	rpcpb "github.com/guyu96/go-timbre/rpc/pb"
	"github.com/rs/cors"
	"google.golang.org/grpc"
)

// grpc on port:7211
// HTTP on port 7200

// Server is rpc server.
type Server struct {
	rpcServer *grpc.Server
	rpcPort   string
	httpPort  string
}

// Returns new server instance
func NewRPCServer(grpcPort, httpPort string) *Server {
	rpc := grpc.NewServer()

	return &Server{
		rpcServer: rpc,
		rpcPort:   ":" + grpcPort,
		httpPort:  ":" + httpPort,
	}
}

// Give this some details about the chain
func (s *Server) Run(node *net.Node) error {
	api := NewAPI(node)
	rpcpb.RegisterApiServer(s.rpcServer, api)

	lis, err := gonet.Listen("tcp", s.rpcPort)
	if err != nil {
		return err
	}

	go s.rpcServer.Serve(lis)

	err = s.RunGateway()
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) RunGateway() error {

	sr, err := NewHTTPServer(s.rpcPort, s.httpPort)
	if err != nil {
		log.Info().Msgf("Failed to start grpc http gateway")
	}
	go func() {
		err = sr.Run()
		if err != nil {
			log.Info().Msgf("Failed to start grpc http gateway")
		}
	}()
	return nil
}

// Middleware for Swagger
type HTTPServer struct {
	handler  http.Handler
	cancel   context.CancelFunc
	rpcPort  string
	httpPort string
}

// This should be the new fuc
// Split into setup and start
func NewHTTPServer(rpcPort, httpPort string) (*HTTPServer, error) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithInsecure()}
	err := rpcpb.RegisterApiHandlerFromEndpoint(ctx, mux, rpcPort, opts)
	if err != nil {
		return nil, err
	}
	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/swagger.json", func(w http.ResponseWriter, req *http.Request) {
		raw, err := ioutil.ReadFile("../../rpc/pb/rpc.swagger.json")
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		io.Copy(w, strings.NewReader(string(raw)))
	})
	httpMux.Handle("/swagger/", http.StripPrefix("/swagger/", http.FileServer(http.Dir("../../rpc/swaggerui"))))
	httpMux.Handle("/", mux)
	return &HTTPServer{
		handler:  cors.Default().Handler(httpMux),
		cancel:   cancel,
		rpcPort:  rpcPort,
		httpPort: httpPort,
	}, nil
}

func (sr *HTTPServer) Run() error {
	defer sr.cancel()
	return http.ListenAndServe(sr.httpPort, sr.handler)
}
