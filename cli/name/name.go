package name

import (
	"fmt"
	"os"

	"github.com/nknorg/nkn/api/httpjson/client"
	. "github.com/nknorg/nkn/cli/common"

	"github.com/urfave/cli"
)

func nameAction(c *cli.Context) error {
	if c.NumFlags() == 0 {
		cli.ShowSubcommandHelp(c)
		return nil
	}

	var err error
	var resp []byte
	switch {
	case c.Bool("reg"):
		name := c.String("name")
		if name == "" {
			fmt.Println("name is required with [--name]")
			return nil
		}
		resp, err = client.Call(Address(), "registername", 0, map[string]interface{}{"name": name})
	case c.Bool("del"):
		name := c.String("name")
		if name == "" {
			fmt.Println("name is required with [--name]")
			return nil
		}
		resp, err = client.Call(Address(), "deletename", 0, map[string]interface{}{"name": name})
	default:
		cli.ShowSubcommandHelp(c)
		return nil
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	FormatOutput(resp)

	return nil
}

func NewCommand() *cli.Command {
	return &cli.Command{
		Name:        "name",
		Usage:       "name registration",
		Description: "With nknc name, you could register name for your address.",
		ArgsUsage:   "[args]",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "reg, r",
				Usage: "register name for your address",
			},
			cli.BoolFlag{
				Name:  "del, d",
				Usage: "delete name of your address",
			},
			cli.StringFlag{
				Name:  "name",
				Usage: "name",
			},
		},
		Action: nameAction,
		OnUsageError: func(c *cli.Context, err error, isSubcommand bool) error {
			PrintError(c, err, "name")
			return cli.NewExitError("", 1)
		},
	}
}
