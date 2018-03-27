package asset

import (
	"fmt"
	"math/rand"
	"os"

	"nkn-core/wallet"
	. "nkn-core/cli/common"
	. "nkn-core/common"
	"nkn-core/net/httpjsonrpc"
	"github.com/urfave/cli"
)

const (
	RANDBYTELEN = 4
)

func parseAssetName(c *cli.Context) string {
	name := c.String("name")
	if name == "" {
		rbuf := make([]byte, RANDBYTELEN)
		rand.Read(rbuf)
		name = "TEST-" + BytesToHexString(rbuf)
	}

	return name
}

func parseAssetID(c *cli.Context) string {
	asset := c.String("asset")
	if asset == "" {
		fmt.Println("missing flag [--asset]")
		os.Exit(1)
	}

	return asset
}

func parseAddress(c *cli.Context) string {
	if address := c.String("to"); address != "" {
		_, err := ToScriptHash(address)
		if err != nil {
			fmt.Println("invalid receiver address")
			os.Exit(1)
		}
		return address
	} else {
		fmt.Println("missing flag [--to]")
		os.Exit(1)
	}

	return ""
}

func assetAction(c *cli.Context) error {
	if c.NumFlags() == 0 {
		cli.ShowSubcommandHelp(c)
		return nil
	}
	value := c.String("value")
	if value == "" {
		fmt.Println("asset amount is required with [--value]")
		return nil
	}

	var err error
	var resp []byte
	switch {
	case c.Bool("reg"):
		resp, err = httpjsonrpc.Call(Address(), "registasset", 0, []interface{}{parseAssetName(c), value})
	case c.Bool("issue"):
		resp, err = httpjsonrpc.Call(Address(), "issueasset", 0, []interface{}{parseAssetID(c), parseAddress(c), value})
	case c.Bool("transfer"):
		resp, err = httpjsonrpc.Call(Address(), "sendtoaddress", 0, []interface{}{parseAssetID(c), parseAddress(c), value})
	case c.Bool("prepaid"):
		rates := c.String("rates")
		if rates == "" {
			fmt.Println("rates is required with [--rates]")
			return nil
		}
		resp, err = httpjsonrpc.Call(Address(), "prepaidasset", 0, []interface{}{parseAssetID(c), value, rates})
	case c.Bool("withdraw"):
		resp, err = httpjsonrpc.Call(Address(), "withdrawasset", 0, []interface{}{parseAssetID(c), value})
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
		Name:        "asset",
		Usage:       "asset registration, issuance and transfer",
		Description: "With nodectl asset, you could control assert through transaction.",
		ArgsUsage:   "[args]",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "reg, r",
				Usage: "regist a new kind of asset",
			},
			cli.BoolFlag{
				Name:  "issue, i",
				Usage: "issue asset that has been registered",
			},
			cli.BoolFlag{
				Name:  "transfer, t",
				Usage: "transfer asset",
			},
			cli.BoolFlag{
				Name:  "prepaid",
				Usage: "prepaid asset",
			},
			cli.BoolFlag{
				Name:  "withdraw",
				Usage: "withdraw asset",
			},
			cli.StringFlag{
				Name:  "wallet, w",
				Usage: "wallet name",
				Value: wallet.WalletFileName,
			},
			cli.StringFlag{
				Name:  "password, p",
				Usage: "wallet password",
			},
			cli.StringFlag{
				Name:  "asset, a",
				Usage: "uniq id for asset",
			},
			cli.StringFlag{
				Name:  "name",
				Usage: "asset name",
			},
			cli.StringFlag{
				Name:  "to",
				Usage: "asset to whom",
			},
			cli.StringFlag{
				Name:  "value, v",
				Usage: "asset amount",
				Value: "",
			},
			cli.StringFlag{
				Name:  "rates",
				Usage: "rates",
			},
		},
		Action: assetAction,
		OnUsageError: func(c *cli.Context, err error, isSubcommand bool) error {
			PrintError(c, err, "asset")
			return cli.NewExitError("", 1)
		},
	}
}
