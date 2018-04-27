package main

import (
	"os"
	"sort"

	_ "github.com/nknorg/nkn/cli"
	"github.com/nknorg/nkn/cli/asset"
	"github.com/nknorg/nkn/cli/bookkeeper"
	. "github.com/nknorg/nkn/cli/common"
	"github.com/nknorg/nkn/cli/debug"
	"github.com/nknorg/nkn/cli/info"
	"github.com/nknorg/nkn/cli/wallet"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "nknc"
	app.Version = Version
	app.HelpName = "nknc"
	app.Usage = "command line tool for blockchain"
	app.UsageText = "nknc [global options] command [command options] [args]"
	app.HideHelp = false
	app.HideVersion = false
	//global options
	app.Flags = []cli.Flag{
		NewIpFlag(),
		NewPortFlag(),
	}
	//commands
	app.Commands = []cli.Command{
		*debug.NewCommand(),
		*info.NewCommand(),
		*wallet.NewCommand(),
		*asset.NewCommand(),
		*bookkeeper.NewCommand(),
	}
	sort.Sort(cli.CommandsByName(app.Commands))
	sort.Sort(cli.FlagsByName(app.Flags))

	app.Run(os.Args)
}
