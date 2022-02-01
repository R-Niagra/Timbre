package noderunner

import (
	"fmt"

	"github.com/guyu96/go-timbre/log"
	"github.com/guyu96/go-timbre/roles"
)

//CreateAndStartMiner will first create new miner instance start it and attach role to the daemon process
func CreateAndStartMiner() error {
	node := GetDaemonNode()
	if node == nil {
		return fmt.Errorf("Daemon node is not running")
	}
	if node.Miner != nil { //Will return if miner instance is already running
		return fmt.Errorf("Miner instance was already created and is running now")
	}
	miner := roles.NewMiner(node)
	node.SetMiner(miner)
	node.Services.AnnounceAndRegister()
	miner.StartMiner()
	log.Info().Msgf("Miner process has started")
	return nil
}

//CreateAndStartStorageProvider creates new sp(storage provider) and start it
func CreateAndStartStorageProvider() error {
	node := GetDaemonNode()
	if node == nil {
		return fmt.Errorf("Daemon node is not running")
	}
	if node.StorageProvider != nil { //Will return if miner instance is already running
		return fmt.Errorf("Storage provider instance has already been created")
	}

	provider := roles.NewStorageProvider(node)
	node.SetStorageProvider(provider)
	go provider.Process()
	log.Info().Msgf("Storage provider process has started")
	return nil
}

//CreateAndStartUser creates new user instance and start it
func CreateAndStartUser() error {

	node := GetDaemonNode()
	if node == nil {
		return fmt.Errorf("Daemon node is not running")
	}
	if node.User != nil { //Will return if miner instance is already running
		return fmt.Errorf("User instance is already running ")
	}

	user := roles.NewUser(node)
	node.SetUser(user)
	go node.User.Setup()
	go node.User.Process()
	log.Info().Msgf("User process has started")

	return nil
}
