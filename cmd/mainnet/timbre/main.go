package main

import (
	"fmt"
	"log"
	"os"
	"sort"

	tcli "github.com/guyu96/go-timbre/cli"

	"github.com/urfave/cli/v2"
)

func main() {

	localCommand := []*cli.Command{
		DaemonCmd,
	}

	app := &cli.App{
		Name:  "Timbre",
		Usage: "A client of decentralized storage powering social network",
		// Version:              build.UserVersion(),
		EnableBashCompletion: true,
		HideHelp:             false,
		Commands:             append(localCommand, tcli.Commands...),

		CommandNotFound: func(c *cli.Context, command string) {
			fmt.Fprintf(c.App.Writer, "There is no %q here. Try help\n", command)
		},
		Action: func(c *cli.Context) error {
			cli.ShowAppHelp(c)
			fmt.Println("-----------------------------------------------------------------------------------------")
			fmt.Println("AUTO-COMPLETE OPTIONS:")
			cli.DefaultAppComplete(c)

			return nil
		},
	}

	sort.Sort(cli.FlagsByName(app.Flags))
	sort.Sort(cli.CommandsByName(app.Commands))
	app.Setup()
	app.Metadata["rpcPort"] = "7211" //Setting the default rpc port

	if err := app.Run(os.Args); err != nil {

		log.Fatal("Error: ", err)

		os.Exit(1)
	}

}
