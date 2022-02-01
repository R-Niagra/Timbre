PBPATH=./protobuf

proto: ${PBPATH}/*.proto
	protoc --proto_path=${PBPATH} --go_out=${PBPATH} ${PBPATH}/*.proto 

clean:
	rm ${PBPATH}/*.pb.go
