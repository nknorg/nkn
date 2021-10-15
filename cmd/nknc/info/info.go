package info

import (
	"fmt"
	"os"

	"github.com/nknorg/nkn/v2/api/httpjson/client"
	nknc "github.com/nknorg/nkn/v2/cmd/nknc/common"

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
	header := c.Int("header")
	headerHash := c.String("headerhash")
	blockcount := c.Bool("blockcount")
	connections := c.Bool("connections")
	neighbor := c.Bool("neighbor")
	ring := c.Bool("ring")
	state := c.Bool("state")
	version := c.Bool("nodeversion")
	balance := c.String("balance")
	nonce := c.String("nonce")
	id := c.String("id")
	pretty := c.Bool("pretty")

	var resp []byte
	var output [][]byte
	if height >= 0 {
		resp, err = client.Call(nknc.Address(), "getblock", 0, map[string]interface{}{"height": height})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		if pretty {
			if p, err := PrettyPrinter(resp).PrettyBlock(); err == nil {
				resp = p // replace resp if pretty success
			} else {
				fmt.Fprintln(os.Stderr, "Fallback to original resp due to PrettyPrint fail: ", err)
			}
		}
		output = append(output, resp)
	}

	if len(blockhash) > 0 {
		resp, err = client.Call(nknc.Address(), "getblock", 0, map[string]interface{}{"hash": blockhash})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		if pretty {
			if p, err := PrettyPrinter(resp).PrettyBlock(); err == nil {
				resp = p // replace resp if pretty success
			} else {
				fmt.Fprintln(os.Stderr, "Fallback to original resp due to PrettyPrint fail: ", err)
			}
		}
		output = append(output, resp)
	}

	if header >= 0 {
		resp, err = client.Call(nknc.Address(), "getheader", 0, map[string]interface{}{"height": header})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if len(headerHash) > 0 {
		resp, err = client.Call(nknc.Address(), "getheader", 0, map[string]interface{}{"hash": headerHash})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if latestblockhash {
		resp, err = client.Call(nknc.Address(), "getlatestblockhash", 0, map[string]interface{}{})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if blockcount {
		resp, err = client.Call(nknc.Address(), "getblockcount", 0, map[string]interface{}{})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if connections {
		resp, err = client.Call(nknc.Address(), "getconnectioncount", 0, map[string]interface{}{})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if neighbor {
		resp, err := client.Call(nknc.Address(), "getneighbor", 0, map[string]interface{}{})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if ring {
		resp, err := client.Call(nknc.Address(), "getchordringinfo", 0, map[string]interface{}{})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if state {
		resp, err := client.Call(nknc.Address(), "getnodestate", 0, map[string]interface{}{})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if txhash != "" {
		resp, err = client.Call(nknc.Address(), "gettransaction", 0, map[string]interface{}{"hash": txhash})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		if pretty {
			if p, err := PrettyPrinter(resp).PrettyTxn(); err == nil {
				resp = p // replace resp if pretty success
			} else {
				fmt.Fprintln(os.Stderr, "Output origin resp due to PrettyPrint fail: ", err)
			}
		}
		output = append(output, resp)
	}

	if version {
		resp, err = client.Call(nknc.Address(), "getversion", 0, map[string]interface{}{})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)

	}

	if balance != "" {
		resp, err := client.Call(nknc.Address(), "getbalancebyaddr", 0, map[string]interface{}{"address": balance})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if nonce != "" {
		resp, err := client.Call(nknc.Address(), "getnoncebyaddr", 0, map[string]interface{}{"address": nonce})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if id != "" {
		resp, err := client.Call(nknc.Address(), "getid", 0, map[string]interface{}{"publickey": id})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	for _, v := range output {
		nknc.FormatOutput(v)
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
			cli.BoolFlag{
				Name:  "pretty, p",
				Usage: "pretty print",
			},
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
			cli.IntFlag{
				Name:  "header",
				Usage: "get block header by height",
				Value: -1,
			},
			cli.StringFlag{
				Name:  "headerhash",
				Usage: "get block header by hash",
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
				Name:  "ring",
				Usage: "chord ring information of current node",
			},
			cli.BoolFlag{
				Name:  "state, s",
				Usage: "current node state",
			},
			cli.BoolFlag{
				Name:  "nodeversion, v",
				Usage: "version of connected remote node",
			},
			cli.StringFlag{
				Name:  "balance",
				Usage: "balance of a address",
			},
			cli.StringFlag{
				Name:  "nonce",
				Usage: "nonce of a address",
			},
			cli.StringFlag{
				Name:  "id",
				Usage: "id from publickey",
			},
		},
		Action: infoAction,
		OnUsageError: func(c *cli.Context, err error, isSubcommand bool) error {
			nknc.PrintError(c, err, "info")
			return cli.NewExitError("", 1)
		},
	}
}
