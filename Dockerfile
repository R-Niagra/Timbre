FROM golang:1.12.1-stretch

# Set necessary environmet variables needed for our image
RUN apt-get update && apt-get install -y unzip
RUN apt-get install -y autoconf automake libtool protobuf-compiler sudo
RUN pwd
ENV SRC_DIR go/src/github.com/go-timbre 
ENV GO111MODULE on

#WORKDIR /tmp/protobuf-3.3.0
RUN mkdir /export


# Install protoc-gen-go
RUN go get github.com/golang/protobuf/protoc-gen-go
RUN cp /go/bin/protoc-gen-go /export
#Ending protobuf installation

COPY . $SRC_DIR



WORKDIR $SRC_DIR

RUN ls

RUN make proto

RUN go mod download && ls
#RUN cp install_pbc.sh /

#WORKDIR /
RUN sudo ./install_pbc.sh
#WORKDIR $SRC_DIR

# Move to /dist directory as the place for resulting binary folder
WORKDIR simulation/main

RUN ls


# Copy binary from build to main folder
# RUN cp /go_timbre/simulation/main

# Export necessary port
EXPOSE 7001

# Command to run when starting the container
CMD ["go","run","main.go"]
