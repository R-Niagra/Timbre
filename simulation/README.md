# Simulation
(THIS FOLDER IS DEPRECATED)

## Setting up network of nodes
1. Navigate to simulation/main/ 
2. Open a terminal and run "go run main.go" to run the bootstrap nodes
3. Open terminals in the same folder to run nodes.
4. Run "go run main.go -p (give a unique port number)". You can add other flags based on the functionality you want from the node.
5. Flags which can be given are:- "-m" for miner, "-s" for storage-provider, "-u" for user/poster, 


This package implements simulation tests.

	Terminal 1:	go run main.go
	Terminal 2:	go run main.go -p 50006 -s
    Terminal 3:	go run main.go -p 50007 -u
	Terminal 4:	go run main.go -p 50008 -m
	Terminal 5:	go run main.go -p 50009 -m
	Terminal 6:	go run main.go -p 50010 -i

	input committee size : 2

	input 'sso' at terminal 2 to send storage offer
	input 'sp' at terminal 3 to send post request
	input 'pb' to print blockchain

# Simulation Script

Provided is a script that allows you to run the above simulation.
To run the scripts it is necessary to have the unix utility Expect installed.
The script calls the above simulation and provides the necessary inputs until a storage offer and post request is sent,
After which control is relegated back to the user.

As of 26/09/19, this Script is quite concrete and does not allow for flexible simulation arguments,
This is planned on being improved on soon.
