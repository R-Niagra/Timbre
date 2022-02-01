package main

import (
	"fmt"
	"os"
	"runtime/pprof"

	"github.com/guyu96/go-timbre/log"

	noderunner "github.com/guyu96/go-timbre/runner/node"

	"github.com/urfave/cli/v2"
)

var daemonStopCmd = &cli.Command{
	Name:  "stop",
	Usage: "Stop a running timbre daemon",
	Flags: []cli.Flag{},
	Action: func(c *cli.Context) error {
		fmt.Println("TODO:- will stop the daemon programme.")
		return nil
	},
}

//DaemonCmd lists all the commands and flags to run daemon process
var DaemonCmd = &cli.Command{

	Name:     "daemon",
	Usage:    "Start a timbre daemon process",
	HideHelp: false,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "bootstrapper",
			Aliases: []string{"b"},
			Usage:   "Runs the process as a bootstrapper",
		},
		&cli.BoolFlag{
			Name:    "miner",
			Aliases: []string{"m"},
			Usage:   "Start miner role on the daemon",
		},
		&cli.StringFlag{
			Name:    "port",
			Aliases: []string{"p"},
			Usage:   "Runs the daemon process on the specified port",
			Value:   "7000",
		},
		&cli.StringFlag{
			Name:    "httpPort",
			Aliases: []string{"http"},
			Usage:   "Port for running the http server on",
			Value:   "7200",
		},
		&cli.StringFlag{
			Name:    "rpcPort",
			Aliases: []string{"rpc"},
			Usage:   "Port for running the rpc on",
			Value:   "7211",
		},
		&cli.StringFlag{
			Name:  "pprof",
			Usage: "file for writing cpu profile to",
		},
		&cli.BoolFlag{
			Name:    "DB-Reset",
			Aliases: []string{"r"},
			Usage:   "Removing stored data from last run",
			Value:   false,
		},
		&cli.StringFlag{
			Name:  "logOutput",
			Usage: "To output logs to the given file",
			Value: "terminal",
		},
	},

	Action: func(c *cli.Context) error {
		if c.NumFlags() == 0 {
			cli.ShowCommandHelp(c, "daemon")
		}

		logDest := c.String("logOutput")
		fmt.Println("Log destination is: ", logDest)
		log.SetNewOutput(logDest)

		if prof := c.String("pprof"); prof != "" {
			profile, err := os.Create(prof)
			if err != nil {
				return err
			}

			if err := pprof.StartCPUProfile(profile); err != nil {
				return err
			}
			defer pprof.StopCPUProfile()
		}
		port := c.String("port")
		httpPort := c.String("httpPort")
		rpcPort := c.String("rpcPort")
		resetDB := c.Bool("DB-Reset")

		if isBootstrapper := c.Bool("bootstrapper"); isBootstrapper {
			//run as a bootstrapper
			fmt.Println("Running as a bootstrapper")
			err := noderunner.RunAsBootstrapper(port, resetDB)
			if err != nil {
				return err
			}
		} else {
			//run as a simple node
			if port == "7000" {
				return fmt.Errorf("Port 7000 is reserved for only bootstrapper node. Plz choose other port")
			}

			fmt.Println("Running and setting new node")
			err := noderunner.RunAndSetNewNode(port, rpcPort, httpPort, resetDB, c.Bool("miner"))
			if err != nil {
				return err
			}

		}

		return nil
	},
	Subcommands: []*cli.Command{
		daemonStopCmd,
	},
}
