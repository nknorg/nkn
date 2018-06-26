package sigchain

import (
	"fmt"
	"os"

	"github.com/nknorg/nkn/api/httpjson/client"
	. "github.com/nknorg/nkn/cli/common"

	"github.com/urfave/cli"
)

func sigchainAction(c *cli.Context) error {
	if c.NumFlags() == 0 {
		cli.ShowSubcommandHelp(c)
		return nil
	}
	var err error
	var resp []byte
	switch {
	case c.Bool("create"):
		resp, err = client.Call(Address(), "sigchaintest", 0, map[string]interface{}{})
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
		Name:        "sigchain",
		Usage:       "signature chain operation",
		Description: "With nknc sigchain, you could create and send sigchain.",
		ArgsUsage:   "[args]",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "create, c",
				Usage: "create a new signature chain",
			},
		},
		Action: sigchainAction,
		OnUsageError: func(c *cli.Context, err error, isSubcommand bool) error {
			PrintError(c, err, "asset")
			return cli.NewExitError("", 1)
		},
	}
}
