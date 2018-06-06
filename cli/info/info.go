package info

import (
	"fmt"
	"os"

	. "github.com/nknorg/nkn/cli/common"
	"github.com/nknorg/nkn/rpc/httpjson"

	"github.com/urfave/cli"
)

func infoAction(c *cli.Context) (err error) {
	if c.NumFlags() == 0 {
		cli.ShowSubcommandHelp(c)
		return nil
	}
	blockhash := c.String("blockhash")
	txhash := c.String("txhash")
	latestblockhash := c.Bool("latestblockhash")
	height := c.Int("height")
	blockcount := c.Bool("blockcount")
	connections := c.Bool("connections")
	neighbor := c.Bool("neighbor")
	state := c.Bool("state")
	version := c.Bool("nodeversion")

	var resp []byte
	var output [][]byte
	if height != -1 {
		resp, err = httpjson.Call(Address(), "getblock", 0, []interface{}{height})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if c.String("blockhash") != "" {
		resp, err = httpjson.Call(Address(), "getblock", 0, []interface{}{blockhash})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if latestblockhash {
		resp, err = httpjson.Call(Address(), "getlatestblockhash", 0, []interface{}{})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if blockcount {
		resp, err = httpjson.Call(Address(), "getblockcount", 0, []interface{}{})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if connections {
		resp, err = httpjson.Call(Address(), "getconnectioncount", 0, []interface{}{})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if neighbor {
		resp, err := httpjson.Call(Address(), "getneighbor", 0, []interface{}{})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if state {
		resp, err := httpjson.Call(Address(), "getnodestate", 0, []interface{}{})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if txhash != "" {
		resp, err = httpjson.Call(Address(), "gettransaction", 0, []interface{}{txhash})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if version {
		resp, err = httpjson.Call(Address(), "getversion", 0, []interface{}{})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)

	}
	for _, v := range output {
		FormatOutput(v)
	}

	return nil
}

func NewCommand() *cli.Command {
	return &cli.Command{
		Name:        "info",
		Usage:       "show blockchain information",
		Description: "With nknc info, you could look up blocks, transactions, etc.",
		ArgsUsage:   "[args]",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "blockhash, b",
				Usage: "hash for querying a block",
			},
			cli.StringFlag{
				Name:  "txhash, t",
				Usage: "hash for querying a transaction",
			},
			cli.BoolFlag{
				Name:  "latestblockhash",
				Usage: "latest block hash",
			},
			cli.IntFlag{
				Name:  "height",
				Usage: "block height for querying a block",
				Value: -1,
			},
			cli.BoolFlag{
				Name:  "blockcount, c",
				Usage: "block number in blockchain",
			},
			cli.BoolFlag{
				Name:  "connections",
				Usage: "connection count",
			},
			cli.BoolFlag{
				Name:  "neighbor",
				Usage: "neighbor information of current node",
			},
			cli.BoolFlag{
				Name:  "state, s",
				Usage: "current node state",
			},
			cli.BoolFlag{
				Name:  "nodeversion, v",
				Usage: "version of connected remote node",
			},
		},
		Action: infoAction,
		OnUsageError: func(c *cli.Context, err error, isSubcommand bool) error {
			PrintError(c, err, "info")
			return cli.NewExitError("", 1)
		},
	}
}
