# Timbre
Timbre is a blockchain-based decentralized forum. This repository contains the golang implementation of Timbre.
(Please note this is a trimmed down version of the original repo)

## Setup
To set up Timbre for development, make sure you have golang (>1.11) installed, clone this repository, and follow the instructions below.

### Ubuntu
1. Fetch project dependencies
    ```
    go mod download
    ```
2. Install [pbc](https://crypto.stanford.edu/pbc/), the pairing-based cryptography library
    ```
    sudo ./install_pbc.sh
    ```
3. Download the latest protobuf compiler `protoc` from https://github.com/protocolbuffers/protobuf/releases

4. Install `protoc`
    ```
    sudo unzip -o $PROTOC_ZIP -d /usr/local bin/protoc
    sudo unzip -o $PROTOC_ZIP -d /usr/local 'include/*'
    ```
5. Compile protobuf structs
    ```
    make proto
    ```
## To run the node
Go to the cmd directory and run the main file. See help for options.
     
## Development
Each package contains an independent README that explains what that package does.

Please be mindful of golang best practices and use the official linting and formatting tools (`golint` and `gofmt`).

Try to write comprehensive unit tests if possible.

The Timbre whitepaper is a good reference for how various Timbre protocols should be implemented.
