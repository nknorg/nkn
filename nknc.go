package main

import (
	"os"
	"sort"
	"syscall"

	_ "github.com/nknorg/nkn/cli"
	"github.com/nknorg/nkn/cli/asset"
	. "github.com/nknorg/nkn/cli/common"
	"github.com/nknorg/nkn/cli/debug"
	"github.com/nknorg/nkn/cli/id"
	"github.com/nknorg/nkn/cli/info"
	"github.com/nknorg/nkn/cli/name"
	"github.com/nknorg/nkn/cli/pruning"
	"github.com/nknorg/nkn/cli/pubsub"
	"github.com/nknorg/nkn/cli/wallet"
	"github.com/nknorg/nnet/log"
	"github.com/urfave/cli"
)

func Err2Errno(err error) int {
	return 1 // TODO: github.com/nknorg/nkn/errno package.
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Panic: %+v", r)
			os.Exit(1)
		}
	}()

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
		*name.NewCommand(),
		*pubsub.NewCommand(),
		*id.NewCommand(),
		*pruning.NewCommand(),
	}
	sort.Sort(cli.CommandsByName(app.Commands))
	sort.Sort(cli.FlagsByName(app.Flags))

	if err := app.Run(os.Args); err != nil {
		switch err.(type) {
		case syscall.Errno:
			os.Exit(int(err.(syscall.Errno)))
		default:
			os.Exit(Err2Errno(err))
		}
	}
}
