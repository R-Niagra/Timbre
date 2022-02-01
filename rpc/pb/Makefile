proto:
	protoc -I/usr/local/include -I. \
	-I.:$(GOPATH)/src:../../protobuf:../..:$(GOPATH)/pkg/mod/github.com/grpc-ecosystem/grpc-gateway@v1.14.3/third_party/googleapis \
  	--go_out=plugins=grpc:. \
	--grpc-gateway_out=logtostderr=true:. \
	--swagger_out=logtostderr=true:. \
  	*.proto

clean:
	rm *.pb.go
	rm *.pb.gw.go
	rm *.json
